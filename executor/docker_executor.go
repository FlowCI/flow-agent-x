package executor

import (
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"io"
	"os"
)

type (
	DockerExecutor struct {
		BaseExecutor
	}
)

func (e *DockerExecutor) Start() error {
	// pull image
	cli, err := client.NewEnvClient()
	if err != nil {
		return err
	}

	reader, err := cli.ImagePull(e.context, "docker.io/library/ubuntu:18.04", types.ImagePullOptions{})
	if err != nil {
		return err
	}

	io.Copy(os.Stdout, reader)

	// start container

	// cp job dir

	// for : run cmd

	return nil
}

func (e *DockerExecutor) Kill() {

}
