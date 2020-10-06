// +build windows

package executor

import (
	"github.com/stretchr/testify/assert"
	"github/flowci/flow-agent-x/domain"
	"testing"
)

func TestPowerShellShouldExecuteCmd(t *testing.T) {
	assert := assert.New(t)

	cmdIn := createPowerShellTestCmd()
	shouldExecCmd(assert, cmdIn)
}

func createPowerShellTestCmd() *domain.ShellIn {
	return &domain.ShellIn{
		CmdIn: domain.CmdIn{
			Type: domain.CmdTypeShell,
		},
		ID: "1-1-1",
		Scripts: []string{
			"echo ${home}",
			"$env:FLOW_VVV=\"flowci\"",
			"$env:FLOW_AAA=\"flow...\"",
		},
		Inputs:     domain.Variables{"INPUT_VAR": "aaa"},
		Timeout:    1800,
		EnvFilters: []string{"FLOW_"},
	}
}
