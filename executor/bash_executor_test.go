package executor

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github/flowci/flow-agent-x/domain"
	"github/flowci/flow-agent-x/util"
	"testing"
	"time"
)

func TestShouldExitAfterExecuted(t *testing.T) {
	assert := assert.New(t)

	ctx, _ := context.WithCancel(context.Background())
	executor := NewBashExecutor(ctx, createTestCmd(), nil)

	time.AfterFunc(2 * time.Second, func() {
		executor.BashChannel() <- "echo $HOME"
		executor.BashChannel() <- "go version"
		executor.BashChannel() <- "export HELLO_WORLD='hello'"
		executor.BashChannel() <- "exit"
	})

	go printLog(executor.LogChannel())

	err := executor.Start()
	assert.Nil(err)

	assert.Equal(domain.CmdStatusSuccess, executor.CmdResult.Status)
	assert.Equal(int64(2), executor.CmdResult.LogSize)
	assert.Equal(0, executor.CmdResult.Code)
	assert.NotNil(executor.CmdResult.FinishAt)
}

func TestShouldExecuteFromInCmd(t *testing.T) {
	assert := assert.New(t)

	testCmd := createTestCmd()
	testCmd.Scripts = []string{"echo $HOME", "export HELLO_WORLD='hello'"}
	testCmd.EnvFilters = []string{"HELLO_WORLD"}

	ctx, _ := context.WithCancel(context.Background())
	executor := NewBashExecutor(ctx, testCmd, nil)
	go printLog(executor.LogChannel())

	err := executor.Start()
	assert.Nil(err)

	assert.Equal(domain.CmdStatusSuccess, executor.CmdResult.Status)
	assert.Equal(int64(1), executor.CmdResult.LogSize)
	assert.Equal(0, executor.CmdResult.Code)
	assert.Equal("hello", executor.CmdResult.Output["HELLO_WORLD"])
	assert.NotNil(executor.CmdResult.FinishAt)
}

func TestShouldExitWithErrorAfterExecuted(t *testing.T) {
	assert := assert.New(t)

	// init:
	testCmd := createTestCmd()
	testCmd.AllowFailure = false
	testCmd.Scripts = []string{"notCommand"}

	// when:
	ctx, _ := context.WithCancel(context.Background())
	executor := NewBashExecutor(ctx, testCmd, nil)
	go printLog(executor.LogChannel())

	err := executor.Start()
	assert.Nil(err)

	// then:
	assert.Equal(int64(1), executor.CmdResult.LogSize)
	assert.Equal(127, executor.CmdResult.Code)
	assert.Equal(domain.CmdStatusException, executor.CmdResult.Status)
	assert.NotNil(executor.CmdResult.FinishAt)
}

func TestShouldExitAfterExecutedButAllowFailure(t *testing.T) {
	assert := assert.New(t)

	// init:
	testCmd := createTestCmd()
	testCmd.AllowFailure = true
	testCmd.Scripts = []string{"notCommand"}

	// when:
	ctx, _ := context.WithCancel(context.Background())
	executor := NewBashExecutor(ctx, testCmd, nil)
	go printLog(executor.LogChannel())

	err := executor.Start()
	assert.Nil(err)

	// then:
	assert.Equal(int64(1), executor.CmdResult.LogSize)
	assert.Equal(0, executor.CmdResult.Code)
	assert.Equal(domain.CmdStatusSuccess, executor.CmdResult.Status)
	assert.NotNil(executor.CmdResult.FinishAt)
}

func TestShouldExitWithTimeout(t *testing.T) {
	assert := assert.New(t)

	testCmd := createTestCmd()
	testCmd.Timeout = 5
	testCmd.Scripts = []string{"echo $HOME", "sleep 9999", "echo ...."}

	ctx, _ := context.WithCancel(context.Background())
	executor := NewBashExecutor(ctx, testCmd, nil)

	go printLog(executor.LogChannel())

	err := executor.Start()
	assert.Nil(err)

	assert.Equal(domain.CmdStatusTimeout, executor.CmdResult.Status)
	assert.Equal(domain.CmdExitCodeTimeOut, executor.CmdResult.Code)
	assert.Equal(int64(1), executor.CmdResult.LogSize)
	assert.NotNil(executor.CmdResult.FinishAt)
}

func TestShouldExitByKill(t *testing.T) {
	assert := assert.New(t)

	testCmd := createTestCmd()
	testCmd.Scripts = []string{"echo $HOME", "sleep 9999", "echo ...."}

	ctx, _ := context.WithCancel(context.Background())
	executor := NewBashExecutor(ctx, testCmd, nil)

	go printLog(executor.LogChannel())

	time.AfterFunc(2 * time.Second, func() {
		executor.Kill()
	})

	err := executor.Start()
	assert.Nil(err)

	assert.Equal(domain.CmdStatusKilled, executor.CmdResult.Status)
	assert.Equal(domain.CmdExitCodeKilled, executor.CmdResult.Code)
	assert.Equal(int64(1), executor.CmdResult.LogSize)
	assert.NotNil(executor.CmdResult.FinishAt)
}

func createTestCmd() *domain.CmdIn {
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

func printLog(channel <-chan *domain.LogItem) {
	for {
		item, ok := <-channel
		if !ok {
			break
		}
		util.LogDebug("[LOG]: %s", item.Content)
	}
}
