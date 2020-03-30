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
