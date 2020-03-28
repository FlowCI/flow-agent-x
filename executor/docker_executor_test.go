package executor

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github/flowci/flow-agent-x/config"
	"github/flowci/flow-agent-x/domain"
	"github/flowci/flow-agent-x/util"
	"testing"
	"time"
)

func init() {
	app := config.GetInstance()
	app.Workspace = getTestDataDir()
	util.EnableDebugLog()
}

func TestShouldExecInDocker(t *testing.T) {
	assert := assert.New(t)

	// init:
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// when:
	cmd := createDockerTestCmd()
	executor := NewExecutor(Docker, ctx, cmd, nil)
	go printLog(executor.LogChannel())

	err := executor.Start()
	assert.NoError(err)

	// then:
	result := executor.GetResult()
	assert.Equal(0, result.Code)
	assert.Equal(int64(2), result.LogSize)
	assert.NotNil(result.FinishAt)
	assert.Equal("flowci", result.Output["FLOW_VVV"])
	assert.Equal("flow...", result.Output["FLOW_AAA"])
}

func TestShouldExecAndExitIfErrorInDocker(t *testing.T) {
	assert := assert.New(t)

	// init:
	cmd := createDockerTestCmd()
	cmd.AllowFailure = false
	cmd.Scripts = []string{"notCommand should exit with error"}

	// when:
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	executor := NewExecutor(Docker, ctx, cmd, nil)
	go printLog(executor.LogChannel())

	err := executor.Start()
	assert.Nil(err)

	// then:
	assert.Equal(int64(1), executor.GetResult().LogSize)
	assert.Equal(127, executor.GetResult().Code)
	assert.Equal(domain.CmdStatusException, executor.GetResult().Status)
	assert.NotNil(executor.GetResult().FinishAt)
}

func TestShouldExecWithErrorIfAllowFailureWithinDocker(t *testing.T) {
	assert := assert.New(t)

	// init:
	testCmd := createDockerTestCmd()
	testCmd.AllowFailure = true
	testCmd.Scripts = []string{"notCommand should exit with error"}

	// when:
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	executor := NewExecutor(Docker, ctx, testCmd, nil)
	go printLog(executor.LogChannel())

	err := executor.Start()
	assert.Nil(err)

	// then:
	assert.Equal(int64(1), executor.GetResult().LogSize)
	assert.Equal(0, executor.GetResult().Code)
	assert.Equal(domain.CmdStatusSuccess, executor.GetResult().Status)
	assert.NotNil(executor.GetResult().FinishAt)
}

func TestShouldExitWithTimeoutInDocker(t *testing.T) {
	assert := assert.New(t)

	testCmd := createDockerTestCmd()
	testCmd.Timeout = 5
	testCmd.Scripts = []string{"echo $HOME", "sleep 9999", "echo ...."}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	executor := NewExecutor(Docker, ctx, testCmd, nil)

	go printLog(executor.LogChannel())

	err := executor.Start()
	assert.Equal(context.DeadlineExceeded, err)

	assert.Equal(domain.CmdStatusTimeout, executor.GetResult().Status)
	assert.Equal(domain.CmdExitCodeTimeOut, executor.GetResult().Code)
	assert.Equal(int64(1), executor.GetResult().LogSize)
	assert.NotNil(executor.GetResult().FinishAt)
}

func TestShouldExitByKillInDocker(t *testing.T) {
	assert := assert.New(t)

	testCmd := createDockerTestCmd()
	testCmd.Scripts = []string{"echo $HOME", "sleep 9999", "echo ...."}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	executor := NewExecutor(Docker, ctx, testCmd, nil)
	go printLog(executor.LogChannel())

	time.AfterFunc(5 * time.Second, func() {
		executor.Kill()
	})

	err := executor.Start()
	assert.Equal(context.Canceled, err)

	assert.Equal(domain.CmdStatusKilled, executor.GetResult().Status)
	assert.Equal(domain.CmdExitCodeKilled, executor.GetResult().Code)
	assert.Equal(int64(1), executor.GetResult().LogSize)
	assert.NotNil(executor.GetResult().FinishAt)
}

func createDockerTestCmd() *domain.CmdIn {
	return &domain.CmdIn{
		FlowId: "flowid", // same as dir flowid in _testdata
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
		Docker: &domain.DockerOption{
			Image:             "ubuntu:18.04",
			Entrypoint:        []string{"/bin/bash"},
			IsDeleteContainer: true,
			IsStopContainer:   true,
		},
	}
}
