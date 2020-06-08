package executor

import (
	"github.com/stretchr/testify/assert"
	"github/flowci/flow-agent-x/config"
	"github/flowci/flow-agent-x/domain"
	"github/flowci/flow-agent-x/util"
	"testing"
	"time"
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

func TestShouldStartDockerInteract(t *testing.T) {
	assert := assert.New(t)

	executor := newExecutor(&domain.ShellIn{
		ID:     "test111",
		FlowId: "test111",
		Scripts: []string{
			"echo hello",
			"sleep 9999",
		},
		Docker: &domain.DockerOption{
			Image: "ubuntu:18.04",
		},
		Timeout: 9999,
	})

	dockerExecutor := executor.(*DockerExecutor)
	assert.NotNil(dockerExecutor)

	err := dockerExecutor.Init()
	assert.NoError(err)

	go func() {
		for {
			log, ok := <-executor.OutputStream()
			if !ok {
				return
			}
			util.LogDebug("[INTERACT]: %s", log.Log)
		}
	}()

	go func() {
		time.Sleep(2 * time.Second)
		executor.InputStream() <- "echo helloworld...\n"
		time.Sleep(2 * time.Second)
		executor.InputStream() <- "exit\n"
	}()

	// docker should start container for cmd before tty
	go func() {
		err := executor.Start()
		assert.NoError(err)
	}()

	for {
		if dockerExecutor.containerId == "" {
			time.Sleep(1 * time.Second)
			continue
		}
		break
	}

	err = executor.StartTty("fakeId")
	assert.NoError(err)
	assert.False(executor.IsInteracting())
}

func createDockerTestCmd() *domain.ShellIn {
	return &domain.ShellIn{
		CmdIn: domain.CmdIn{
			Type: domain.CmdTypeShell,
		},
		FlowId: "flowid", // same as dir flowid in _testdata
		ID:     "1-1-1",
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
