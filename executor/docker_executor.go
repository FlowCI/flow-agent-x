package executor

import (
	"archive/tar"
	"bufio"
	"bytes"
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

func (e *DockerExecutor) Start() (out error) {
	defer func() {
		if err := recover(); err != nil {
			out = err.(error)
		}
	}()

	e.workDirInContainer = "/ws/" + e.inCmd.FlowId
	e.inVars[domain.VarAgentJobDir] = e.workDirInContainer

	// pull image
	cli, err := client.NewEnvClient()
	util.PanicIfErr(err)
	defer cli.Close()

	e.pullImage(cli)
	cid := e.createContainer(cli)

	e.startContainer(cli, cid)
	e.copyToContainer(cli, cid)
	e.runCmdInContainer(cli, cid)

	return
}

func (e *DockerExecutor) Kill() {

}

//--------------------------------------------
// private methods
//--------------------------------------------

func (e *DockerExecutor) pullImage(cli *client.Client) {
	fullRef := "docker.io/library/" + e.inCmd.Docker.Image
	reader, err := cli.ImagePull(e.context, fullRef, types.ImagePullOptions{})

	util.PanicIfErr(err)
	e.writeLogToChannel(reader)
}

func (e *DockerExecutor) createContainer(cli *client.Client) string {
	docker := e.inCmd.Docker

	config := &container.Config{
		Image:        docker.Image,
		Env:          append(e.inVars.ToStringArray(), e.inCmd.VarsToStringArray()...),
		Entrypoint:   docker.Entrypoint,
		Tty:          false,
		AttachStdin:  true,
		AttachStderr: true,
		AttachStdout: true,
		OpenStdin:    true,
		StdinOnce:    true,
		WorkingDir:   e.workDirInContainer,
	}

	resp, err := cli.ContainerCreate(e.context, config, nil, nil, "")
	util.PanicIfErr(err)
	util.LogDebug("Container created %s", resp.ID)

	return resp.ID
}

func (e *DockerExecutor) startContainer(cli *client.Client, cid string) {
	err := cli.ContainerStart(e.context, cid, types.ContainerStartOptions{})
	util.PanicIfErr(err)
}

func (e *DockerExecutor) copyToContainer(cli *client.Client, containerId string) {
	reader, err := tarArchiveFromPath(e.workDir)
	util.PanicIfErr(err)

	config := types.CopyToContainerOptions{
		AllowOverwriteDirWithFile: true,
	}

	err = cli.CopyToContainer(e.context, containerId, e.workDirInContainer, reader, config)
	util.PanicIfErr(err)
	util.LogDebug("Job working dir been created in container")
}

func (e *DockerExecutor) runCmdInContainer(cli *client.Client, cid string) int {
	config := types.ContainerAttachOptions{
		Stream: true,
		Stdin:  true,
		Stdout: true,
		Stderr: true,
		Logs:   true,
	}

	attach, err := cli.ContainerAttach(e.context, cid, config)
	util.PanicIfErr(err)

	go func() {
		io.Copy(os.Stdout, attach.Reader)
	}()

	go func() {
		attach.Conn.Write([]byte("cd /etc\n"))
		attach.Conn.Write([]byte("ls -al\n"))
		attach.Conn.Write([]byte("echo hello 11111111111\n"))
		attach.Conn.Write([]byte("echo hello 11111111111\n"))
		attach.Conn.Write([]byte("echo hello 11111111111\n"))
		attach.Conn.Write([]byte("echo hello 11111111111\n"))

	}()

	time.Sleep(5 * time.Second)
	_, err = cli.ContainerInspect(e.context, cid)
	util.PanicIfErr(err)

	return 0
}

func (e *DockerExecutor) writeLogToChannel(reader io.Reader) {
	bufferReader := bufio.NewReaderSize(reader, defaultReaderBufferSize)
	var builder strings.Builder

	for {
		line, err := readLine(bufferReader, builder)
		builder.Reset()

		if err != nil {
			return
		}

		log := &domain.LogItem{
			CmdID:   e.CmdID(),
			Content: line,
		}

		e.logChannel <- log
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
