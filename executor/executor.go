package executor

import (
	"bufio"
	"context"
	"fmt"
	"github.com/google/uuid"
	"github/flowci/flow-agent-x/domain"
	"github/flowci/flow-agent-x/util"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	linuxBash = "/bin/bash"
	//linuxBashShebang = "#!/bin/bash -i" // add -i enable to source .bashrc

	defaultLogChannelBufferSize = 10000
	defaultLogWaitingDuration   = 5 * time.Second
	defaultReaderBufferSize     = 8 * 1024
)

type TypeOfExecutor int

type Executor interface {
	Init() error

	CmdID() string

	BashChannel() chan<- string

	LogChannel() <-chan *domain.LogItem

	Start() error

	Kill()

	GetResult() *domain.ExecutedCmd
}

type BaseExecutor struct {
	workspace   string
	pluginDir   string
	context     context.Context
	cancelFunc  context.CancelFunc
	inCmd       *domain.CmdIn
	vars        domain.Variables     // vars from input and in cmd
	bashChannel chan string          // bash script comes from
	logChannel  chan *domain.LogItem // output log
	endTag      string
	CmdResult   *domain.ExecutedCmd
	stdOutWg    sync.WaitGroup // init on subclasses
}

type Options struct {
	Parent    context.Context
	Workspace string
	PluginDir string
	Cmd       *domain.CmdIn
	Vars      domain.Variables
}

func NewExecutor(options Options) Executor {
	if options.Vars == nil {
		options.Vars = domain.NewVariables()
	}

	cmd := options.Cmd

	vars := domain.ConnectVars(options.Vars, cmd.Inputs)
	vars.Resolve()

	base := BaseExecutor{
		workspace:   options.Workspace,
		pluginDir:   options.PluginDir,
		bashChannel: make(chan string),
		logChannel:  make(chan *domain.LogItem, defaultLogChannelBufferSize),
		inCmd:       cmd,
		vars:        vars,
		CmdResult:   domain.NewExecutedCmd(cmd),
	}

	ctx, cancel := context.WithTimeout(options.Parent, time.Duration(cmd.Timeout)*time.Second)
	base.context = ctx
	base.cancelFunc = cancel

	endUUID, _ := uuid.NewRandom()
	base.endTag = fmt.Sprintf("=====EOF-%s=====", endUUID)

	if cmd.HasDockerOption() {
		return &DockerExecutor{
			BaseExecutor: base,
		}
	}

	return &BashExecutor{
		BaseExecutor: base,
	}
}

// CmdID current bash executor cmd id
func (b *BaseExecutor) CmdID() string {
	return b.inCmd.ID
}

// BashChannel for input bash script
func (b *BaseExecutor) BashChannel() chan<- string {
	return b.bashChannel
}

// LogChannel for output log from stdout, stdin
func (b *BaseExecutor) LogChannel() <-chan *domain.LogItem {
	return b.logChannel
}

func (b *BaseExecutor) GetResult() *domain.ExecutedCmd {
	return b.CmdResult
}

// Stop stop current running script
func (b *BaseExecutor) Kill() {
	b.cancelFunc()
}

//====================================================================
//	private
//====================================================================

func (b *BaseExecutor) startConsumeStdIn(stdin io.Writer) context.CancelFunc {
	ctx, cancel := context.WithCancel(b.context)

	consumer := func() {
		for {
			select {
			case <-ctx.Done():
				return
			case script, ok := <-b.bashChannel:
				if !ok {
					return
				}
				_, _ = io.WriteString(stdin, appendNewLine(script))
				util.LogDebug("----- exec: %s", script)
			}
		}
	}

	// start
	go consumer()

	b.sendScriptFromCmdIn()
	return cancel
}

func (b *BaseExecutor) startConsumeStdOut(reader io.Reader, onExitFunc func()) context.CancelFunc {
	ctx, cancel := context.WithCancel(b.context)
	cmdResult := b.CmdResult

	consumer := func() {
		defer func() {
			// handle panic: send on closed channel
			if err := recover(); err != nil {
				util.LogWarn(err.(error).Error())
			}
			onExitFunc()
		}()

		bufferReader := bufio.NewReaderSize(reader, defaultReaderBufferSize)
		var builder strings.Builder

		for {
			select {
			case <-ctx.Done():
				return
			default:
				line, err := readLine(bufferReader, builder)
				builder.Reset()

				if err != nil {
					return
				}

				// to read system env vars in the end
				if strings.Contains(line, b.endTag) {
					for {
						envLine, err := readLine(bufferReader, builder)
						builder.Reset()

						if err != nil {
							return
						}

						index := strings.IndexAny(envLine, "=")
						if index == -1 {
							continue
						}

						key := envLine[0:index]
						value := envLine[index+1:]

						if matchEnvFilter(key, b.inCmd.EnvFilters) {
							cmdResult.Output[key] = value
						}
					}
				}

				// send log item instance to channel
				atomic.AddInt64(&cmdResult.LogSize, 1)
				b.logChannel <- &domain.LogItem{CmdID: cmdResult.ID, Content: line}
			}
		}
	}

	go consumer()
	return cancel
}

func (b *BaseExecutor) sendScriptFromCmdIn() {
	set := "set -e"
	if b.inCmd.AllowFailure {
		set = "set +e"
	}

	b.bashChannel <- set

	for _, script := range b.inCmd.Scripts {
		b.bashChannel <- script
	}

	// write for end term
	if len(b.inCmd.EnvFilters) > 0 {
		b.bashChannel <- "echo " + b.endTag
		b.bashChannel <- "env"
	}

	b.bashChannel <- "exit"
}

func (b *BaseExecutor) closeChannels() {
	if len(b.LogChannel()) > 0 {
		util.Wait(&b.stdOutWg, defaultLogWaitingDuration)
	}

	close(b.bashChannel)
	close(b.logChannel)
}

func (b *BaseExecutor) toStartStatus(pid int) {
	b.CmdResult.Status = domain.CmdStatusRunning
	b.CmdResult.ProcessId = pid
	b.CmdResult.StartAt = time.Now()
}

func (b *BaseExecutor) toErrorStatus(err error) error {
	b.CmdResult.Status = domain.CmdStatusException
	b.CmdResult.Error = err.Error()
	b.CmdResult.FinishAt = time.Now()
	return err
}

func (b *BaseExecutor) toTimeOutStatus() {
	b.CmdResult.Status = domain.CmdStatusTimeout
	b.CmdResult.Code = domain.CmdExitCodeTimeOut
	b.CmdResult.FinishAt = time.Now()
}

func (b *BaseExecutor) toKilledStatus() {
	b.CmdResult.Status = domain.CmdStatusKilled
	b.CmdResult.Code = domain.CmdExitCodeKilled
	b.CmdResult.FinishAt = time.Now()
}

func (b *BaseExecutor) toFinishStatus(exitCode int) {
	b.CmdResult.FinishAt = time.Now()
	b.CmdResult.Code = exitCode

	if exitCode != 0 {
		_ = b.toErrorStatus(fmt.Errorf("exit status %d", exitCode))
		return
	}

	b.CmdResult.Status = domain.CmdStatusSuccess
}
