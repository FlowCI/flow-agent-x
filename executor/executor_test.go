package executor

import (
	"context"
	"encoding/base64"
	"github.com/flowci/flow-agent-x/config"
	"github.com/flowci/flow-agent-x/domain"
	"github.com/flowci/flow-agent-x/util"
	"github.com/stretchr/testify/assert"
	"path"
	"runtime"
	"time"
)

func printLog(stdout <-chan string) {
	for {
		item, ok := <-stdout
		if !ok {
			break
		}

		bytes, _ := base64.StdEncoding.DecodeString(item)
		util.LogDebug("[LOG]: %s", string(bytes))
	}
}
func getTestDataDir() string {
	_, filename, _, _ := runtime.Caller(0)
	base := path.Dir(filename)
	return path.Join(base, "_testdata")
}

func newExecutor(cmd *domain.ShellIn, k8s bool) Executor {
	ctx, _ := context.WithCancel(context.Background())
	app := config.GetInstance()

	options := Options{
		K8s: &domain.K8sConfig{
			Enabled:   k8s,
			InCluster: false,
		},
		Parent:    ctx,
		Workspace: app.Workspace,
		PluginDir: app.PluginDir,
		Cmd:       cmd,
		Volumes: []*domain.DockerVolume{
			{
				Name:   "pyenv",
				Script: "init.sh",
				Dest:   "/ws/.pyenv",
				Image:  "flowci/pyenv:1.3",
				Init:   "init-pyenv-volume.sh",
			},
		},
	}

	return NewExecutor(options)
}

func shouldExecCmd(assert *assert.Assertions, cmd *domain.ShellIn) *domain.ShellOut {
	// when:
	executor := newExecutor(cmd, false)
	assert.NoError(executor.Init())

	go printLog(executor.Stdout())

	err := executor.Start()
	assert.NoError(err)

	// then:
	result := executor.GetResult()
	assert.Equal(0, result.Code)
	assert.True(result.LogSize > 0)
	assert.NotNil(result.FinishAt)
	assert.Equal("flowci", result.Output["FLOW_VVV"])
	assert.Equal("flow...", result.Output["FLOW_AAA"])

	return executor.GetResult()
}

func shouldExecWithError(assert *assert.Assertions, cmd *domain.ShellIn) {
	// init:
	cmd.AllowFailure = false
	cmd.Bash = []string{
		"notCommand should exit with error",
		"echo should_not_printed",
	}
	cmd.Pwsh = []string{
		"notCommand should exit with error",
		"echo should_not_printed",
	}

	// when:
	executor := newExecutor(cmd, false)
	assert.NoError(executor.Init())

	go printLog(executor.Stdout())

	err := executor.Start()
	assert.NoError(err)

	// then:
	result := executor.GetResult()
	assert.True(result.LogSize > 0)
	assert.True(result.Code != 0)
	assert.Equal(domain.CmdStatusException, result.Status)
	assert.NotNil(result.FinishAt)
}

func shouldExecWithErrorButAllowFailure(assert *assert.Assertions, cmd *domain.ShellIn) {
	// init:
	cmd.AllowFailure = true
	cmd.Bash = []string{"notCommand should exit with error"}
	cmd.Pwsh = []string{"notCommand should exit with error"}

	// when:
	executor := newExecutor(cmd, false)
	assert.NoError(executor.Init())

	go printLog(executor.Stdout())

	err := executor.Start()
	assert.NoError(err)

	// then:
	result := executor.GetResult()
	assert.True(result.LogSize > 0)
	assert.True(result.Code != 0)
	assert.Equal(domain.CmdStatusSuccess, result.Status)
	assert.NotNil(result.FinishAt)
}

func shouldExecButTimeOut(assert *assert.Assertions, cmd *domain.ShellIn) {
	// init:
	cmd.Timeout = 5
	cmd.Bash = []string{"echo $HOME", "sleep 9999", "echo ...."}
	cmd.Pwsh = []string{"echo ${HOME}", "sleep 9999", "echo ...."}

	// when:
	executor := newExecutor(cmd, false)
	assert.NoError(executor.Init())

	go printLog(executor.Stdout())

	err := executor.Start()
	assert.NoError(err)

	// then:
	result := executor.GetResult()
	assert.True(result.LogSize > 0)
	assert.Equal(domain.CmdStatusTimeout, result.Status)
	assert.Equal(domain.CmdExitCodeTimeOut, result.Code)
	assert.NotNil(result.FinishAt)
}

func shouldExecButKilled(assert *assert.Assertions, cmd *domain.ShellIn) {
	// init:
	cmd.Bash = []string{"echo $HOME", "sleep 9999", "echo ...."}
	cmd.Pwsh = []string{"echo ${HOME}", "sleep 9999", "echo ...."}

	// when:
	executor := newExecutor(cmd, false)
	assert.NoError(executor.Init())

	go printLog(executor.Stdout())

	time.AfterFunc(5*time.Second, func() {
		executor.Kill()
	})

	err := executor.Start()
	assert.NoError(err)

	// then:
	result := executor.GetResult()
	assert.True(result.LogSize > 0)
	assert.Equal(domain.CmdStatusKilled, result.Status)
	assert.Equal(domain.CmdExitCodeKilled, result.Code)
	assert.NotNil(result.FinishAt)
}
