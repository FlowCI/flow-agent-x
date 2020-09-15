package executor

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/volume"
	volumetypes "github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"github/flowci/flow-agent-x/domain"
	"github/flowci/flow-agent-x/util"
	"path/filepath"
	"strings"
	"time"
)

const (
	dockerWorkspace     = "/ws"
	dockerPluginDir     = dockerWorkspace + "/.plugins"
	dockerBin           = "/ws/bin"
	dockerEnvFile       = "/tmp/.env"
	dockerPullRetry     = 3
	dockerSock          = "/var/run/docker.sock"
	dockerNetwork       = "flow-ci-agent-default"
	dockerNetworkDriver = "bridge"

	dockerShellPidPath = "/tmp/.shell.pid"
	writeShellPid      = "echo $$ > /tmp/.shell.pid\n"
	writeTtyPid        = "echo $$ > /tmp/.tty.pid\n"

	killShell = "kill -9 $(cat /tmp/.shell.pid)"
	killTty   = "kill -9 $(cat /tmp/.tty.pid)"
)

type (
	DockerExecutor struct {
		BaseExecutor
		agentVolume types.Volume
		cli         *client.Client
		configs     []*domain.DockerConfig
		ttyExecId   string
		workDir     string
		envFile     string
	}
)

func (d *DockerExecutor) runtime() *domain.DockerConfig {
	if len(d.configs) > 0 {
		return d.configs[0]
	}

	return nil
}

func (d *DockerExecutor) Init() (out error) {
	defer func() {
		if err := recover(); err != nil {
			out = err.(error)
		}
	}()

	d.result.StartAt = time.Now()

	d.cli, out = client.NewEnvClient()
	util.PanicIfErr(out)
	util.LogInfo("Docker client version: %s", d.cli.ClientVersion())

	d.initVolumeData()
	d.initNetwork()
	d.initAgentVolume()
	d.initConfig()

	return
}

func (d *DockerExecutor) Start() (out error) {
	defer func() {
		if err := recover(); err != nil {
			out = d.handleErrors(err.(error))
		}

		d.cleanupContainer()
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
		<-d.ttyCtx.Done()
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
	}()

	if d.IsInteracting() {
		return fmt.Errorf("interaction is ongoning")
	}

	runtime := d.runtime()

	if runtime.ContainerID == "" {
		return fmt.Errorf("container not started")
	}

	config := types.ExecConfig{
		Tty:          true,
		AttachStdin:  true,
		AttachStderr: true,
		AttachStdout: true,
		Cmd:          runtime.Config.Entrypoint,
	}

	exec, err := d.cli.ContainerExecCreate(d.context, runtime.ContainerID, config)
	util.PanicIfErr(err)

	attach, err := d.cli.ContainerExecAttach(d.context, exec.ID, config)
	util.PanicIfErr(err)

	d.ttyExecId = exec.ID
	d.ttyId = ttyId
	d.ttyCtx, d.ttyCancel = context.WithCancel(d.context)

	defer func() {
		d.ttyCancel()
		d.ttyCtx = nil
		d.ttyCancel = nil
	}()

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

//--------------------------------------------
// private methods
//--------------------------------------------

func (d *DockerExecutor) initVolumeData() {
	for _, v := range d.volumes {
		if !v.HasImage() {
			continue
		}

		args := filters.NewArgs()
		args.Add("name", v.Name)

		r, err := d.cli.VolumeList(d.context, args)
		util.PanicIfErr(err)

		if len(r.Volumes) > 0 {
			util.LogDebug("volume %s existed", v.Name)
			continue
		}

		// pull image
		err = d.pullImageWithName(v.Image)
		util.PanicIfErr(err)

		// create volume
		createdVolume, err := d.cli.VolumeCreate(d.context, volumetypes.VolumesCreateBody{
			Name: v.Name,
		})
		util.PanicIfErr(err)

		// create container
		cName := fmt.Sprintf("%s-init", createdVolume.Name)
		c, err := d.cli.ContainerCreate(d.context,
			&container.Config{
				Image: v.Image,
				Cmd:   []string{v.InitScriptInImage()},
			},
			&container.HostConfig{
				Mounts: []mount.Mount{
					{
						Type:   mount.TypeVolume,
						Source: createdVolume.Name,
						Target: v.DefaultTargetInImage(),
					},
				},
			},
			nil,
			cName,
		)
		util.PanicIfErr(err)

		// run container
		err = d.cli.ContainerStart(d.context, c.ID, types.ContainerStartOptions{})
		util.PanicIfErr(err)

		_, _ = d.cli.ContainerWait(d.context, c.ID)

		err = d.cli.ContainerRemove(d.context, c.ID, types.ContainerRemoveOptions{})
		util.PanicIfErr(err)
	}
}

// setup default network for agent
func (d *DockerExecutor) initNetwork() {
	args := filters.NewArgs()
	args.Add("name", dockerNetwork)

	list, err := d.cli.NetworkList(d.context, types.NetworkListOptions{
		Filters: args,
	})
	util.PanicIfErr(err)

	if len(list) == 0 {
		network, err := d.cli.NetworkCreate(d.context, dockerNetwork, types.NetworkCreate{
			CheckDuplicate: true,
			Driver:         dockerNetworkDriver,
		})
		util.PanicIfErr(err)
		util.LogInfo("network %s=%s has been created", dockerNetwork, network.ID)
	}
}

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
	d.configs = make([]*domain.DockerConfig, len(d.inCmd.Dockers))

	// find run time docker option
	var runtimeOption *domain.DockerOption
	for i, item := range d.inCmd.Dockers {
		item.SetDefaultNetwork(dockerNetwork)

		if item.IsRuntime {
			runtimeOption = item
			continue
		}
		d.configs[i] = item.ToConfig()
	}

	if runtimeOption == nil {
		if len(d.inCmd.Dockers) > 1 {
			panic(fmt.Errorf("no runtime docker option available"))
		}

		// set runtime image if only one docker option
		runtimeOption = d.inCmd.Dockers[0]
	}

	// set job work dir in the container = /ws/{flow id}
	d.workDir = filepath.Join(dockerWorkspace, util.ParseString(d.inCmd.FlowId))
	d.vars[domain.VarAgentWorkspace] = dockerWorkspace
	d.vars[domain.VarAgentJobDir] = d.workDir
	d.vars[domain.VarAgentPluginDir] = dockerPluginDir
	d.vars[domain.VarAgentDockerNetwork] = dockerNetwork

	// setup run time config
	binds := []string{d.agentVolume.Name + ":" + dockerWorkspace}
	for _, v := range d.volumes {
		ok, _ := d.getVolume(v.Name)
		if !ok {
			util.LogWarn("Volume %s not found", v.Name)
			continue
		}
		binds = append(binds, v.ToBindStr())
	}

	if util.IsFileExists(dockerSock) {
		binds = append(binds, fmt.Sprintf("%s:%s", dockerSock, dockerSock))
	}

	d.vars.Resolve()
	config := runtimeOption.ToRuntimeConfig(d.vars, d.workDir, binds)

	// set default entrypoint for runtime container
	if !config.HasEntrypoint() {
		config.Config.Entrypoint = []string{linuxBash}
	}

	// set runtime to the first element in the config array
	d.configs[0] = config
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
	for _, c := range d.configs {
		err := d.pullImageWithName(c.Config.Image)
		util.PanicIfErr(err)
	}
}

func (d *DockerExecutor) pullImageWithName(image string) (err error) {
	fullRef := "docker.io/library/" + image
	if strings.Contains(image, "/") {
		fullRef = "docker.io/" + image
	}

	for i := 0; i < dockerPullRetry; i++ {
		reader, err := d.cli.ImagePull(d.context, fullRef, types.ImagePullOptions{})
		if err != nil {
			d.writeSingleLog(fmt.Sprintf("Unable to pull image %s since %s, retrying", image, err.Error()))
			continue
		}

		d.writeLog(reader, false, false)
		break
	}

	return
}

func (d *DockerExecutor) startContainer() {
	ids := make([]string, len(d.configs))

	// create and start containers
	for i, c := range d.configs {
		if d.resume(c.ContainerID) {
			continue
		}

		resp, err := d.cli.ContainerCreate(d.context, c.Config, c.Host, nil, c.Name)
		util.PanicIfErr(err)

		err = d.cli.ContainerStart(d.context, resp.ID, types.ContainerStartOptions{})
		util.PanicIfErr(err)
		util.LogInfo("Container started %s %s", c.Config.Image, resp.ID)

		c.ContainerID = resp.ID
		ids[i] = c.ContainerID
	}

	d.result.Containers = ids
}

func (d *DockerExecutor) resume(cid string) bool {
	if util.IsEmptyString(cid) {
		return false
	}

	inspect, err := d.cli.ContainerInspect(d.context, cid)
	if client.IsErrContainerNotFound(err) {
		util.LogWarn("Container %s not found, will create a new one", cid)
		return false
	}

	util.PanicIfErr(err)

	if inspect.State.Status != "exited" {
		util.LogWarn("Container %s status not exited, will create a new one", cid)
		return false
	}

	timeout := 5 * time.Second
	err = d.cli.ContainerRestart(d.context, cid, &timeout)

	// resume
	if err == nil {
		util.LogInfo("Container %s resumed", inspect.ID)
		return true
	}

	// delete container that cannot resume
	_ = d.cli.ContainerRemove(d.context, cid, types.ContainerRemoveOptions{
		Force: true,
	})

	util.LogWarn("Failed to resume container %s, deleted", cid)
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

		err = d.cli.CopyToContainer(d.context, d.runtime().ContainerID, dockerWorkspace, reader, config)
		util.PanicIfErr(err)
		util.LogDebug("Plugin dir been created in container")
	}
}

func (d *DockerExecutor) runShell() string {
	runtime := d.runtime()

	config := types.ExecConfig{
		Tty:          false,
		AttachStdin:  true,
		AttachStderr: true,
		AttachStdout: true,
		Cmd:          runtime.Config.Entrypoint,
	}

	exec, err := d.cli.ContainerExecCreate(d.context, runtime.ContainerID, config)
	util.PanicIfErr(err)

	attach, err := d.cli.ContainerExecAttach(d.context, exec.ID, types.ExecConfig{Tty: false})
	util.PanicIfErr(err)

	setupContainerIpAndBin := func(in chan string) {
		for i, c := range d.configs {
			inspect, _ := d.cli.ContainerInspect(d.context, c.ContainerID)
			address := inspect.NetworkSettings.IPAddress

			if address == "" && len(inspect.NetworkSettings.Networks) > 0 {
				for _, v := range inspect.NetworkSettings.Networks {
					address += v.IPAddress + ","
				}
				address = address[:len(address)-1]
			}

			in <- fmt.Sprintf(domain.VarExportContainerIdPattern, i, c.ContainerID)
			in <- fmt.Sprintf(domain.VarExportContainerIpPattern, i, address)
		}

		in <- fmt.Sprintf("mkdir -p %s", dockerBin)
		in <- fmt.Sprintf("export PATH=%s:$PATH", dockerBin)

		for _, f := range binFiles {
			path := dockerBin + "/" + f.name
			b64 := base64.StdEncoding.EncodeToString(f.content)
			in <- fmt.Sprintf("echo -e \"%s\" | base64 -d > %s", b64, path)
			in <- fmt.Sprintf("chmod %s %s", f.permissionStr, path)
		}
	}

	writeEnvAfter := func(in chan string) {
		in <- "env > " + dockerEnvFile
	}

	doScript := func(script string) string {
		return script
	}

	_, _ = attach.Conn.Write([]byte(writeShellPid))

	d.writeLog(attach.Reader, true, true)
	d.writeCmd(attach.Conn, setupContainerIpAndBin, writeEnvAfter, doScript)

	return exec.ID
}

// run single bash script with new context
func (d *DockerExecutor) runSingleScript(script string) error {
	ctx := context.Background()

	exec, err := d.cli.ContainerExecCreate(ctx, d.runtime().ContainerID, types.ExecConfig{
		Cmd: []string{linuxBash, "-c", script},
	})

	if err != nil {
		return err
	}

	util.LogDebug("Script: %s will run", script)
	return d.cli.ContainerExecStart(ctx, exec.ID, types.ExecStartCheck{Detach: true, Tty: false})
}

func (d *DockerExecutor) exportEnv() {
	reader, _, err := d.cli.CopyFromContainer(d.context, d.runtime().ContainerID, dockerEnvFile)
	if err != nil {
		return
	}

	defer reader.Close()
	d.result.Output = readEnvFromReader(reader, d.inCmd.EnvFilters)
}

func (d *DockerExecutor) cleanupContainer() {
	if d.cli == nil {
		return
	}

	for _, c := range d.configs {
		if c.IsDelete {
			err := d.cli.ContainerRemove(d.context, c.ContainerID, types.ContainerRemoveOptions{Force: true})
			if !util.LogIfError(err) {
				util.LogInfo("Container %s %s for cmd %s has been deleted", c.Config.Image, c.ContainerID, d.inCmd.ID)
			}
			continue
		}

		if c.IsStop {
			err := d.cli.ContainerStop(d.context, c.ContainerID, nil)
			if !util.LogIfError(err) {
				util.LogInfo("Container %s %s for cmd %s has been stopped", c.Config.Image, c.ContainerID, d.inCmd.ID)
			}
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
