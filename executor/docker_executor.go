package executor

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github/flowci/flow-agent-x/domain"
	"github/flowci/flow-agent-x/util"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	dockerBaseDir = "/ws"
	dockerPluginDir = dockerBaseDir + "/plugins"
)

type (
	DockerExecutor struct {
		BaseExecutor
		cli                  *client.Client
		containerId          string
		workDirInContainer   string
		pluginDirInContainer string
	}
)

func (d *DockerExecutor) Start() (out error) {
	defer func() {
		if err := recover(); err != nil {
			out = d.handleErrors(err.(error))
		}

		if d.cli != nil {
			d.cleanupContainer()
		}

		d.closeChannels()
	}()

	d.stdOutWg.Add(1)

	d.workDirInContainer = dockerBaseDir + "/" + filepath.Base(d.workDir)
	d.inVars[domain.VarAgentJobDir] = d.workDirInContainer
	d.inVars[domain.VarAgentPluginDir] = dockerPluginDir

	var err error
	d.cli, err = client.NewEnvClient()
	util.PanicIfErr(err)

	d.pullImage()
	d.createContainer()

	d.startContainer()
	d.copyToContainer()

	eid := d.runCmdInContainer()
	exitCode := d.waitForExit(eid)

	if d.CmdResult.IsFinishStatus() {
		return nil
	}

	d.toFinishStatus(exitCode)
	return
}

//--------------------------------------------
// private methods
//--------------------------------------------

func (d *DockerExecutor) handleErrors(err error) error {
	if err == context.DeadlineExceeded {
		util.LogDebug("Timeout..")
		d.toTimeOutStatus()
		return nil
	}

	if err == context.Canceled {
		util.LogDebug("Cancel..")
		d.toKilledStatus()
		return nil
	}

	_ = d.toErrorStatus(err)
	return err
}

func (d *DockerExecutor) pullImage() {
	image := d.inCmd.Docker.Image
	fullRef := "docker.io/library/" + image
	if strings.Contains(image, "/") {
		fullRef = "docker.io/" + image
	}

	reader, err := d.cli.ImagePull(d.context, fullRef, types.ImagePullOptions{})

	util.PanicIfErr(err)
	d.writeLogToChannel(reader)
}

func (d *DockerExecutor) createContainer() {
	docker := d.inCmd.Docker
	portSet, portMap, err := nat.ParsePortSpecs(docker.Ports)
	util.PanicIfErr(err)

	config := &container.Config{
		Image:        docker.Image,
		Env:          append(d.inVars.ToStringArray(), d.inCmd.VarsToStringArray()...),
		Entrypoint:   docker.Entrypoint,
		ExposedPorts: portSet,
		Tty:          false,
		AttachStdin:  true,
		AttachStderr: true,
		AttachStdout: true,
		OpenStdin:    true,
		StdinOnce:    true,
		WorkingDir:   d.workDirInContainer,
	}

	hostConfig := &container.HostConfig{
		NetworkMode:  container.NetworkMode(docker.NetworkMode),
		PortBindings: portMap,
	}

	resp, err := d.cli.ContainerCreate(d.context, config, hostConfig, nil, "")
	util.PanicIfErr(err)
	util.LogDebug("Container created %s", resp.ID)

	d.containerId = resp.ID
	d.CmdResult.ContainerId = resp.ID
}

func (d *DockerExecutor) startContainer() {
	err := d.cli.ContainerStart(d.context, d.containerId, types.ContainerStartOptions{})
	util.PanicIfErr(err)
}

func (d *DockerExecutor) copyToContainer() {
	config := types.CopyToContainerOptions{
		AllowOverwriteDirWithFile: true,
	}

	if !util.IsEmptyString(d.workDir) {
		reader, err := tarArchiveFromPath(d.workDir)
		util.PanicIfErr(err)

		err = d.cli.CopyToContainer(d.context, d.containerId, dockerBaseDir, reader, config)
		util.PanicIfErr(err)
		util.LogDebug("Job working dir been created in container")
	}

	if !util.IsEmptyString(d.pluginDir) {
		reader, err := tarArchiveFromPath(d.pluginDir)
		util.PanicIfErr(err)

		err = d.cli.CopyToContainer(d.context, d.containerId, dockerBaseDir, reader, config)
		util.PanicIfErr(err)
		util.LogDebug("Plugin dir been created in container")
	}
}

func (d *DockerExecutor) runCmdInContainer() string {
	config := types.ExecConfig{
		Tty:          false,
		AttachStdin:  true,
		AttachStderr: true,
		AttachStdout: true,
		Cmd:          []string{linuxBash},
	}

	exec, err := d.cli.ContainerExecCreate(d.context, d.containerId, config)
	util.PanicIfErr(err)

	attach, err := d.cli.ContainerExecAttach(d.context, exec.ID, types.ExecConfig{})
	util.PanicIfErr(err)

	onStdOutExit := func() {
		d.stdOutWg.Done()
		util.LogDebug("[Exit]: StdOut/Err, log size = %d", d.CmdResult.LogSize)
	}

	_ = d.startConsumeStdOut(attach.Reader, onStdOutExit)
	_ = d.startConsumeStdIn(attach.Conn)

	return exec.ID
}

func (d *DockerExecutor) cleanupContainer() {
	option := d.inCmd.Docker

	if option.IsDeleteContainer {
		err := d.cli.ContainerRemove(d.context, d.containerId, types.ContainerRemoveOptions{Force: true})
		if !util.LogIfError(err) {
			util.LogInfo("Container %s for cmd %s has been deleted", d.containerId, d.CmdID())
		}
		return
	}

	if option.IsStopContainer {
		err := d.cli.ContainerStop(d.context, d.containerId, nil)
		if !util.LogIfError(err) {
			util.LogInfo("Container %s for cmd %s has been stopped", d.containerId, d.CmdID())
		}
	}
}

func (d *DockerExecutor) waitForExit(eid string) int {
	inspect, err := d.cli.ContainerExecInspect(d.context, eid)
	util.PanicIfErr(err)
	d.toStartStatus(inspect.Pid)

	for {
		inspect, err = d.cli.ContainerExecInspect(d.context, eid)
		util.PanicIfErr(err)

		if !inspect.Running {
			break
		}

		time.Sleep(1 * time.Second)
	}

	return inspect.ExitCode
}

func (d *DockerExecutor) writeLogToChannel(reader io.Reader) {
	bufferReader := bufio.NewReaderSize(reader, defaultReaderBufferSize)
	var builder strings.Builder

	for {
		line, err := readLine(bufferReader, builder)
		builder.Reset()

		if err != nil {
			return
		}

		log := &domain.LogItem{
			CmdID:   d.CmdID(),
			Content: line,
		}

		d.logChannel <- log
	}
}

//--------------------------------------------
// util methods
//--------------------------------------------

// tar dir, ex: abc/.. output is archived abc dir
func tarArchiveFromPath(path string) (io.Reader, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	dir := filepath.Dir(path)

	ok := filepath.Walk(path, func(file string, fi os.FileInfo, err error) (out error) {
		defer func() {
			if err := recover(); err != nil {
				out = err.(error)
			}
		}()

		util.PanicIfErr(err)

		header, err := tar.FileInfoHeader(fi, fi.Name())
		util.PanicIfErr(err)

		header.Name = strings.TrimPrefix(strings.Replace(file, dir, "", -1), string(filepath.Separator))
		err = tw.WriteHeader(header)
		util.PanicIfErr(err)

		f, err := os.Open(file)
		util.PanicIfErr(err)

		if fi.IsDir() {
			return
		}

		_, err = io.Copy(tw, f)
		util.PanicIfErr(err)

		err = f.Close()
		util.PanicIfErr(err)

		return
	})

	if ok != nil {
		return nil, ok
	}

	ok = tw.Close()
	if ok != nil {
		return nil, ok
	}

	return bufio.NewReader(&buf), nil
}
