package executor

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github/flowci/flow-agent-x/domain"
	"github/flowci/flow-agent-x/util"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type (
	DockerExecutor struct {
		BaseExecutor
		workDirInContainer string
	}
)

func (d *DockerExecutor) Start() (out error) {
	defer func() {
		if err := recover(); err != nil {
			out = d.handleErrors(err.(error))
		}

		d.closeChannels()
	}()

	d.stdOutWg.Add(1)
	d.workDirInContainer = "/ws/" + d.inCmd.FlowId
	d.inVars[domain.VarAgentJobDir] = d.workDirInContainer

	// pull image
	cli, err := client.NewEnvClient()
	util.PanicIfErr(err)
	defer cli.Close()

	d.pullImage(cli)

	cid := d.createContainer(cli)
	defer d.cleanupContainer(cli, cid)

	d.startContainer(cli, cid)
	d.copyToContainer(cli, cid)

	eid := d.runCmdInContainer(cli, cid)
	exitCode := d.waitForExit(cli, eid)

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

func (d *DockerExecutor) pullImage(cli *client.Client) {
	fullRef := "docker.io/library/" + d.inCmd.Docker.Image
	reader, err := cli.ImagePull(d.context, fullRef, types.ImagePullOptions{})

	util.PanicIfErr(err)
	d.writeLogToChannel(reader)
}

func (d *DockerExecutor) createContainer(cli *client.Client) string {
	docker := d.inCmd.Docker

	config := &container.Config{
		Image:        docker.Image,
		Env:          append(d.inVars.ToStringArray(), d.inCmd.VarsToStringArray()...),
		Entrypoint:   docker.Entrypoint,
		Tty:          false,
		AttachStdin:  true,
		AttachStderr: true,
		AttachStdout: true,
		OpenStdin:    true,
		StdinOnce:    true,
		WorkingDir:   d.workDirInContainer,
	}

	resp, err := cli.ContainerCreate(d.context, config, nil, nil, "")
	util.PanicIfErr(err)
	util.LogDebug("Container created %s", resp.ID)

	return resp.ID
}

func (d *DockerExecutor) startContainer(cli *client.Client, cid string) {
	err := cli.ContainerStart(d.context, cid, types.ContainerStartOptions{})
	util.PanicIfErr(err)
}

func (d *DockerExecutor) copyToContainer(cli *client.Client, containerId string) {
	reader, err := tarArchiveFromPath(d.workDir)
	util.PanicIfErr(err)

	config := types.CopyToContainerOptions{
		AllowOverwriteDirWithFile: true,
	}

	err = cli.CopyToContainer(d.context, containerId, d.workDirInContainer, reader, config)
	util.PanicIfErr(err)
	util.LogDebug("Job working dir been created in container")
}

func (d *DockerExecutor) runCmdInContainer(cli *client.Client, cid string) string {
	config := types.ExecConfig{
		Tty:          false,
		AttachStdin:  true,
		AttachStderr: true,
		AttachStdout: true,
		Cmd:          []string{linuxBash},
	}

	exec, err := cli.ContainerExecCreate(d.context, cid, config)
	util.PanicIfErr(err)

	attach, err := cli.ContainerExecAttach(d.context, exec.ID, types.ExecConfig{})
	util.PanicIfErr(err)

	onStdOutExit := func() {
		d.stdOutWg.Done()
		util.LogDebug("[Exit]: StdOut/Err, log size = %d", d.CmdResult.LogSize)
	}

	_ = d.startConsumeStdOut(attach.Reader, onStdOutExit)
	_ = d.startConsumeStdIn(attach.Conn)

	return exec.ID
}

func (d *DockerExecutor) cleanupContainer(cli *client.Client, cid string) {
	option := d.inCmd.Docker

	if option.IsDeleteContainer {
		err := cli.ContainerRemove(d.context, cid, types.ContainerRemoveOptions{Force: true})
		if !util.LogIfError(err) {
			util.LogInfo("Container %s for cmd %s has been deleted", cid, d.CmdID())
		}
		return
	}

	if option.IsStopContainer {
		err := cli.ContainerStop(d.context, cid, nil)
		if !util.LogIfError(err) {
			util.LogInfo("Container %s for cmd %s has been stopped", cid, d.CmdID())
		}
	}
}

func (d *DockerExecutor) waitForExit(cli *client.Client, eid string) int {
	inspect, err := cli.ContainerExecInspect(d.context, eid)
	util.PanicIfErr(err)
	d.toStartStatus(inspect.Pid)

	for {
		inspect, err = cli.ContainerExecInspect(d.context, eid)
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

func tarArchiveFromPath(path string) (io.Reader, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	ok := filepath.Walk(path, func(file string, fi os.FileInfo, err error) (out error) {
		defer func() {
			if err := recover(); err != nil {
				out = err.(error)
			}
		}()

		util.PanicIfErr(err)

		header, err := tar.FileInfoHeader(fi, fi.Name())
		util.PanicIfErr(err)

		header.Name = strings.TrimPrefix(strings.Replace(file, path, "", -1), string(filepath.Separator))
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
