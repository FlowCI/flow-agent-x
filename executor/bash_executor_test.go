package executor

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github/flowci/flow-agent-x/domain"
	"github/flowci/flow-agent-x/util"
	"testing"
	"time"
)

func init() {
	util.EnableDebugLog()
}

func TestShouldExitAfterExecuted(t *testing.T) {
	assert := assert.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	executor := NewExecutor(Bash, ctx, createBashTestCmd(), nil)

	time.AfterFunc(2 * time.Second, func() {
		executor.BashChannel() <- "echo $HOME"
		executor.BashChannel() <- "go version"
		executor.BashChannel() <- "export HELLO_WORLD='hello'"
		executor.BashChannel() <- "exit"
	})

	go printLog(executor.LogChannel())

	err := executor.Start()
	assert.Nil(err)

	result := executor.GetResult()
	assert.Equal(domain.CmdStatusSuccess, result)
	assert.Equal(int64(2), result.LogSize)
	assert.Equal(0, result.Code)
	assert.NotNil(result.FinishAt)
}

func TestShouldExecuteFromInCmd(t *testing.T) {
	assert := assert.New(t)

	testCmd := createBashTestCmd()
	testCmd.Scripts = []string{"echo $HOME", "export HELLO_WORLD='hello'"}
	testCmd.EnvFilters = []string{"HELLO_WORLD"}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	executor := NewExecutor(Bash, ctx, testCmd, nil)
	go printLog(executor.LogChannel())

	err := executor.Start()
	assert.Nil(err)

	result := executor.GetResult()
	assert.Equal(domain.CmdStatusSuccess, result.Status)
	assert.Equal(int64(1), result.LogSize)
	assert.Equal(0, result.Code)
	assert.Equal("hello", result.Output["HELLO_WORLD"])
	assert.NotNil(result.FinishAt)
}

func TestShouldExitWithErrorAfterExecuted(t *testing.T) {
	assert := assert.New(t)

	// init:
	testCmd := createBashTestCmd()
	testCmd.AllowFailure = false
	testCmd.Scripts = []string{"notCommand"}

	// when:
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	executor := NewExecutor(Bash, ctx, testCmd, nil)
	go printLog(executor.LogChannel())

	err := executor.Start()
	assert.Nil(err)

	// then:
	assert.Equal(int64(1), executor.GetResult().LogSize)
	assert.Equal(127, executor.GetResult().Code)
	assert.Equal(domain.CmdStatusException, executor.GetResult().Status)
	assert.NotNil(executor.GetResult().FinishAt)
}

func TestShouldExitAfterExecutedButAllowFailure(t *testing.T) {
	assert := assert.New(t)

	// init:
	testCmd := createBashTestCmd()
	testCmd.AllowFailure = true
	testCmd.Scripts = []string{"notCommand"}

	// when:
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	executor := NewExecutor(Bash, ctx, testCmd, nil)
	go printLog(executor.LogChannel())

	err := executor.Start()
	assert.Nil(err)

	// then:
	assert.Equal(int64(1), executor.GetResult().LogSize)
	assert.Equal(0, executor.GetResult().Code)
	assert.Equal(domain.CmdStatusSuccess, executor.GetResult().Status)
	assert.NotNil(executor.GetResult().FinishAt)
}

func TestShouldExitWithTimeout(t *testing.T) {
	assert := assert.New(t)

	testCmd := createBashTestCmd()
	testCmd.Timeout = 5
	testCmd.Scripts = []string{"echo $HOME", "sleep 9999", "echo ...."}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	executor := NewExecutor(Bash, ctx, testCmd, nil)

	go printLog(executor.LogChannel())

	err := executor.Start()
	assert.Nil(err)

	assert.Equal(domain.CmdStatusTimeout, executor.GetResult().Status)
	assert.Equal(domain.CmdExitCodeTimeOut, executor.GetResult().Code)
	assert.Equal(int64(1), executor.GetResult().LogSize)
	assert.NotNil(executor.GetResult().FinishAt)
}

func TestShouldExitByKill(t *testing.T) {
	assert := assert.New(t)

	testCmd := createBashTestCmd()
	testCmd.Scripts = []string{"echo $HOME", "sleep 9999", "echo ...."}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	executor := NewExecutor(Bash, ctx, testCmd, nil)

	go printLog(executor.LogChannel())

	time.AfterFunc(2 * time.Second, func() {
		executor.Kill()
	})

	err := executor.Start()
	assert.Nil(err)

	assert.Equal(domain.CmdStatusKilled, executor.GetResult().Status)
	assert.Equal(domain.CmdExitCodeKilled, executor.GetResult().Code)
	assert.Equal(int64(1), executor.GetResult().LogSize)
	assert.NotNil(executor.GetResult().FinishAt)
}

func createBashTestCmd() *domain.CmdIn {
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
	}
}
