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
	shouldExecCmd(assert, cmd, Docker)
}

func TestShouldExecWithErrorInDocker(t *testing.T) {
	assert := assert.New(t)
	cmd := createDockerTestCmd()
	shouldExecWithError(assert, cmd, Docker)
}

func TestShouldExecWithErrorIfAllowFailureWithinDocker(t *testing.T) {
	assert := assert.New(t)
	cmd := createDockerTestCmd()
	shouldExecWithErrorButAllowFailure(assert, cmd, Docker)
}

func TestShouldExitWithTimeoutInDocker(t *testing.T) {
	assert := assert.New(t)
	cmd := createDockerTestCmd()
	shouldExecButTimeOut(assert, cmd, Docker)
}

func TestShouldExitByKillInDocker(t *testing.T) {
	assert := assert.New(t)
	cmd := createDockerTestCmd()
	shouldExecButKilled(assert, cmd, Docker)
}

func createDockerTestCmd() *domain.CmdIn {
	return &domain.CmdIn{
		FlowId: "flowid", // same as dir flowid in _testdata
		Cmd: domain.Cmd{
			ID: "1-1-1",
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
		Docker: &domain.DockerOption{
			Image:             "ubuntu:18.04",
			Entrypoint:        []string{"/bin/bash"},
			IsDeleteContainer: true,
			IsStopContainer:   true,
		},
	}
}
