// +build windows

package executor

import (
	"github.com/stretchr/testify/assert"
	"github/flowci/flow-agent-x/domain"
	"testing"
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
