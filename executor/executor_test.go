package executor

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github/flowci/flow-agent-x/config"
	"github/flowci/flow-agent-x/domain"
	"github/flowci/flow-agent-x/util"
	"path"
	"path/filepath"
	"runtime"
	"time"
)

func printLog(channel <-chan *domain.LogItem) {
	for {
		item, ok := <-channel
		if !ok {
			break
		}
		util.LogDebug("[LOG]: %s", item.Content)
	}
}
func getTestDataDir() string {
	_, filename, _, _ := runtime.Caller(0)
	base := path.Dir(filename)
	return path.Join(base, "_testdata")
}

func newExecutor(cmd *domain.CmdIn) Executor {
	ctx, _ := context.WithCancel(context.Background())

	app := config.GetInstance()
	workDir := filepath.Join(app.Workspace, util.ParseString(cmd.FlowId))

	options := Options{
		Parent:    ctx,
		WorkDir:   workDir,
		PluginDir: app.PluginDir,
		Cmd:       cmd,
	}

	return NewExecutor(options)
}

func shouldExecCmd(assert *assert.Assertions, cmd *domain.CmdIn) {
	// when:
	executor := newExecutor(cmd)
	go printLog(executor.LogChannel())

	err := executor.Start()
	assert.NoError(err)

	// then:
	result := executor.GetResult()
	assert.Equal(0, result.Code)
	assert.Equal(int64(2), result.LogSize)
	assert.NotNil(result.FinishAt)
	assert.Equal("flowci", result.Output["FLOW_VVV"])
	assert.Equal("flow...", result.Output["FLOW_AAA"])
}

func shouldExecWithError(assert *assert.Assertions, cmd *domain.CmdIn) {
	// init:
	cmd.AllowFailure = false
	cmd.Scripts = []string{"notCommand should exit with error"}

	// when:
	executor := newExecutor(cmd)
	go printLog(executor.LogChannel())

	err := executor.Start()
	assert.NoError(err)

	// then:
	assert.Equal(int64(1), executor.GetResult().LogSize)
	assert.Equal(127, executor.GetResult().Code)
	assert.Equal(domain.CmdStatusException, executor.GetResult().Status)
	assert.NotNil(executor.GetResult().FinishAt)
}

func shouldExecWithErrorButAllowFailure(assert *assert.Assertions, cmd *domain.CmdIn) {
	// init:
	cmd.AllowFailure = true
	cmd.Scripts = []string{"notCommand should exit with error"}

	// when:
	executor := newExecutor(cmd)
	go printLog(executor.LogChannel())

	err := executor.Start()
	assert.NoError(err)

	// then:
	assert.Equal(int64(1), executor.GetResult().LogSize)
	assert.Equal(0, executor.GetResult().Code)
	assert.Equal(domain.CmdStatusSuccess, executor.GetResult().Status)
	assert.NotNil(executor.GetResult().FinishAt)
}

func shouldExecButTimeOut(assert *assert.Assertions, cmd *domain.CmdIn) {
	// init:
	cmd.Timeout = 5
	cmd.Scripts = []string{"echo $HOME", "sleep 9999", "echo ...."}

	// when:
	executor := newExecutor(cmd)
	go printLog(executor.LogChannel())

	err := executor.Start()
	assert.NoError(err)

	// then:
	assert.Equal(domain.CmdStatusTimeout, executor.GetResult().Status)
	assert.Equal(domain.CmdExitCodeTimeOut, executor.GetResult().Code)
	assert.Equal(int64(1), executor.GetResult().LogSize)
	assert.NotNil(executor.GetResult().FinishAt)
}

func shouldExecButKilled(assert *assert.Assertions, cmd *domain.CmdIn) {
	// init:
	cmd.Scripts = []string{"echo $HOME", "sleep 9999", "echo ...."}

	// when:
	executor := newExecutor(cmd)
	go printLog(executor.LogChannel())

	time.AfterFunc(5*time.Second, func() {
		executor.Kill()
	})

	err := executor.Start()
	assert.NoError(err)

	// then:
	assert.Equal(domain.CmdStatusKilled, executor.GetResult().Status)
	assert.Equal(domain.CmdExitCodeKilled, executor.GetResult().Code)
	assert.Equal(int64(1), executor.GetResult().LogSize)
	assert.NotNil(executor.GetResult().FinishAt)
}


