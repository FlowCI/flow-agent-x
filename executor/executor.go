package executor

import (
	"context"
	"encoding/base64"
	"fmt"
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

	defaultChannelBufferSize  = 1000
	defaultLogWaitingDuration = 5 * time.Second
	defaultReaderBufferSize   = 8 * 1024 // 8k
)

type TypeOfExecutor int

type Executor interface {
	Init() error

	CmdId() string

	TtyId() string

	LogChannel() <-chan []byte

	TtyIn() chan<- string

	TtyOut() <-chan string

	Start() error

	StartTty(ttyId string, onStarted func(ttyId string)) error

	StopTty()

	IsInteracting() bool

	Kill()

	GetResult() *domain.ShellOut
}

type BaseExecutor struct {
	agentId     string // should be agent token
	workspace   string
	pluginDir   string
	context     context.Context
	cancelFunc  context.CancelFunc
	inCmd       *domain.ShellIn
	result      *domain.ShellOut
	vars        domain.Variables // vars from input and in cmd
	bashChannel chan string      // bash script comes from
	logChannel  chan []byte      // output log
	stdOutWg    sync.WaitGroup   // init on subclasses

	ttyId  string
	ttyIn  chan string // base script
	ttyOut chan string // b64 log content
}

type Options struct {
	AgentId   string
	Parent    context.Context
	Workspace string
	PluginDir string
	Cmd       *domain.ShellIn
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
		logChannel:  make(chan []byte, defaultChannelBufferSize),
		inCmd:       cmd,
		vars:        vars,
		result:      domain.NewShellOutput(cmd),
		ttyIn:       make(chan string, defaultChannelBufferSize),
		ttyOut:      make(chan string, defaultChannelBufferSize),
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

func (b *BaseExecutor) TtyId() string {
	return b.ttyId
}

func (b *BaseExecutor) FlowId() string {
	return b.inCmd.FlowId
}

// LogChannel for output log from stdout, stdin
func (b *BaseExecutor) LogChannel() <-chan []byte {
	return b.logChannel
}

func (b *BaseExecutor) TtyIn() chan<- string {
	return b.ttyIn
}

func (b *BaseExecutor) TtyOut() <-chan string {
	return b.ttyOut
}

func (b *BaseExecutor) GetResult() *domain.ShellOut {
	return b.result
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

	close(b.ttyIn)
	close(b.ttyOut)
}

func (b *BaseExecutor) writeLog(src io.Reader, doneOnWaitGroup bool) {
	go func() {
		defer func() {
			if err := recover(); err != nil {
				util.LogWarn(err.(error).Error())
			}

			if doneOnWaitGroup {
				b.stdOutWg.Done()
			}

			util.LogDebug("[Exit]: StdOut/Err, log size = %d", b.result.LogSize)
		}()

		buf := make([]byte, defaultReaderBufferSize)
		for {
			select {
			case <-b.context.Done():
				return
			default:
				n, err := src.Read(buf)
				if err != nil {
					return
				}

				b.logChannel <- removeDockerHeader(buf[0:n])
				atomic.AddInt64(&b.result.LogSize, int64(n))
			}
		}
	}()
}

func (b *BaseExecutor) writeSingleLog(msg string) {
	b.logChannel <- []byte(msg)
}

func (b *BaseExecutor) writeTtyIn(writer io.Writer) {
	for {
		inputStr, ok := <-b.ttyIn
		if !ok {
			return
		}

		in := []byte(inputStr)
		if strings.LastIndexByte(inputStr, util.UnixLineBreak) == -1 {
			in = append(in, util.UnixLineBreak)
		}
		_, _ = writer.Write(in)
	}
}

func (b *BaseExecutor) writeTtyOut(reader io.Reader) {
	buf := make([]byte, defaultReaderBufferSize)
	for {
		select {
		case <-b.context.Done():
			return
		default:
			n, err := reader.Read(buf)
			if err != nil {
				return
			}
			b.ttyOut <- base64.StdEncoding.EncodeToString(removeDockerHeader(buf[0:n]))
		}
	}
}

func (b *BaseExecutor) toStartStatus(pid int) {
	b.result.Status = domain.CmdStatusRunning
	b.result.ProcessId = pid
	b.result.StartAt = time.Now()
}

func (b *BaseExecutor) toErrorStatus(err error) error {
	b.result.Status = domain.CmdStatusException
	b.result.Error = err.Error()
	b.result.FinishAt = time.Now()
	return err
}

func (b *BaseExecutor) toTimeOutStatus() {
	b.result.Status = domain.CmdStatusTimeout
	b.result.Code = domain.CmdExitCodeTimeOut
	b.result.FinishAt = time.Now()
}

func (b *BaseExecutor) toKilledStatus() {
	b.result.Status = domain.CmdStatusKilled
	b.result.Code = domain.CmdExitCodeKilled
	b.result.FinishAt = time.Now()
}

func (b *BaseExecutor) toFinishStatus(exitCode int) {
	b.result.FinishAt = time.Now()
	b.result.Code = exitCode

	if exitCode == 0 {
		b.result.Status = domain.CmdStatusSuccess
		return
	}

	// no exported environment since it's failure
	if b.inCmd.AllowFailure {
		b.result.Status = domain.CmdStatusSuccess
		return
	}

	_ = b.toErrorStatus(fmt.Errorf("exit status %d", exitCode))
	return
}
