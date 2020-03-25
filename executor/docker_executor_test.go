package executor

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github/flowci/flow-agent-x/domain"
	"testing"
)

func TestShouldPullImage(t *testing.T) {
	assert := assert.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	executor := NewExecutor(Docker, ctx, createDockerTestCmd(), nil)
	err := executor.Start()
	assert.NoError(err)

}

func createDockerTestCmd() *domain.CmdIn {
	return &domain.CmdIn{
		Cmd: domain.Cmd{
			ID: "1-1-1",
		},
		Scripts: []string{
			"set -e",
			"echo bbb",
			"sleep 5",
			">&2 echo $INPUT_VAR",
			"export FLOW_VVV=flowci",
			"export FLOW_AAA=flow...",
		},
		Inputs:     domain.Variables{"INPUT_VAR": "aaa"},
		Timeout:    1800,
		EnvFilters: []string{"FLOW_"},
		Docker: 	&domain.DockerDesc{
			Image: "ubuntu:18.04",
		},
	}
}
