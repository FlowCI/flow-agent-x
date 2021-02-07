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
	linuxBash     = "/bin/bash"
	//linuxBashShebang = "#!/bin/bash -i" // add -i enable to source .bashrc

	defaultChannelBufferSize  = 1000
	defaultLogWaitingDuration = 5 * time.Second
	defaultReaderBufferSize   = 8 * 1024 // 8k
)

type Executor interface {
	Init() error

	CacheDir() (string, string)

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

	Close()

	GetResult() *domain.ShellOut
}

type BaseExecutor struct {
	k8sConfig *domain.K8sConfig

	agentId   string // should be agent token
	workspace string // agent workspace
	pluginDir string
	jobDir    string // job workspace

	cacheInputDir  string // downloaded cache temp dir
	cacheOutputDir string // temp dir that need to upload

	os         string // current operation system
	context    context.Context
	cancelFunc context.CancelFunc

	volumes []*domain.DockerVolume
	inCmd   *domain.ShellIn
	result  *domain.ShellOut
	vars    domain.Variables // vars from input and in cmd

	stdout   chan string    // output log
	stdOutWg sync.WaitGroup // init on subclasses

	ttyId     string
	ttyIn     chan string // b64 script
	ttyOut    chan string // b64 log content
	ttyCtx    context.Context
	ttyCancel context.CancelFunc
}

type Options struct {
	K8s *domain.K8sConfig

	AgentId     string
	Parent      context.Context
	Workspace   string
	PluginDir   string
	CacheSrcDir string
	Cmd         *domain.ShellIn
	Vars        domain.Variables
	Volumes     []*domain.DockerVolume
}

func NewExecutor(options Options) Executor {
	if options.Vars == nil {
		options.Vars = domain.NewVariables()
	}

	cmd := options.Cmd
	base := BaseExecutor{
		k8sConfig:     options.K8s,
		agentId:       options.AgentId,
		workspace:     options.Workspace,
		pluginDir:     options.PluginDir,
		cacheInputDir: options.CacheSrcDir,
		volumes:       options.Volumes,
		stdout:        make(chan string, defaultChannelBufferSize),
		inCmd:         cmd,
		vars:          domain.ConnectVars(options.Vars, cmd.Inputs),
		result:        domain.NewShellOutput(cmd),
		ttyIn:         make(chan string, defaultChannelBufferSize),
		ttyOut:        make(chan string, defaultChannelBufferSize),
	}

	ctx, cancel := context.WithTimeout(options.Parent, time.Duration(cmd.Timeout)*time.Second)
	base.context = ctx
	base.cancelFunc = cancel

	if cmd.HasDockerOption() {
		return &dockerExecutor{
			BaseExecutor: base,
		}
	}

	return &shellExecutor{
		BaseExecutor: base,
	}
}

func (b *BaseExecutor) CacheDir() (input string, output string) {
	input = b.cacheInputDir
	output = b.cacheOutputDir
	return
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

func (b *BaseExecutor) Close() {
	if len(b.stdout) > 0 {
		util.Wait(&b.stdOutWg, defaultLogWaitingDuration)
	}

	close(b.stdout)
	close(b.ttyIn)
	close(b.ttyOut)

	b.cancelFunc()
}

//====================================================================
//	private
//====================================================================

func (b *BaseExecutor) isK8sEnabled() bool {
	return b.k8sConfig != nil && b.k8sConfig.Enabled
}

func (b *BaseExecutor) writeCmd(stdin io.Writer, before, after func() []string, doScript func(string) string) {
	write := func(script string) {
		_, _ = io.WriteString(stdin, appendNewLine(script, b.os))
		util.LogDebug("----- exec: %s", script)
	}

	// source volume script
	if b.os != util.OSWin {
		write("set +e") // ignore source file failure
		for _, v := range b.volumes {
			if util.IsEmptyString(v.Script) {
				continue
			}
			write(fmt.Sprintf("source %s > /dev/null 2>&1", v.ScriptPath()))
		}
	}

	if before != nil {
		for _, script := range before() {
			write(script)
		}
	}

	// write shell script from cmd
	for _, script := range scriptForExitOnError(b.os) {
		write(script)
	}

	for _, script := range b.getScripts() {
		write(doScript(script))
	}

	if after != nil {
		for _, script := range after() {
			write(script)
		}
	}

	write("exit")
}

func (b *BaseExecutor) getScripts() []string {
	scripts := b.inCmd.Bash
	if b.os == util.OSWin {
		scripts = b.inCmd.Pwsh
	}

	isAllEmpty := true
	for _, script := range scripts {
		if !util.IsEmptyString(script) {
			isAllEmpty = false
		}
	}

	if isAllEmpty {
		panic(fmt.Errorf("agent: Missing bash or pwsh section in flow YAML"))
	}

	return scripts
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
		if inputStr == "\r" {
			inputStr = newLineForOs(b.os)
		}

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
		n, err := reader.Read(buf)
		if err != nil {
			return
		}
		b.ttyOut <- base64.StdEncoding.EncodeToString(removeDockerHeader(buf[0:n]))
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
