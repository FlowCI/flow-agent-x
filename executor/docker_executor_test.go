package executor

import (
	"github.com/stretchr/testify/assert"
	"github/flowci/flow-agent-x/config"
	"github/flowci/flow-agent-x/domain"
	"github/flowci/flow-agent-x/util"
	"testing"
)

func init() {
	app := config.GetInstance()
	app.Workspace = getTestDataDir()
	util.EnableDebugLog()
}

func TestShouldExecInDocker(t *testing.T) {
	assert := assert.New(t)
	cmd := createDockerTestCmd()
	shouldExecCmd(assert, cmd)
}

func TestShouldExecWithErrorInDocker(t *testing.T) {
	assert := assert.New(t)
	cmd := createDockerTestCmd()
	shouldExecWithError(assert, cmd)
}

func TestShouldExecWithErrorIfAllowFailureWithinDocker(t *testing.T) {
	assert := assert.New(t)
	cmd := createDockerTestCmd()
	shouldExecWithErrorButAllowFailure(assert, cmd)
}

func TestShouldExitWithTimeoutInDocker(t *testing.T) {
	assert := assert.New(t)
	cmd := createDockerTestCmd()
	shouldExecButTimeOut(assert, cmd)
}

func TestShouldExitByKillInDocker(t *testing.T) {
	assert := assert.New(t)
	cmd := createDockerTestCmd()
	shouldExecButKilled(assert, cmd)
}

func TestShouldReuseContainer(t *testing.T) {
	assert := assert.New(t)

	// run cmd in container
	cmd := createDockerTestCmd()
	cmd.Docker.IsStopContainer = true
	cmd.Docker.IsDeleteContainer = false

	result := shouldExecCmd(assert, cmd)
	assert.NotEmpty(result.ContainerId)

	// run cmd in container from first step
	cmd = createDockerTestCmd()
	cmd.ContainerId = result.ContainerId
	cmd.Docker.IsStopContainer = true
	cmd.Docker.IsDeleteContainer = true

	resultFromReuse := shouldExecCmd(assert, cmd)
	assert.NotEmpty(resultFromReuse.ContainerId)
	assert.Equal(result.ContainerId, resultFromReuse.ContainerId)
}

func createDockerTestCmd() *domain.ShellCmd {
	return &domain.ShellCmd{
		CmdIn: domain.CmdIn{
			Type: domain.CmdTypeShell,
		},
		FlowId: "flowid", // same as dir flowid in _testdata
		ID: "1-1-1",
		Docker: &domain.DockerOption{
			Image:             "ubuntu:18.04",
			Entrypoint:        []string{"/bin/bash"},
			IsDeleteContainer: true,
			IsStopContainer:   true,
		},
		Scripts: []string{
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
