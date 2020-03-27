package executor

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github/flowci/flow-agent-x/config"
	"github/flowci/flow-agent-x/domain"
	"github/flowci/flow-agent-x/util"
	"testing"
)

func init() {
	util.EnableDebugLog()
}

func TestShouldPullImage(t *testing.T) {
	assert := assert.New(t)

	// init:
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app := config.GetInstance()
	app.Workspace = getTestDataDir()

	// when:
	cmd := createDockerTestCmd()
	cmd.FlowId = "flowid" // same as dir flowid in _testdata

	executor := NewExecutor(Docker, ctx, cmd, nil)
	go printLog(executor.LogChannel())

	err := executor.Start()
	assert.NoError(err)

}

func createDockerTestCmd() *domain.CmdIn {
	return &domain.CmdIn{
		Cmd: domain.Cmd{
			ID: "1-1-1",
		},
		Scripts: []string{
			"echo bbb",
			"sleep 5",
			">&2 echo $INPUT_VAR",
			"export FLOW_VVV=flowci",
			"export FLOW_AAA=flow...",
		},
		Inputs:     domain.Variables{"INPUT_VAR": "aaa"},
		Timeout:    1800,
		EnvFilters: []string{"FLOW_"},
		Docker: &domain.DockerDesc{
			Image:      "ubuntu:18.04",
			Entrypoint: []string{"/bin/bash"},
		},
	}
}
