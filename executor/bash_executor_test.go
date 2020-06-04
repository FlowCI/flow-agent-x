package executor

import (
	"github.com/stretchr/testify/assert"
	"github/flowci/flow-agent-x/domain"
	"github/flowci/flow-agent-x/util"
	"testing"
)

func init() {
	util.EnableDebugLog()
}

func TestShouldExecInBash(t *testing.T) {
	assert := assert.New(t)
	cmd := createBashTestCmd()

	ok, _ := hasPyenv()
	assert.True(ok)

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
