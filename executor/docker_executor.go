package executor

import (
	"bufio"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github/flowci/flow-agent-x/domain"
	"github/flowci/flow-agent-x/util"
	"io"
	"strings"
)

type (
	DockerExecutor struct {
		BaseExecutor
	}
)

func (e *DockerExecutor) Start() error {
	// pull image
	cli, err := client.NewEnvClient()
	util.PanicIfErr(err)
	defer cli.Close()

	e.pullImage(cli)

	e.createContainer(cli)

	// cp job dir

	// for : run cmd

	return nil
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

func (e *DockerExecutor) createContainer(cli *client.Client) {
	docker := e.inCmd.Docker

	resp, err := cli.ContainerCreate(
		e.context,
		&container.Config{
			Image:      docker.Image,
			Entrypoint: docker.Entrypoint,
			Cmd:        []string{"echo", "hello world"},
			Tty:        false,
		},
		nil, nil, "")

	util.PanicIfErr(err)

}

func (e *DockerExecutor) writeLogToChannel(reader io.ReadCloser) {
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
