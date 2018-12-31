package cmd

import (
	"testing"

	"github.com/flowci/flow-agent-x/domain"
	"github.com/stretchr/testify/assert"
)

func TestShouldRunLinuxShell(t *testing.T) {
	assert := assert.New(t)

	// init: cmd
	cmd := &domain.CmdIn{}
	cmd.ID = "1-1-1"
	cmd.Scripts = []string{"echo bbb", "sleep 5", "echo aaa"}

	// when: new shell executor and run
	executor := NewShellExecutor(cmd)
	defer executor.Close()

	err := executor.Run()
	assert.Nil(err)

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
