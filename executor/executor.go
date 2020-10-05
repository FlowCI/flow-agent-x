package executor

import (
	"context"
	"encoding/base64"
	"fmt"
	"github/flowci/flow-agent-x/domain"
	"github/flowci/flow-agent-x/util"
	"io"
	"sync"
	"sync/atomic"
	"time"
)

const (
	winPowerShell = "powershell.exe"
	linuxBash = "/bin/bash"
	//linuxBashShebang = "#!/bin/bash -i" // add -i enable to source .bashrc

	defaultChannelBufferSize  = 1000
	defaultLogWaitingDuration = 5 * time.Second
	defaultReaderBufferSize   = 8 * 1024 // 8k
)

type Executor interface {
	Init() error

	CmdIn() *domain.ShellIn

	StartTty(ttyId string, onStarted func(ttyId string)) error

	StopTty()

	TtyId() string

	TtyIn() chan<- string

	TtyOut() <-chan string // b64 out

	IsInteracting() bool

	Start() error

	Stdout() <-chan string // b64 log

	Kill()

	GetResult() *domain.ShellOut
}

type BaseExecutor struct {
	k8sConfig *domain.K8sConfig

	agentId    string // should be agent token
	workspace  string
	pluginDir  string
	volumes    []*domain.DockerVolume
	context    context.Context
	cancelFunc context.CancelFunc
	inCmd      *domain.ShellIn
	result     *domain.ShellOut
	vars       domain.Variables // vars from input and in cmd
	stdin      chan string      // bash script comes from
	stdout     chan string      // output log
	stdOutWg   sync.WaitGroup   // init on subclasses

	ttyId     string
	ttyIn     chan string // b64 script
	ttyOut    chan string // b64 log content
	ttyCtx    context.Context
	ttyCancel context.CancelFunc
}

type Options struct {
	K8s *domain.K8sConfig

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
	base := BaseExecutor{
		k8sConfig: options.K8s,
		agentId:   options.AgentId,
		workspace: options.Workspace,
		pluginDir: options.PluginDir,
		volumes:   options.Volumes,
		stdin:     make(chan string),
		stdout:    make(chan string, defaultChannelBufferSize),
		inCmd:     cmd,
		vars:      domain.ConnectVars(options.Vars, cmd.Inputs),
		result:    domain.NewShellOutput(cmd),
		ttyIn:     make(chan string, defaultChannelBufferSize),
		ttyOut:    make(chan string, defaultChannelBufferSize),
	}

	ctx, cancel := context.WithTimeout(options.Parent, time.Duration(cmd.Timeout)*time.Second)
	base.context = ctx
	base.cancelFunc = cancel

	if cmd.HasDockerOption() {
		return &DockerExecutor{
			BaseExecutor: base,
		}
	}

	return &shellExecutor{
		BaseExecutor: base,
	}
}

// CmdID current bash executor cmd id
func (b *BaseExecutor) CmdIn() *domain.ShellIn {
	return b.inCmd
}

func (b *BaseExecutor) TtyId() string {
	return b.ttyId
}

// LogChannel for output log from stdout, stdin
func (b *BaseExecutor) Stdout() <-chan string {
	return b.stdout
}

func (b *BaseExecutor) TtyIn() chan<- string {
	return b.ttyIn
}

func (b *BaseExecutor) TtyOut() <-chan string {
	return b.ttyOut
}

func (b *BaseExecutor) IsInteracting() bool {
	return b.ttyCtx != nil && b.ttyCancel != nil
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

func (b *BaseExecutor) isK8sEnabled() bool {
	return b.k8sConfig != nil && b.k8sConfig.Enabled
}

func (b *BaseExecutor) writeCmd(stdin io.Writer, before, after func(chan string), doScript func(string) string) {
	consumer := func() {
		for {
			select {
			case <-b.context.Done():
				return
			case script, ok := <-b.stdin:
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

	// source volume script
	b.stdin <- "set +e" // ignore source file failure
	for _, v := range b.volumes {
		if util.IsEmptyString(v.Script) {
			continue
		}
		b.stdin <- fmt.Sprintf("source %s > /dev/null 2>&1", v.ScriptPath())
	}

	if before != nil {
		before(b.stdin)
	}

	// write shell script from cmd
	b.stdin <- "set -e"
	for _, script := range b.inCmd.Scripts {
		b.stdin <- doScript(script)
	}

	if after != nil {
		after(b.stdin)
	}

	b.stdin <- "exit"
}

func (b *BaseExecutor) closeChannels() {
	if len(b.stdout) > 0 {
		util.Wait(&b.stdOutWg, defaultLogWaitingDuration)
	}

	close(b.stdin)
	close(b.stdout)

	close(b.ttyIn)
	close(b.ttyOut)
}

func (b *BaseExecutor) writeLog(src io.Reader, inThread, doneOnWaitGroup bool) {
	write := func() {
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

				b.stdout <- base64.StdEncoding.EncodeToString(removeDockerHeader(buf[0:n]))
				atomic.AddInt64(&b.result.LogSize, int64(n))
			}
		}
	}

	if inThread {
		go write()
		return
	}

	write()
}

func (b *BaseExecutor) writeSingleLog(msg string) {
	b.stdout <- base64.StdEncoding.EncodeToString([]byte(msg + "\n"))
}

func (b *BaseExecutor) writeTtyIn(writer io.Writer) {
	for inputStr := range b.ttyIn {
		in := []byte(inputStr)
		_, err := writer.Write(in)

		if err != nil {
			util.LogIfError(err)
			return
		}
	}
}

func (b *BaseExecutor) writeTtyOut(reader io.Reader) {
	buf := make([]byte, defaultReaderBufferSize)
	for {
		select {
		case <-b.ttyCtx.Done():
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
