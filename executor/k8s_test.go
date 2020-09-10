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

func TestShouldExecInK8s(t *testing.T) {
	assert := assert.New(t)
	cmd := createK8sTestCmd()

	executor := newExecutor(cmd, true)
	assert.NotNil(executor)

	go printLog(executor.Stdout())

	// init executor
	err := executor.Init()
	assert.NoError(err)

	// start pod
	err = executor.Start()
	assert.NoError(err)

	assert.Equal(0, executor.GetResult().Code)
	assert.True(executor.GetResult().ProcessId > 0)

	// verify output
	output := executor.GetResult().Output
	assert.Equal("flowci", output["FLOW_VVV"])
	assert.Equal("flow...", output["FLOW_AAA"])
}

func createK8sTestCmd() *domain.ShellIn {
	return &domain.ShellIn{
		CmdIn: domain.CmdIn{
			Type: domain.CmdTypeShell,
		},
		FlowId: "flowid", // same as dir flowid in _testdata
		ID:     "1-1-1",
		Dockers: []*domain.DockerOption{
			{
				Image:      "flowci/debian-docker",
				Name: 		"helloworld-run-from-docker-r9o7pxm",
				IsRuntime:  true,
				Environment: map[string]string{
					"VAR_HELLO": "WORLD",
				},
			},
			{
				Image: "nginx",
				Name:  "helloworld-step-1-xa2adf",
				Ports: []string{"80:80"},
			},
		},
		Scripts: []string{
			"echo bbb",
			"sleep 5",
			">&2 echo $INPUT_VAR",
			"python3 -V",
			"export FLOW_VVV=flowci",
			"export FLOW_AAA=flow...",
		},
		Inputs:     domain.Variables{"INPUT_VAR": "aaa"},
		Timeout:    1800,
		EnvFilters: []string{"FLOW_"},
	}
}
