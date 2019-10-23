package executor

import (
	"bufio"
	"context"
	"fmt"
	"github.com/google/uuid"
	"github/flowci/flow-agent-x/domain"
	"github/flowci/flow-agent-x/util"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

var (
	linuxBash        = "/bin/bash"
	linuxBashShebang = "#!/bin/bash -i" // add -i enable to source .bashrc

	defaultLogChannelBufferSize = 10000
	defaultLogWaitingDuration   = 5 * time.Second
)

type (
	BashExecutor struct {
		cmdId  string
		inCmd  *domain.CmdIn
		inVars domain.Variables

		context    context.Context
		cancelFunc context.CancelFunc

		bashChannel chan string          // bash script comes from
		logChannel  chan *domain.LogItem // output log
		stdOutWg    sync.WaitGroup
		endTag      string

		CmdResult *domain.ExecutedCmd
	}
)

// NewBashExecutor create new instance of bash executor
func NewBashExecutor(parent context.Context, inCmd *domain.CmdIn, vars domain.Variables) *BashExecutor {
	instance := &BashExecutor{
		cmdId:       inCmd.ID,
		bashChannel: make(chan string),
		logChannel:  make(chan *domain.LogItem, defaultLogChannelBufferSize),
		inCmd:       inCmd,
		inVars:      vars,
		CmdResult:   domain.NewExecutedCmd(inCmd),
	}

	if vars == nil {
		instance.inVars = make(domain.Variables, 0)
	}

	ctx, cancel := context.WithTimeout(parent, time.Duration(inCmd.Timeout)*time.Second)
	instance.context = ctx
	instance.cancelFunc = cancel

	endUUID, _ := uuid.NewRandom()
	instance.endTag = fmt.Sprintf("=====EOF-%s=====", endUUID)
	instance.stdOutWg.Add(2)

	return instance
}

//====================================================================
//	Public
//====================================================================

// CmdID current bash executor cmd id
func (b *BashExecutor) CmdID() string {
	return b.cmdId
}

// BashChannel for input bash script
func (b *BashExecutor) BashChannel() chan<- string {
	return b.bashChannel
}

// LogChannel for output log from stdout, stdin
func (b *BashExecutor) LogChannel() <-chan *domain.LogItem {
	return b.logChannel
}

// Start run the cmd from domain.CmdIn
func (b *BashExecutor) Start() error {
	defer func() {
		close(b.bashChannel)

		if len(b.LogChannel()) > 0 {
			util.Wait(&b.stdOutWg, defaultLogWaitingDuration)
		}

		close(b.logChannel)
	}()

	// init exec.command
	command := exec.Command(linuxBash)
	command.Dir = b.inCmd.WorkDir

	if err := createWorkDir(command.Dir); err != nil {
		return b.toErrorStatus(err)
	}

	stdin, _ := command.StdinPipe()
	stdout, _ := command.StdoutPipe()
	stderr, _ := command.StderrPipe()

	command.Env = append(os.Environ(), b.inCmd.VarsToStringArray()...)
	command.Env = append(command.Env, b.inVars.ToStringArray()...)

	go func() {
		select {
		case <-b.context.Done():
			err := b.context.Err()

			if err == context.DeadlineExceeded {
				util.LogDebug("Timeout..")
				_ = command.Process.Kill()
				b.toTimeOutStatus()
				return
			}

			if err == context.Canceled {
				util.LogDebug("Cancel..")
				_ = command.Process.Kill()
				b.toKilledStatus()
			}
		}
	}()

	// start command
	if err := command.Start(); err != nil {
		return b.toErrorStatus(err)
	}

	cancelForStdOut := b.startConsumeStdOut(stdout)
	cancelForStdErr := b.startConsumeStdOut(stderr)
	cancelForStdIn := b.startConsumeStdIn(stdin)

	defer func() {
		cancelForStdOut()
		cancelForStdErr()
		cancelForStdIn()
	}()

	b.toStartStatus(command.Process.Pid)

	// wait or timeout
	err := command.Wait()
	util.LogDebug("[Done]: Shell for %s", b.cmdId)

	if b.CmdResult.Status == domain.CmdStatusTimeout {
		return nil
	}

	if b.CmdResult.Status == domain.CmdStatusKilled {
		return nil
	}

	// to finish status
	b.toFinishStatus(err, getExitCode(command))
	return nil
}

// Stop stop current running script
func (b *BashExecutor) Kill() {
	b.cancelFunc()
}

//====================================================================
//	private
//====================================================================

func (b *BashExecutor) toStartStatus(pid int) {
	b.CmdResult.Status = domain.CmdStatusRunning
	b.CmdResult.ProcessId = pid
	b.CmdResult.StartAt = time.Now()
}

func (b *BashExecutor) toErrorStatus(err error) error {
	b.CmdResult.Status = domain.CmdStatusException
	b.CmdResult.Error = err.Error()
	b.CmdResult.FinishAt = time.Now()
	return err
}

func (b *BashExecutor) toTimeOutStatus() {
	b.CmdResult.Status = domain.CmdStatusTimeout
	b.CmdResult.Code = domain.CmdExitCodeTimeOut
	b.CmdResult.FinishAt = time.Now()
}

func (b *BashExecutor) toKilledStatus() {
	b.CmdResult.Status = domain.CmdStatusKilled
	b.CmdResult.Code = domain.CmdExitCodeKilled
	b.CmdResult.FinishAt = time.Now()
}

func (b *BashExecutor) toFinishStatus(errFromCmd error, exitCode int) {
	b.CmdResult.FinishAt = time.Now()
	b.CmdResult.Code = exitCode

	// success status if no err
	if errFromCmd == nil {
		b.CmdResult.Status = domain.CmdStatusSuccess
		return
	}

	exitError, _ := errFromCmd.(*exec.ExitError)
	_ = b.toErrorStatus(exitError)
}

func (b *BashExecutor) startConsumeStdIn(stdin io.WriteCloser) context.CancelFunc {
	ctx, cancel := context.WithCancel(b.context)

	consumer := func() {
		defer func() {
			_ = stdin.Close()
			util.LogDebug("[Exit]: StdIn")
		}()

		for {
			select {
			case <-ctx.Done():
				return
			case script, ok := <-b.bashChannel:
				if !ok {
					return
				}
				_, _ = io.WriteString(stdin, appendNewLine(script))
			}
		}
	}

	// start
	go consumer()

	b.sendScriptForAllowFailure()
	b.sendScriptFromCmdIn()

	return cancel
}

func (b *BashExecutor) sendScriptFromCmdIn() {
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

func (b *BashExecutor) sendScriptForAllowFailure() {
	set := "set -e"

	if b.inCmd.AllowFailure {
		set = "set +e"
	}

	b.bashChannel <- set
}

func (b *BashExecutor) startConsumeStdOut(reader io.ReadCloser) context.CancelFunc {
	ctx, cancel := context.WithCancel(b.context)
	cmdResult := b.CmdResult

	consumer := func() {
		defer func() {
			_ = reader.Close()
			b.stdOutWg.Done()
			util.LogDebug("[Exit]: StdOut/Err, log size = %d", cmdResult.LogSize)
		}()

		scanner := bufio.NewScanner(reader)

		for {
			select {
			case <-ctx.Done():
				return
			default:
				if !scanner.Scan() {
					time.Sleep(1 * time.Second)
					break
				}

				line := scanner.Text()

				// to read system env vars in the end
				if strings.EqualFold(line, b.endTag) {
					for scanner.Scan() {
						envLine := scanner.Text()
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
					return
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

//====================================================================
//	util
//====================================================================

func createWorkDir(dir string) error {
	if !util.IsEmptyString(dir) {
		err := os.MkdirAll(dir, os.ModePerm)
		if util.HasError(err) {
			return err
		}
	}
	return nil
}

func getExitCode(cmd *exec.Cmd) int {
	ws := cmd.ProcessState.Sys().(syscall.WaitStatus)
	return ws.ExitStatus()
}

func matchEnvFilter(env string, filters []string) bool {
	for _, filter := range filters {
		if strings.HasPrefix(env, filter) {
			return true
		}
	}
	return false
}

func appendNewLine(script string) string {
	if !strings.HasSuffix(script, util.UnixLineBreak) {
		script += util.UnixLineBreak
	}
	return script
}
