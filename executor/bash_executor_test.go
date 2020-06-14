package executor

import (
	"encoding/base64"
	"github.com/stretchr/testify/assert"
	"github/flowci/flow-agent-x/domain"
	"github/flowci/flow-agent-x/util"
	"testing"
	"time"
)

func init() {
	util.EnableDebugLog()
}

func TestShouldExecInBash(t *testing.T) {
	assert := assert.New(t)
	cmd := createBashTestCmd()
	//
	//ok, _ := hasPyenv()
	//assert.True(ok)

	shouldExecCmd(assert, cmd)
}

func TestShouldExecWithErrorInBash(t *testing.T) {
	assert := assert.New(t)
	cmd := createBashTestCmd()
	shouldExecWithError(assert, cmd)
}

func TestShouldExecWithErrorButAllowFailureInBash(t *testing.T) {
	assert := assert.New(t)
	cmd := createBashTestCmd()
	shouldExecWithErrorButAllowFailure(assert, cmd)
}

func TestShouldExecButTimeout(t *testing.T) {
	assert := assert.New(t)
	cmd := createBashTestCmd()
	shouldExecButTimeOut(assert, cmd)
}

func TestShouldExitByKill(t *testing.T) {
	assert := assert.New(t)
	cmd := createBashTestCmd()
	shouldExecButKilled(assert, cmd)
}

func TestShouldStartBashInteract(t *testing.T) {
	assert := assert.New(t)

	executor := newExecutor(&domain.ShellIn{
		ID:     "test111",
		FlowId: "test111",
		Scripts: []string{
			"echo hello",
		},
		Timeout: 9999,
	})

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
		executor.TtyIn() <- "cd ~/\n"
		executor.TtyIn() <- "ls -l\n"
		time.Sleep(2 * time.Second)
		executor.TtyIn() <- string([]byte{4})
	}()

	err := executor.StartTty("fakeId", func(ttyId string) {
		util.LogDebug("Tty Started")
	})
	assert.NoError(err)
	assert.False(executor.IsInteracting())
}

func createBashTestCmd() *domain.ShellIn {
	return &domain.ShellIn{
		CmdIn: domain.CmdIn{
			Type: domain.CmdTypeShell,
		},
		ID: "1-1-1",
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
