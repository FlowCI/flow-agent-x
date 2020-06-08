package executor

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"fmt"
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
	"sync"
	"time"
)

const (
	dockerWorkspace = "/ws"
	dockerPluginDir = dockerWorkspace + "/.plugins"
	dockerEnvFile   = "/tmp/.env"
	dockerPullRetry = 3

	writeShellPid = "echo $$ > ~/.shell.pid\n"
	writeTtyPid   = "echo $$ > ~/.tty.pid\n"

	killShell = "kill -9 $(cat ~/.shell.pid)"
	killTty   = "kill -9 $(cat ~/.tty.pid)"
)

type (
	DockerExecutor struct {
		BaseExecutor
		volumes         []*domain.DockerVolume
		agentVolume     types.Volume
		cli             *client.Client
		containerConfig *container.Config
		hostConfig      *container.HostConfig
		containerId     string
		ttyExecId       string
		ttyWait         sync.WaitGroup
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

	d.initAgentVolume()
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
	d.stdOutWg.Add(1)

	d.pullImage()
	d.startContainer()
	d.copyPlugins()

	eid := d.runShell()
	util.LogDebug("Exec %s is running", eid)

	exitCode := d.waitForExit(eid, func(pid int) {
		d.toStartStatus(pid)
	})
	d.exportEnv()

	// wait for tty if it's running
	if d.IsInteracting() {
		util.LogDebug("Tty is running, wait..")
		d.ttyWait.Wait()
	}

	if d.result.IsFinishStatus() {
		return nil
	}

	d.toFinishStatus(exitCode)
	return
}

func (d *DockerExecutor) StartTty(ttyId string, onStarted func(ttyId string)) (out error) {
	defer func() {
		if err := recover(); err != nil {
			out = err.(error)
		}

		d.ttyExecId = ""
		d.ttyId = ""
		d.ttyWait.Done()

		close(d.streamIn)
		close(d.streamOut)
	}()

	if d.IsInteracting() {
		return fmt.Errorf("interaction is ongoning")
	}

	if d.containerId == "" {
		return fmt.Errorf("container not started")
	}

	config := types.ExecConfig{
		Tty:          true,
		AttachStdin:  true,
		AttachStderr: true,
		AttachStdout: true,
		Cmd:          []string{linuxBash},
	}

	exec, err := d.cli.ContainerExecCreate(d.context, d.containerId, config)
	util.PanicIfErr(err)

	d.ttyExecId = exec.ID
	d.ttyId = ttyId
	d.ttyWait.Add(1)

	attach, err := d.cli.ContainerExecAttach(d.context, exec.ID, config)
	util.PanicIfErr(err)
	onStarted(ttyId)

	// write pid for tty bash
	_, _ = attach.Conn.Write([]byte(writeTtyPid))

	go d.writeTtyIn(attach.Conn)
	go d.writeTtyOut(attach.Reader)

	d.waitForExit(exec.ID, nil)
	return
}

func (d *DockerExecutor) StopTty() {
	err := d.runSingleScript(killTty)
	util.LogIfError(err)
}

func (d *DockerExecutor) IsInteracting() bool {
	return d.ttyExecId != ""
}

//--------------------------------------------
// private methods
//--------------------------------------------

// agent volume that bind to /ws inside docker
func (d *DockerExecutor) initAgentVolume() {
	name := "agent-" + d.agentId
	ok, v := d.getVolume(name)

	if !ok {
		body := volume.VolumesCreateBody{
			Name: name,
		}

		created, err := d.cli.VolumeCreate(d.context, body)
		util.PanicIfErr(err)

		d.agentVolume = created
		util.LogInfo("Agent volume '%s' created", name)
	} else {
		d.agentVolume = *v
		util.LogInfo("Agent volume '%s' existed", name)
	}
}

func (d *DockerExecutor) initConfig() {
	docker := d.inCmd.Docker

	// set job work dir in the container = /ws/{flow id}
	d.workDir = filepath.Join(dockerWorkspace, util.ParseString(d.FlowId()))
	d.vars[domain.VarAgentWorkspace] = dockerWorkspace
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
		User:         docker.User,
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
		Binds:        []string{d.agentVolume.Name + ":" + dockerWorkspace},
	}

	for _, v := range d.volumes {
		ok, _ := d.getVolume(v.Name)
		if !ok {
			util.LogWarn("Volume %s not found", v.Name)
			continue
		}
		d.hostConfig.Binds = append(d.hostConfig.Binds, v.ToBindStr())
	}
}

func (d *DockerExecutor) handleErrors(err error) error {
	kill := func() {
		_ = d.runSingleScript(killShell)
		_ = d.runSingleScript(killTty)
	}

	if err == context.DeadlineExceeded {
		util.LogDebug("Timeout..")
		kill()
		d.toTimeOutStatus()
		d.context = context.Background() // reset context for further docker operation
		return nil
	}

	if err == context.Canceled {
		util.LogDebug("Cancel..")
		kill()
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

	var err error
	for i := 0; i < dockerPullRetry; i++ {
		reader, err := d.cli.ImagePull(d.context, fullRef, types.ImagePullOptions{})
		if err != nil {
			d.writeSingleLog(fmt.Sprintf("Unable to pull image %s, retrying", image))
			continue
		}

		d.writeLog(reader, false)
		break
	}

	util.PanicIfErr(err)
}

func (d *DockerExecutor) startContainer() {
	if d.tryToResume() {
		return
	}

	resp, err := d.cli.ContainerCreate(d.context, d.containerConfig, d.hostConfig, nil, "")
	util.PanicIfErr(err)

	cid := resp.ID
	d.containerId = cid
	d.result.ContainerId = cid
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
		d.result.ContainerId = containerIdToReuse
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

// copy plugin to docker container from real plugin dir
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

func (d *DockerExecutor) runShell() string {
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

	initScriptInVolume := func(in chan string) {
		for _, v := range d.volumes {
			if util.IsEmptyString(v.Script) {
				continue
			}

			in <- "source " + v.ScriptPath()
		}
	}

	writeEnv := func(in chan string) {
		in <- "env > " + dockerEnvFile
	}

	_, _ = attach.Conn.Write([]byte(writeShellPid))

	d.writeLog(attach.Reader, true)
	d.writeCmd(attach.Conn, initScriptInVolume, writeEnv)

	return exec.ID
}

func (d *DockerExecutor) runSingleScript(script string) error {
	exec, err := d.cli.ContainerExecCreate(d.context, d.containerId, types.ExecConfig{
		Cmd: []string{"/bin/bash", "-c", script},
	})

	if err != nil {
		return err
	}

	return d.cli.ContainerExecStart(d.context, exec.ID, types.ExecStartCheck{Detach: true, Tty: false})
}

func (d *DockerExecutor) exportEnv() {
	reader, _, err := d.cli.CopyFromContainer(d.context, d.containerId, dockerEnvFile)
	if err != nil {
		return
	}

	defer reader.Close()
	d.result.Output = readEnvFromReader(reader, d.inCmd.EnvFilters)
}

func (d *DockerExecutor) cleanupContainer() {
	option := d.inCmd.Docker

	if option.IsDeleteContainer {
		err := d.cli.ContainerRemove(d.context, d.containerId, types.ContainerRemoveOptions{Force: true})
		if !util.LogIfError(err) {
			util.LogInfo("Container %s for cmd %s has been deleted", d.containerId, d.CmdId())
		}
		return
	}

	if option.IsStopContainer {
		err := d.cli.ContainerStop(d.context, d.containerId, nil)
		if !util.LogIfError(err) {
			util.LogInfo("Container %s for cmd %s has been stopped", d.containerId, d.CmdId())
		}
	}
}

func (d *DockerExecutor) waitForExit(eid string, onStarted func(int)) int {
	inspect, err := d.cli.ContainerExecInspect(d.context, eid)
	util.PanicIfErr(err)
	if onStarted != nil {
		onStarted(inspect.Pid)
	}

	for {
		inspect, err = d.cli.ContainerExecInspect(d.context, eid)
		util.PanicIfErr(err)

		if !inspect.Running {
			break
		}

		time.Sleep(2 * time.Second)
	}

	return inspect.ExitCode
}

func (d *DockerExecutor) getVolume(name string) (bool, *types.Volume) {
	filter := filters.NewArgs()
	filter.Add("name", name)

	list, err := d.cli.VolumeList(d.context, filter)
	util.PanicIfErr(err)

	if len(list.Volumes) == 1 {
		return true, list.Volumes[0]
	}

	return false, nil
}

//--------------------------------------------
// util methods
//--------------------------------------------

func hasPyenv() (bool, string) {
	root := os.Getenv("PYENV_ROOT")

	if util.IsEmptyString(root) {
		root = "${HOME}/.pyenv"
	}

	root = util.ParseString(root)
	_, err := os.Stat(root)
	return !os.IsNotExist(err), root
}

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
