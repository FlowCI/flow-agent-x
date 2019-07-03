package executor

import (
	"flow-agent-x/config"
	"flow-agent-x/util"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"flow-agent-x/domain"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

var (
	cmd = &domain.CmdIn{
		Cmd: domain.Cmd{
			ID: "1-1-1",
		},
		Scripts: []string{
			"set -e",
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
)

func init() {
	log.SetFormatter(&log.JSONFormatter{})
	log.SetOutput(os.Stdout)
	log.SetLevel(log.DebugLevel)

	configInstance := config.GetInstance()
	configInstance.LoggingDir = util.ParseString(filepath.Join("${HOME}", ".flow.ci.agent", "logs"))
}

func TestShouldRunLinuxShell(t *testing.T) {
	assert := assert.New(t)

	// when: new shell executor and run
	executor := NewShellExecutor(cmd, config.GetInstance().LoggingDir)
	err := executor.Run()
	assert.Nil(err)

	// then: verify result of shell executor
	result := executor.Result
	assert.NotNil(result)
	assert.Equal(int64(2), result.LogSize)
	assert.Equal(0, result.Code)
	assert.False(result.StartAt.IsZero())
	assert.False(result.FinishAt.IsZero())
	assert.Equal("flowci", result.Output["FLOW_VVV"])
	assert.Equal("flow...", result.Output["FLOW_AAA"])

	// then: verify first log output
	firstLog := <-executor.GetLogChannel()
	assert.Equal(cmd.ID, firstLog.CmdID)
	assert.Equal("bbb", firstLog.Content)

	// then: verify second of log output
	secondLog := <-executor.GetLogChannel()
	assert.Equal(cmd.ID, secondLog.CmdID)
	assert.Equal("aaa", secondLog.Content)
}

func TestShouldRunLinuxShellButTimeOut(t *testing.T) {
	assert := assert.New(t)

	// init: cmd with timeout
	cmd.Timeout = 1

	// when: new shell executor and run
	executor := NewShellExecutor(cmd, config.GetInstance().LoggingDir)
	err := executor.Run()
	assert.Nil(err)

	result := executor.Result
	assert.NotNil(result)

	// then:
	assert.False(result.StartAt.IsZero())
	assert.False(result.FinishAt.IsZero())
	assert.True(result.ProcessId > 0)
	assert.Equal(domain.CmdExitCodeTimeOut, result.Code)
	assert.Equal(domain.CmdStatusTimeout, executor.Result.Status)
}

func TestShouldRunLinuxShellButKilled(t *testing.T) {
	assert := assert.New(t)

	// init:
	cmd.Scripts = []string{"set -e", "sleep 9999"}
	cmd.Timeout = 18000

	// when: new shell executor and run
	executor := NewShellExecutor(cmd, config.GetInstance().LoggingDir)

	go func() {
		time.Sleep(5 * time.Second)
		_ = executor.Kill()
	}()

	err := executor.Run()
	assert.Nil(err)

	// then:
	assert.Equal(-1, executor.Result.Code)
	assert.Equal(domain.CmdStatusKilled, executor.Result.Status)
}

func TestShouldCmdNotFoundErr(t *testing.T) {
	assert := assert.New(t)

	// init:
	cmd.Scripts = []string{"set -e", "notCommand"}

	// when:
	executor := NewShellExecutor(cmd, config.GetInstance().LoggingDir)
	err := executor.Run()
	assert.Nil(err)

	// then:
	assert.Equal(127, executor.Result.Code)
	assert.Equal(domain.CmdStatusException, executor.Result.Status)
}

func TestShouldWorkOnInteractMode(t *testing.T) {
	assert := assert.New(t)

	// init:
	cmd.Scripts = nil
	executor := NewShellExecutor(cmd, config.GetInstance().LoggingDir)
	executor.EnableInteractMode = true
	cmdChannel := executor.GetCmdChannel()
	logChannel := executor.GetLogChannel()

	go printLog(logChannel)

	go func() {
		for i := 0; i < 5; i++ {
			script := fmt.Sprintf("echo i = %d", i)
			cmdChannel <- script
			cmdChannel <- `echo "\033[0;31m $? \033[0m"`
			time.Sleep(1 * time.Second)
		}

		cmdChannel <- ExitCmd
	}()

	err := executor.Run()
	assert.Nil(err)
}

func TestShouldGetRawLogWithSuccessStatus(t *testing.T) {
	assert := assert.New(t)

	cmd.Scripts = []string{"echo hello"}

	executor := NewShellExecutor(cmd, config.GetInstance().LoggingDir)
	executor.EnableRawLog = true

	go printLog(executor.GetLogChannel())
	go printRaw(executor.GetRawChannel())

	err := executor.Run()
	assert.Nil(err)

	assert.Equal(domain.CmdStatusSuccess, executor.Result.Status)
	assert.True(executor.Result.Code == 0)
}

func TestShouldGetRawLogWithExceptionStatus(t *testing.T) {
	assert := assert.New(t)

	cmd.Scripts = []string{"rm aa"}

	executor := NewShellExecutor(cmd, config.GetInstance().LoggingDir)
	executor.EnableRawLog = true

	go printLog(executor.GetLogChannel())
	go printRaw(executor.GetRawChannel())

	err := executor.Run()
	assert.Nil(err)

	assert.Equal(domain.CmdStatusException, executor.Result.Status)
	assert.True(executor.Result.Code > 0)
}

func printLog(channel <-chan *domain.LogItem) {
	for {
		item, ok := <-channel
		if !ok {
			break
		}
		log.Debug("[LOG]: ", item.Content)
	}
}

func printRaw(channel <-chan string) {
	for {
		item, ok := <-channel
		if !ok {
			break
		}
		log.Debug("[RAW]: ", item)
	}
}
