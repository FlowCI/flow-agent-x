package executor

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/volume"
	volumetypes "github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"github.com/flowci/flow-agent-x/domain"
	"github.com/flowci/flow-agent-x/util"
	"io/ioutil"
	"os"
	"strings"
	"time"
)

const (
	dockerWorkspace       = "/ws"
	dockerPluginDir       = dockerWorkspace + "/.plugins"
	dockerBin             = "/ws/bin"
	dockerEnvFile         = "/tmp/.env"
	dockerPullRetry       = 3
	dockerSock            = "/var/run/docker.sock"
	dockerNetwork         = "flow-ci-agent-default"
	dockerNetworkDriver   = "bridge"
	dockerVarDockerHost   = "DOCKER_HOST"
	dockerDefaultExitCode = -1

	dockerShellPidPath = "/tmp/.shell.pid"
	writeShellPid      = "echo $$ > /tmp/.shell.pid\n"
	writeTtyPid        = "echo $$ > /tmp/.tty.pid\n"

	killShell = "kill -9 $(cat /tmp/.shell.pid)"
	killTty   = "kill -9 $(cat /tmp/.tty.pid)"
)

var (
	placeHolder void
)

type (
	void struct{}

	dockerExecutor struct {
		BaseExecutor
		wsFromDockerVolume bool
		wsVolume           types.Volume
		cli                *client.Client
		configs            []*domain.DockerConfig
		ttyExecId          string
		envFile            string
	}
)

func (d *dockerExecutor) runtime() *domain.DockerConfig {
	if len(d.configs) > 0 {
		return d.configs[0]
	}

	return nil
}

func (d *dockerExecutor) Init() (out error) {
	defer util.RecoverPanic(func(e error) {
		out = e
	})

	d.os = util.OSLinux // only support unix based image
	d.result.StartAt = time.Now()

	d.cli, out = client.NewEnvClient()
	util.PanicIfErr(out)
	util.LogInfo("Docker client version: %s", d.cli.ClientVersion())

	d.initVolumeData()
	d.initNetwork()
	d.initWorkspaceVolume()
	d.initConfig()

	return nil
}

func (d *dockerExecutor) Start() (out error) {
	for i := d.inCmd.Retry; i >= 0; i-- {
		out = d.doStart()
		r := d.result

		if r.Status == domain.CmdStatusException || out != nil {
			if i > 0 {
				d.writeSingleLog(">>>>>>> retry >>>>>>>")
			}
			continue
		}

		break
	}

	return
}

func (d *dockerExecutor) doStart() (out error) {
	defer util.RecoverPanic(func(e error) {
		out = d.handleErrors(e)
	})

	defer d.cleanupContainer()

	// one for pull image output, and one for cmd output
	d.stdOutWg.Add(1)

	d.pullImage()
	d.startContainer()
	d.copyPlugins()
	d.copyCache()

	eid := d.runShell()
	util.LogDebug("Exec %s is running", eid)

	exitCode := d.waitForExit(d.context, eid, func(pid int) {
		d.toStartStatus(pid)
	})
	d.exportEnv()

	// wait for tty if it's running
	if d.IsInteracting() {
		util.LogDebug("Tty is running, wait..")
		<-d.ttyCtx.Done()
	}

	d.writeCache()

	if d.result.IsFinishStatus() {
		return nil
	}

	d.toFinishStatus(exitCode)
	return
}

func (d *dockerExecutor) StartTty(ttyId string, onStarted func(ttyId string)) (out error) {
	defer util.RecoverPanic(func(e error) {
		out = e

		d.ttyExecId = ""
		d.ttyId = ""
	})

	if d.IsInteracting() {
		panic(fmt.Errorf("interaction is ongoning"))
	}

	runtime := d.runtime()

	if runtime.ContainerID == "" {
		panic(fmt.Errorf("container not started"))
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

	d.waitForExit(d.context, exec.ID, nil)
	return
}

func (d *dockerExecutor) StopTty() {
	_, err := d.runSingleScript(killTty)
	util.LogIfError(err)
}

//--------------------------------------------
// private methods
//--------------------------------------------

func (d *dockerExecutor) initVolumeData() {
	runCmd := func(v *domain.DockerVolume) {
		c, err := d.cli.ContainerCreate(d.context,
			&container.Config{
				Image: v.Image,
				Cmd:   []string{v.InitScriptInImage()},
			},
			&container.HostConfig{
				Mounts: []mount.Mount{
					{
						Type:   mount.TypeVolume,
						Source: v.Name,
						Target: v.DefaultTargetInImage(),
					},
				},
			},
			nil,
			fmt.Sprintf("%s-init", v.Name),
		)
		util.PanicIfErr(err)

		defer func() {
			_ = d.cli.ContainerRemove(d.context, c.ID, types.ContainerRemoveOptions{})
		}()

		// run container and wait
		err = d.cli.ContainerStart(d.context, c.ID, types.ContainerStartOptions{})
		util.PanicIfErr(err)

		_, _ = d.cli.ContainerWait(d.context, c.ID)
	}

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
		err = d.pullImageWithName(v.Image, nil)
		util.PanicIfErr(err)

		// create volume
		_, err = d.cli.VolumeCreate(d.context, volumetypes.VolumesCreateBody{
			Name: v.Name,
		})
		util.PanicIfErr(err)

		runCmd(v)
	}
}

// setup default network for agent
func (d *dockerExecutor) initNetwork() {
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

func (d *dockerExecutor) initConfig() {
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
	d.jobDir = dockerWorkspace + "/" + util.ParseString(d.inCmd.FlowId)
	d.vars[domain.VarAgentWorkspace] = dockerWorkspace
	d.vars[domain.VarAgentJobDir] = d.jobDir
	d.vars[domain.VarAgentPluginDir] = dockerPluginDir
	d.vars[domain.VarAgentDockerNetwork] = dockerNetwork

	// setup workspace
	ws := d.workspace
	if d.wsFromDockerVolume {
		ws = d.wsVolume.Name
	}

	// setup run time config
	binds := []string{ws + ":" + dockerWorkspace}

	for _, v := range d.volumes {
		ok, _ := d.getVolume(v.Name)
		if !ok {
			util.LogWarn("Volume %s not found", v.Name)
			continue
		}
		binds = append(binds, v.ToBindStr())
	}

	// mount docker dock if exit or running on Windows
	if util.IsFileExists(dockerSock) || util.IsWindows() {
		binds = append(binds, fmt.Sprintf("%s:%s", dockerSock, dockerSock))
	}

	// set agent ip and docker host env
	if d.isK8sEnabled() {
		agentIpKey := fmt.Sprintf(domain.VarAgentIpPattern, "en0")
		d.vars[agentIpKey] = d.k8sConfig.PodIp
		d.vars[dockerVarDockerHost] = fmt.Sprintf("tcp://%s:2375", d.k8sConfig.PodIp)
	}

	d.vars.Resolve()
	config := runtimeOption.ToRuntimeConfig(domain.ConnectVars(d.vars, d.secretVars), d.jobDir, binds)

	// set default entrypoint for runtime container
	if !config.HasEntrypoint() {
		config.Config.Entrypoint = []string{linuxBash}
	}

	// set runtime to the first element in the config array
	d.configs[0] = config
}

// agent volume that bind to /ws inside docker
func (d *dockerExecutor) initWorkspaceVolume() {
	if !d.wsFromDockerVolume {
		util.LogInfo("Workspace volume will be mounted from host machine")
		return
	}

	name := "agent-" + d.agentId
	ok, v := d.getVolume(name)

	if ok {
		d.wsVolume = *v
		util.LogInfo("Agent volume '%s' existed", name)
		return
	}

	body := volume.VolumesCreateBody{Name: name}
	created, err := d.cli.VolumeCreate(d.context, body)
	util.PanicIfErr(err)

	d.wsVolume = created
	util.LogInfo("Workspace volume '%s' created", name)
	return
}

func (d *dockerExecutor) handleErrors(err error) error {
	kill := func() {
		_, _ = d.runSingleScript(killShell)
		_, _ = d.runSingleScript(killTty)
	}

	util.LogWarn("handleError on docker: %s", err.Error())

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

func (d *dockerExecutor) findImageLocally(image string) (bool, error) {
	list, err := d.cli.ImageList(d.context, types.ImageListOptions{All: true})
	if err != nil {
		return false, err
	}

	for _, imageInfo := range list {
		for _, t := range imageInfo.RepoTags {
			if t == image {
				return true, nil
			}
		}
	}

	return false, nil
}

func (d *dockerExecutor) pullImage() {
	for _, c := range d.configs {
		err := d.pullImageWithName(c.Config.Image, c.Auth)
		util.PanicIfErr(err)
	}
}

func (d *dockerExecutor) pullImageWithName(image string, auth *domain.SimpleAuthPair) (out error) {
	if isOnLocal, err := d.findImageLocally(image); isOnLocal {
		out = err
		return
	}

	if util.HasError(out) {
		return
	}

	fullRef := image

	if isDockerHubImage(image) {
		fullRef = "docker.io/library/" + image
		if strings.Contains(image, "/") {
			fullRef = "docker.io/" + image
		}
	}

	options := types.ImagePullOptions{}
	if auth != nil {
		jsonBytes, err := json.Marshal(auth)
		if err != nil {
			return err
		}
		options.RegistryAuth = base64.StdEncoding.EncodeToString(jsonBytes)
	}

	for i := 0; i < dockerPullRetry; i++ {
		reader, err := d.cli.ImagePull(d.context, fullRef, options)
		if err != nil {
			out = err
			d.writeSingleLog(fmt.Sprintf("Unable to pull image %s since %s, retrying", image, err.Error()))
			continue
		}

		d.writeLog(reader, false, false)
		break
	}

	return
}

func (d *dockerExecutor) startContainer() {
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

		msg := fmt.Sprintf("Container started %s %s", c.Config.Image, resp.ID)
		util.LogInfo(msg)
		d.writeSingleLog(msg)

		c.ContainerID = resp.ID
		ids[i] = c.ContainerID
	}

	d.result.Containers = ids
}

func (d *dockerExecutor) resume(cid string) bool {
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
func (d *dockerExecutor) copyPlugins() {
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

// copy cache to job dir in docker container if cache defined
func (d *dockerExecutor) copyCache() {
	if !util.HasString(d.cacheInputDir) {
		return
	}

	defer util.RecoverPanic(func(e error) {
		util.LogWarn(e.Error())
	})

	files, err := ioutil.ReadDir(d.cacheInputDir)
	util.PanicIfErr(err)

	// do not over write exiting file
	config := types.CopyToContainerOptions{
		AllowOverwriteDirWithFile: false,
	}

	for _, f := range files {
		cachePath := d.cacheInputDir + util.UnixPathSeparator + f.Name()
		reader, err := tarArchiveFromPath(cachePath)
		util.PanicIfErr(err)

		// rm existing path and copy cache into dest dir
		dest := d.jobDir + util.UnixPathSeparator + f.Name()
		_, _ = d.runSingleScript("rm -rf " + dest)

		err = d.cli.CopyToContainer(d.context, d.runtime().ContainerID, d.jobDir, reader, config)
		util.PanicIfErr(err)
		d.writeSingleLog(fmt.Sprintf("cache %s has been applied", f.Name()))

		// remove cache from cache dir anyway
		_ = os.RemoveAll(cachePath)
	}
}

func (d *dockerExecutor) writeCache() {
	if !d.inCmd.HasCache() {
		return
	}

	dir, err := ioutil.TempDir("", "_cache_output_")
	if err != nil {
		util.LogWarn(err.Error())
		return
	}

	d.cacheOutputDir = dir
	cache := d.inCmd.Cache

	for _, path := range cache.Paths {
		cachePath := d.jobDir + util.UnixPathSeparator + path
		tarStream, _, err := d.cli.CopyFromContainer(d.context, d.runtime().ContainerID, cachePath)

		if err != nil {
			util.LogWarn(err.Error())
			continue
		}

		err = untarFromReader(tarStream, dir)
		if err != nil {
			util.LogWarn(err.Error())
		}

		tarStream.Close()
	}
}

func (d *dockerExecutor) runShell() string {
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

	setupContainerIpAndBin := func() []string {
		var scripts []string

		for i, c := range d.configs {
			inspect, _ := d.cli.ContainerInspect(d.context, c.ContainerID)
			address := inspect.NetworkSettings.IPAddress

			if address == "" && len(inspect.NetworkSettings.Networks) > 0 {
				for _, v := range inspect.NetworkSettings.Networks {
					address += v.IPAddress + ","
				}
				address = address[:len(address)-1]
			}

			scripts = append(scripts, fmt.Sprintf(domain.VarExportContainerIdPattern, i, c.ContainerID))
			scripts = append(scripts, fmt.Sprintf(domain.VarExportContainerIpPattern, i, address))
		}

		scripts = append(scripts, fmt.Sprintf("mkdir -p %s", dockerBin))
		scripts = append(scripts, fmt.Sprintf("export PATH=%s:$PATH", dockerBin))

		for _, f := range binFiles {
			path := dockerBin + "/" + f.name
			b64 := base64.StdEncoding.EncodeToString(f.content)

			scripts = append(scripts, fmt.Sprintf("echo -e \"%s\" | base64 -d > %s", b64, path))
			scripts = append(scripts, fmt.Sprintf("chmod %s %s", f.permissionStr, path))
		}

		return scripts
	}

	writeEnvAfter := func() []string {
		return []string{"env > " + dockerEnvFile}
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
func (d *dockerExecutor) runSingleScript(script string) (exitCode int, err error) {
	defer util.RecoverPanic(func(e error) {
		exitCode = dockerDefaultExitCode
	})

	ctx := context.Background()

	exec, err := d.cli.ContainerExecCreate(ctx, d.runtime().ContainerID, types.ExecConfig{
		Cmd: []string{linuxBash, "-c", script},
	})
	util.PanicIfErr(err)

	util.LogDebug("Script: %s will run", script)
	err = d.cli.ContainerExecStart(ctx, exec.ID, types.ExecStartCheck{Detach: true, Tty: false})
	util.PanicIfErr(err)

	exitCode = d.waitForExit(ctx, exec.ID, nil)
	return
}

func (d *dockerExecutor) exportEnv() {
	reader, _, err := d.cli.CopyFromContainer(d.context, d.runtime().ContainerID, dockerEnvFile)
	if err != nil {
		return
	}

	defer reader.Close()
	d.result.Output = readEnvFromReader(d.os, reader, d.inCmd.EnvFilters)
}

func (d *dockerExecutor) cleanupContainer() {
	if d.cli == nil {
		return
	}

	// apply new context since d.context might cancelled
	ctx := context.Background()

	for _, c := range d.configs {
		if c.IsDelete {
			err := d.cli.ContainerRemove(ctx, c.ContainerID, types.ContainerRemoveOptions{Force: true})
			if !util.LogIfError(err) {
				util.LogInfo("Container %s %s for cmd %s has been deleted", c.Config.Image, c.ContainerID, d.inCmd.ID)
			}
			continue
		}

		if c.IsStop {
			err := d.cli.ContainerStop(ctx, c.ContainerID, nil)
			if !util.LogIfError(err) {
				util.LogInfo("Container %s %s for cmd %s has been stopped", c.Config.Image, c.ContainerID, d.inCmd.ID)
			}
		}
	}
}

func (d *dockerExecutor) waitForExit(ctx context.Context, eid string, onStarted func(int)) int {
	inspect, err := d.cli.ContainerExecInspect(ctx, eid)
	util.PanicIfErr(err)
	if onStarted != nil {
		onStarted(inspect.Pid)
	}

	for {
		inspect, err = d.cli.ContainerExecInspect(ctx, eid)
		util.PanicIfErr(err)

		if !inspect.Running {
			break
		}

		time.Sleep(1 * time.Second)
	}

	return inspect.ExitCode
}

func (d *dockerExecutor) getVolume(name string) (bool, *types.Volume) {
	filter := filters.NewArgs()
	filter.Add("name", name)

	list, err := d.cli.VolumeList(d.context, filter)
	util.PanicIfErr(err)

	if len(list.Volumes) == 1 {
		return true, list.Volumes[0]
	}

	return false, nil
}
