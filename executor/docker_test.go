package executor

import (
	"encoding/base64"
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

	cmd.Dockers[0].IsStopContainer = true
	cmd.Dockers[0].IsDeleteContainer = false

	result := shouldExecCmd(assert, cmd)
	assert.Equal(1, len(result.Containers))

	// run cmd in container from first step
	cmd = createDockerTestCmd()
	cmd.Dockers[0].ContainerID = result.Containers[0]
	cmd.Dockers[0].IsStopContainer = true
	cmd.Dockers[0].IsDeleteContainer = true

	resultFromReuse := shouldExecCmd(assert, cmd)
	assert.Equal(1, len(resultFromReuse.Containers))
	assert.Equal(result.Containers[0], resultFromReuse.Containers[0])
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
		Dockers: []*domain.DockerOption{
			{
				Image:     "ubuntu:18.04",
				IsRuntime: true,
			},
		},
		Timeout: 9999,
	}, false)

	dockerExecutor := executor.(*DockerExecutor)
	assert.NotNil(dockerExecutor)

	err := dockerExecutor.Init()
	assert.NoError(err)

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
		executor.TtyIn() <- "echo helloworld...\n"
		time.Sleep(2 * time.Second)
		executor.TtyIn() <- "sleep 9999\n"
		time.Sleep(2 * time.Second)
		executor.TtyIn() <- "exit\n"
	}()

	// docker should start container for cmd before tty
	go func() {
		err := executor.Start()
		assert.NoError(err)
	}()

	for {
		if dockerExecutor.runtime().ContainerID == "" {
			time.Sleep(1 * time.Second)
			continue
		}
		break
	}

	// kill after 10 seconds
	go func() {
		time.Sleep(10 * time.Second)
		executor.StopTty()
	}()

	err = executor.StartTty("fakeId", func(ttyId string) {
		util.LogDebug("Tty started")
	})
	assert.NoError(err)
	assert.False(executor.IsInteracting())
}

func TestShouldRunWithTwoContainers(t *testing.T) {
	assert := assert.New(t)

	cmd := createDockerTestCmd()
	cmd.Dockers = append(cmd.Dockers, &domain.DockerOption{
		Image: "mysql:5.6",
		Environment: map[string]string{
			"MYSQL_ROOT_PASSWORD": "test",
		},
		Ports: []string{"3306:3306"},
		IsDeleteContainer: true,
	})
	cmd.Dockers = append(cmd.Dockers, &domain.DockerOption{
		Image: "mysql:5.6",
		Command: []string{"mysql", "-h127.0.0.1", "-uroot", "-ptest"},
		IsDeleteContainer: true,
	})

	executor := newExecutor(cmd, false)
	assert.NoError(executor.Init())

	err := executor.Start()
	assert.NoError(err)

	r := executor.GetResult()
	assert.Equal(3, len(r.Containers))
}

func createDockerTestCmd() *domain.ShellIn {
	return &domain.ShellIn{
		CmdIn: domain.CmdIn{
			Type: domain.CmdTypeShell,
		},
		FlowId: "flowid", // same as dir flowid in _testdata
		ID:     "1-1-1",
		Dockers: []*domain.DockerOption{
			{
				Image:             "ubuntu:18.04",
				IsDeleteContainer: true,
				IsStopContainer:   true,
				IsRuntime:         true,
			},
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
