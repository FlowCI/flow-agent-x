package executor

import (
	"encoding/base64"
	"github.com/stretchr/testify/assert"
	"github/flowci/flow-agent-x/domain"
	"github/flowci/flow-agent-x/util"
	v1 "k8s.io/api/core/v1"
	"testing"
	"time"
)

func init() {
	util.EnableDebugLog()
}

func TestShouldSetNetworkToHostOnDockerRun(t *testing.T) {
	assert := assert.New(t)

	assert.Equal("docker run --network=host", k8sSetDockerNetwork("docker run"))
	assert.Equal("docker run --network=host -it --rm --privileged ubuntu:18.04 bash", k8sSetDockerNetwork("docker run -it --rm --privileged ubuntu:18.04 bash"))
	assert.Equal("docker run -it --network=host --rm ubuntu:18.04 bash", k8sSetDockerNetwork("docker run -it --network=my-net --rm ubuntu:18.04 bash"))
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

func TestShouldK8sWithInteraction(t *testing.T) {
	assert := assert.New(t)
	cmd := createK8sTestCmd()

	executor := newExecutor(cmd, true)
	assert.NotNil(executor)

	go printLog(executor.Stdout())

	// init executor
	err := executor.Init()
	assert.NoError(err)

	k8sExecutor := executor.(*K8sExecutor)
	assert.NotNil(k8sExecutor)

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

	// start pod
	go func() {
		err = executor.Start()
		assert.NoError(err)
	}()

	for {
		if k8sExecutor.pod == nil || k8sExecutor.pod.Status.Phase != v1.PodRunning{
			time.Sleep(1 * time.Second)
			continue
		}
		break
	}

	go func() {
		time.Sleep(2 * time.Second)
		executor.TtyIn() <- "echo helloworld > /tmp/1.log\n"
		time.Sleep(2 * time.Second)
		executor.TtyIn() <- "sleep 9999\n"
		time.Sleep(2 * time.Second)
		executor.StopTty()
	}()

	err = executor.StartTty("fakeId", func(ttyId string) {
		util.LogDebug("Tty started")
	})

	assert.NoError(err)
	assert.False(executor.IsInteracting())
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
			"sleep 10",
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
