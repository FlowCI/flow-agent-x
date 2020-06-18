package executor

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github/flowci/flow-agent-x/config"
	"github/flowci/flow-agent-x/domain"
	"github/flowci/flow-agent-x/util"
	"path"
	"runtime"
	"time"
)

func printLog(channel <-chan []byte) {
	for {
		item, ok := <-channel
		if !ok {
			break
		}
		util.LogDebug("[LOG]: %s", item)
	}
}
func getTestDataDir() string {
	_, filename, _, _ := runtime.Caller(0)
	base := path.Dir(filename)
	return path.Join(base, "_testdata")
}

func newExecutor(cmd *domain.ShellIn) Executor {
	ctx, _ := context.WithCancel(context.Background())
	app := config.GetInstance()

	options := Options{
		Parent:    ctx,
		Workspace: app.Workspace,
		PluginDir: app.PluginDir,
		Cmd:       cmd,
		Volumes: []*domain.DockerVolume{
			{
				Name:   "pyenv",
				Script: "init.sh",
				Dest:   "/ws/.pyenv",
			},
		},
	}

	return NewExecutor(options)
}

func shouldExecCmd(assert *assert.Assertions, cmd *domain.ShellIn) *domain.ShellOut {
	// when:
	executor := newExecutor(cmd)
	assert.NoError(executor.Init())

	go printLog(executor.LogChannel())

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
	cmd.Scripts = []string{"notCommand should exit with error"}

	// when:
	executor := newExecutor(cmd)
	assert.NoError(executor.Init())

	go printLog(executor.LogChannel())

	err := executor.Start()
	assert.NoError(err)

	// then:
	result := executor.GetResult()
	assert.True(result.LogSize > 0)
	assert.Equal(127, result.Code)
	assert.Equal(domain.CmdStatusException, result.Status)
	assert.NotNil(result.FinishAt)
}

func shouldExecWithErrorButAllowFailure(assert *assert.Assertions, cmd *domain.ShellIn) {
	// init:
	cmd.AllowFailure = true
	cmd.Scripts = []string{"notCommand should exit with error"}

	// when:
	executor := newExecutor(cmd)
	assert.NoError(executor.Init())

	go printLog(executor.LogChannel())

	err := executor.Start()
	assert.NoError(err)

	// then:
	result := executor.GetResult()
	assert.True(result.LogSize > 0)
	assert.Equal(127, result.Code)
	assert.Equal(domain.CmdStatusSuccess, result.Status)
	assert.NotNil(result.FinishAt)
}

func shouldExecButTimeOut(assert *assert.Assertions, cmd *domain.ShellIn) {
	// init:
	cmd.Timeout = 5
	cmd.Scripts = []string{"echo $HOME", "sleep 9999", "echo ...."}

	// when:
	executor := newExecutor(cmd)
	assert.NoError(executor.Init())

	go printLog(executor.LogChannel())

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
	cmd.Scripts = []string{"echo $HOME", "sleep 9999", "echo ...."}

	// when:
	executor := newExecutor(cmd)
	assert.NoError(executor.Init())

	go printLog(executor.LogChannel())

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
