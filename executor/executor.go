package executor

import (
	"context"
	"fmt"
	"github/flowci/flow-agent-x/domain"
	"github/flowci/flow-agent-x/util"
	"io"
	"sync"
	"sync/atomic"
	"time"
)

const (
	linuxBash = "/bin/bash"
	//linuxBashShebang = "#!/bin/bash -i" // add -i enable to source .bashrc

	defaultLogChannelBufferSize = 10000
	defaultLogWaitingDuration   = 5 * time.Second
	defaultReaderBufferSize     = 8 * 1024 // 8k
)

type TypeOfExecutor int

type Executor interface {
	Init() error

	CmdId() string

	BashChannel() chan<- string

	LogChannel() <-chan *domain.LogItem

	Start() error

	Kill()

	GetResult() *domain.ExecutedCmd
}

type BaseExecutor struct {
	agentId     string // should be agent token
	workspace   string
	pluginDir   string
	context     context.Context
	cancelFunc  context.CancelFunc
	inCmd       *domain.CmdIn
	vars        domain.Variables     // vars from input and in cmd
	bashChannel chan string          // bash script comes from
	logChannel  chan *domain.LogItem // output log
	CmdResult   *domain.ExecutedCmd
	stdOutWg    sync.WaitGroup // init on subclasses
}

type Options struct {
	AgentId   string
	Parent    context.Context
	Workspace string
	PluginDir string
	Cmd       *domain.CmdIn
	Vars      domain.Variables
	Volumes   []*domain.DockerVolume
}

func NewExecutor(options Options) Executor {
	if options.Vars == nil {
		options.Vars = domain.NewVariables()
	}

	cmd := options.Cmd

	vars := domain.ConnectVars(options.Vars, cmd.Inputs)
	vars.Resolve()

	base := BaseExecutor{
		agentId:     options.AgentId,
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

	if cmd.HasDockerOption() {
		return &DockerExecutor{
			BaseExecutor: base,
			volumes:      options.Volumes,
		}
	}

	return &BashExecutor{
		BaseExecutor: base,
	}
}

// CmdID current bash executor cmd id
func (b *BaseExecutor) CmdId() string {
	return b.inCmd.ID
}

func (b *BaseExecutor) JobId() string {
	return b.inCmd.JobId
}

func (b *BaseExecutor) FlowId() string {
	return b.inCmd.FlowId
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

func (b *BaseExecutor) writeCmd(stdin io.Writer, before, after func(chan string)) {
	consumer := func() {
		for {
			select {
			case <-b.context.Done():
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

	b.bashChannel <- "set -e"

	if before != nil {
		before(b.bashChannel)
	}

	for _, script := range b.inCmd.Scripts {
		b.bashChannel <- script
	}

	if after != nil {
		after(b.bashChannel)
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

func (b *BaseExecutor) writeLog(reader io.Reader, doneOnWaitGroup bool) {
	go func() {
		defer func() {
			if err := recover(); err != nil {
				util.LogWarn(err.(error).Error())
			}

			if doneOnWaitGroup {
				b.stdOutWg.Done()
			}

			util.LogDebug("[Exit]: StdOut/Err, log size = %d", b.CmdResult.LogSize)
		}()

		buffer := make([]byte, defaultReaderBufferSize)

		for {
			select {
			case <-b.context.Done():
				return
			default:
				n, err := reader.Read(buffer)
				if err != nil {
					return
				}

				b.logChannel <- &domain.LogItem{
					CmdId:   b.CmdId(),
					Content: removeDockerHeader(buffer[0:n]),
				}

				atomic.AddInt64(&b.CmdResult.LogSize, int64(n))
			}
		}
	}()
}

func (b *BaseExecutor) writeSingleLog(msg string) {
	b.logChannel <- &domain.LogItem{
		CmdId:   b.CmdId(),
		Content: []byte(msg),
	}
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

	if exitCode == 0 {
		b.CmdResult.Status = domain.CmdStatusSuccess
		return
	}

	// no exported environment since it's failure
	if b.inCmd.AllowFailure {
		b.CmdResult.Status = domain.CmdStatusSuccess
		return
	}

	_ = b.toErrorStatus(fmt.Errorf("exit status %d", exitCode))
	return

}
