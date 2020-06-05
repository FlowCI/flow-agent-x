package executor

import (
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

	executor := newExecutor(&domain.ShellCmd{
		ID:     "test111",
		FlowId: "test111",
		Scripts: []string{
			"echo hello",
		},
		Timeout: 9999,
	})

	go func() {
		for {
			log, ok := <-executor.OutputStream()
			if !ok {
				return
			}
			util.LogDebug(log.Output)
		}
	}()

	go func() {
		time.Sleep(2 * time.Second)
		executor.InputStream() <- &domain.TtyIn{
			Script: "ls\n",
		}
		time.Sleep(2 * time.Second)
		executor.InputStream() <- &domain.TtyIn{
			Script: "exit\n",
		}
	}()

	err := executor.StartTty()
	assert.NoError(err)
	assert.False(executor.IsInteracting())
}

func createBashTestCmd() *domain.ShellCmd {
	return &domain.ShellCmd{
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
