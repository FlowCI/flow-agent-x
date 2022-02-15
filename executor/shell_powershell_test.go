//go:build windows
// +build windows

package executor

import (
	"encoding/base64"
	"github.com/flowci/flow-agent-x/domain"
	"github.com/flowci/flow-agent-x/util"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestShouldExecInPowerShell(t *testing.T) {
	assert := assert.New(t)

	cmdIn := createPowerShellTestCmd()
	shouldExecCmd(assert, cmdIn)
}

func TestShouldExecWithErrorInPowerShell(t *testing.T) {
	assert := assert.New(t)
	cmd := createPowerShellTestCmd()
	shouldExecWithError(assert, cmd)
}

func TestShouldExecWithErrorButAllowFailureInPowerShell(t *testing.T) {
	assert := assert.New(t)
	cmd := createPowerShellTestCmd()
	shouldExecWithErrorButAllowFailure(assert, cmd)
}

func TestShouldExecButTimeoutInPowerShell(t *testing.T) {
	assert := assert.New(t)
	cmd := createPowerShellTestCmd()
	shouldExecButTimeOut(assert, cmd)
}

func TestShouldExitByKillInPowerShell(t *testing.T) {
	assert := assert.New(t)
	cmd := createPowerShellTestCmd()
	shouldExecButKilled(assert, cmd)
}

func TestShouldStartTtyInPowerShell(t *testing.T) {
	assert := assert.New(t)

	executor := newExecutor(&domain.ShellIn{
		ID:     "test111",
		FlowId: "test111",
		Scripts: []string{
			"echo hello",
		},
		Timeout: 9999,
	}, false)

	go func() {
		for {
			log, ok := <-executor.TtyOut()
			if !ok {
				return
			}

			content, _ := base64.StdEncoding.DecodeString(log)
			util.LogDebug("------ %s", content)
		}
	}()

	go func() {
		time.Sleep(2 * time.Second)
		executor.TtyIn() <- "cd ~/\r\n"
		executor.TtyIn() <- "ls\r\n"
		time.Sleep(2 * time.Second)
		executor.TtyIn() <- "exit\r\n"
	}()

	err := executor.StartTty("fakeId", func(ttyId string) {
		util.LogDebug("Tty Started")
	})
	assert.NoError(err)
	assert.False(executor.IsInteracting())
}

func createPowerShellTestCmd() *domain.ShellIn {
	return &domain.ShellIn{
		CmdIn: domain.CmdIn{
			Type: domain.CmdTypeShell,
		},
		ID: "1-1-1",
		Scripts: []string{
			"echo bbb",
			"sleep 5",
			"$env:FLOW_VVV=\"flowci\"",
			"$env:FLOW_AAA=\"flow...\"",
		},
		Inputs:     domain.Variables{"INPUT_VAR": "aaa"},
		Timeout:    1800,
		EnvFilters: []string{"FLOW_"},
	}
}
