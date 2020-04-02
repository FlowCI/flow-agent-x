package executor

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/volume"
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
	dockerWorkspace = "/ws"
	dockerPluginDir = dockerWorkspace + "/.plugins"
)

type (
	DockerExecutor struct {
		BaseExecutor
		flowVolume           types.Volume
		cli                  *client.Client
		containerConfig      *container.Config
		hostConfig           *container.HostConfig
		containerId          string
		workDir              string
	}
)

func (d *DockerExecutor) Init() (out error) {
	defer func() {
		if err := recover(); err != nil {
			out = err.(error)
		}
	}()

	d.cli, out = client.NewEnvClient()
	util.PanicIfErr(out)

	d.initJobVolume()
	d.initConfig()

	return
}

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

	d.pullImage()
	d.createContainer()

	d.startContainer()
	d.copyPlugins()

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

// create job volume based on flow id
func (d *DockerExecutor) initJobVolume() {
	name := "flow-" + d.inCmd.FlowId

	filter := filters.NewArgs()
	filter.Add("name", name)

	list, err := d.cli.VolumeList(d.context, filter)
	util.PanicIfErr(err)

	if len(list.Volumes) == 1 {
		d.flowVolume = *list.Volumes[0]
		util.LogInfo("Job volume '%s' existed", name)
		return
	}

	d.flowVolume, err = d.cli.VolumeCreate(d.context, volume.VolumesCreateBody{
		Name: name,
	})
	util.PanicIfErr(err)
	util.LogInfo("Job volume '%s' created", name)
}

func (d *DockerExecutor) initConfig() {
	docker := d.inCmd.Docker

	// set work dir in the container
	d.workDir = filepath.Join(dockerWorkspace, util.ParseString(d.inCmd.FlowId))
	d.vars[domain.VarAgentJobDir] = d.workDir
	d.vars[domain.VarAgentPluginDir] = dockerPluginDir

	portSet, portMap, err := nat.ParsePortSpecs(docker.Ports)
	util.PanicIfErr(err)

	image := util.ParseStringWithSource(docker.Image, d.vars)

	entrypoint := make([]string, len(docker.Entrypoint))
	for i, item := range docker.Entrypoint {
		entrypoint[i] = util.ParseStringWithSource(item, d.vars)
	}

	d.containerConfig = &container.Config{
		Image:        image,
		Env:          d.vars.ToStringArray(),
		Entrypoint:   entrypoint,
		ExposedPorts: portSet,
		Tty:          false,
		AttachStdin:  true,
		AttachStderr: true,
		AttachStdout: true,
		OpenStdin:    true,
		StdinOnce:    true,
		WorkingDir:   d.workDir,
	}

	d.hostConfig = &container.HostConfig{
		NetworkMode:  container.NetworkMode(docker.NetworkMode),
		PortBindings: portMap,
		Binds:        []string{d.flowVolume.Name + ":" + d.workDir},
	}
}

func (d *DockerExecutor) handleErrors(err error) error {
	if err == context.DeadlineExceeded {
		util.LogDebug("Timeout..")
		d.toTimeOutStatus()
		d.context = context.Background() // reset context for further docker operation
		return nil
	}

	if err == context.Canceled {
		util.LogDebug("Cancel..")
		d.toKilledStatus()
		d.context = context.Background() // reset context for further docker operation
		return nil
	}

	_ = d.toErrorStatus(err)
	return err
}

func (d *DockerExecutor) pullImage() {
	image := d.containerConfig.Image

	fullRef := "docker.io/library/" + image
	if strings.Contains(image, "/") {
		fullRef = "docker.io/" + image
	}

	reader, err := d.cli.ImagePull(d.context, fullRef, types.ImagePullOptions{})

	util.PanicIfErr(err)
	d.writeLogToChannel(reader)
}

func (d *DockerExecutor) createContainer() {
	resp, err := d.cli.ContainerCreate(d.context, d.containerConfig, d.hostConfig, nil, "")
	util.PanicIfErr(err)
	util.LogDebug("Container created %s", resp.ID)

	d.containerId = resp.ID
	d.CmdResult.ContainerId = resp.ID
}

func (d *DockerExecutor) startContainer() {
	err := d.cli.ContainerStart(d.context, d.containerId, types.ContainerStartOptions{})
	util.PanicIfErr(err)
}

func (d *DockerExecutor) copyPlugins() {
	config := types.CopyToContainerOptions{
		AllowOverwriteDirWithFile: true,
	}

	if !util.IsEmptyString(d.pluginDir) {
		reader, err := tarArchiveFromPath(d.pluginDir)
		util.PanicIfErr(err)

		err = d.cli.CopyToContainer(d.context, d.containerId, dockerWorkspace, reader, config)
		util.PanicIfErr(err)
		util.LogDebug("Plugin dir been created in container")
	}
}

func (d *DockerExecutor) runCmdInContainer() string {
	config := types.ExecConfig{
		Tty:          true,
		AttachStdin:  true,
		AttachStderr: true,
		AttachStdout: true,
		Cmd:          []string{linuxBash},
	}

	exec, err := d.cli.ContainerExecCreate(d.context, d.containerId, config)
	util.PanicIfErr(err)

	attach, err := d.cli.ContainerExecAttach(d.context, exec.ID, types.ExecConfig{Tty: true})
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

// tar dir, ex: abc/.. output is archived content .. in dir
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
