package cmd

import (
	"testing"

	"github.com/flowci/flow-agent-x/domain"
	"github.com/stretchr/testify/assert"
)

var (
	cmd = &domain.CmdIn{
		Cmd: domain.Cmd{
			ID: "1-1-1",
		},
		Scripts: []string{"echo bbb", "sleep 5", "echo aaa"},
		Timeout: 10,
	}
)

func TestShouldRunLinuxShell(t *testing.T) {
	assert := assert.New(t)

	// when: new shell executor and run
	executor := NewShellExecutor(cmd)
	err := executor.Run()
	assert.Nil(err)

	// then: verfiy result of shell executor
	result := executor.Result
	assert.NotNil(result)
	assert.Equal(0, result.Code)
	assert.False(result.StartAt.IsZero())
	assert.False(result.FinishAt.IsZero())

	// then: verify first log output
	firstLog := <-executor.LogChannel
	assert.Equal(cmd.ID, firstLog.CmdID)
	assert.Equal("bbb", firstLog.Content)
	assert.Equal(int64(1), firstLog.Number)

	// then: verify second of log output
	secondLog := <-executor.LogChannel
	assert.Equal(cmd.ID, secondLog.CmdID)
	assert.Equal("aaa", secondLog.Content)
	assert.Equal(int64(2), secondLog.Number)
}

func TestShouldRunLinuxShellWithTimeOut(t *testing.T) {
	assert := assert.New(t)

	// init: cmd with timeout
	cmd.Timeout = 2

	// when: new shell executor and run
	executor := NewShellExecutor(cmd)
	err := executor.Run()
	assert.Nil(err)

	result := executor.Result
	assert.NotNil(result)

	// then:
	assert.False(result.StartAt.IsZero())
	assert.False(result.FinishAt.IsZero())
	assert.True(result.ProcessId > 0)
	assert.Equal(domain.CmdExitCodeTimeOut, result.Code)
}

func TestShouldCmdNotFoundErr(t *testing.T) {
	assert := assert.New(t)

	// init:
	cmd.Scripts = []string{"notCommand"}

	// when:
	executor := NewShellExecutor(cmd)
	err := executor.Run()
	assert.Nil(err)

	// then:
	assert.Equal(127, executor.Result.Code)
	assert.Equal(domain.CmdStatusException, executor.Result.Status)
}
