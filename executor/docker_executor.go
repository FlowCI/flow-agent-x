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
	dockerEnvFile = "/tmp/.env"
)

type (
	DockerExecutor struct {
		BaseExecutor
		flowVolume      types.Volume
		cli             *client.Client
		containerConfig *container.Config
		hostConfig      *container.HostConfig
		containerId     string
		workDir         string
		envFile         string
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

	// one for pull image output, and one for cmd output
	d.stdOutWg.Add(2)

	d.pullImage()
	d.startContainer()
	d.copyPlugins()

	eid := d.runCmdInContainer()
	exitCode := d.waitForExit(eid)
	d.exportEnv()

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
	d.writeLog(reader)
}

func (d *DockerExecutor) startContainer() {
	if d.tryToResume() {
		return
	}

	resp, err := d.cli.ContainerCreate(d.context, d.containerConfig, d.hostConfig, nil, "")
	util.PanicIfErr(err)

	cid := resp.ID
	d.containerId = cid
	d.CmdResult.ContainerId = cid
	util.LogDebug("Container created %s", cid)

	err = d.cli.ContainerStart(d.context, cid, types.ContainerStartOptions{})
	util.PanicIfErr(err)
	util.LogDebug("Container started %s", cid)
}

func (d *DockerExecutor) tryToResume() bool {
	containerIdToReuse := d.inCmd.ContainerId

	if util.IsEmptyString(containerIdToReuse) {
		return false
	}

	inspect, err := d.cli.ContainerInspect(d.context, containerIdToReuse)
	if client.IsErrContainerNotFound(err) {
		util.LogWarn("Container %s not found, will create a new one", containerIdToReuse)
		return false
	}

	util.PanicIfErr(err)

	if inspect.State.Status != "exited" {
		util.LogWarn("Container %s status not exited, will create a new one", containerIdToReuse)
		return false
	}

	timeout := 5 * time.Second
	err = d.cli.ContainerRestart(d.context, containerIdToReuse, &timeout)

	// resume
	if err == nil {
		d.containerId = containerIdToReuse
		d.CmdResult.ContainerId = containerIdToReuse
		util.LogInfo("Container %s resumed", inspect.ID)
		return true
	}

	// delete container that cannot resume
	_ = d.cli.ContainerRemove(d.context, containerIdToReuse, types.ContainerRemoveOptions{
		Force: true,
	})

	util.LogWarn("Failed to resume container %s, deleted", containerIdToReuse)
	return false
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
		Tty:          false,
		AttachStdin:  true,
		AttachStderr: true,
		AttachStdout: true,
		Cmd:          []string{linuxBash},
	}

	exec, err := d.cli.ContainerExecCreate(d.context, d.containerId, config)
	util.PanicIfErr(err)

	attach, err := d.cli.ContainerExecAttach(d.context, exec.ID, types.ExecConfig{Tty: false})
	util.PanicIfErr(err)

	d.writeLog(attach.Reader)
	d.writeCmd(attach.Conn, func(in chan string) {
		in <- "env > " + dockerEnvFile
	})

	return exec.ID
}

func (d *DockerExecutor) exportEnv() {
	reader, _, err := d.cli.CopyFromContainer(d.context, d.containerId, dockerEnvFile)
	if err != nil {
		return
	}

	defer reader.Close()
	d.CmdResult.Output = readEnvFromReader(reader, d.inCmd.EnvFilters)
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
