package executor

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"flow-agent-x/domain"
	"flow-agent-x/util"

	"github.com/google/uuid"
)

const (
	// ExitCmd script 'exit'
	ExitCmd = "exit"

	logBufferSize = 10000

	// s/\x1b\[[0-9;]*m//g
	// s/\x1b\[[0-9;]*[a-zA-Z]//g
	StripColor = "perl -pe 's/\\x1b\\[[0-9;]*[a-zA-Z]//g'"

	MacScriptPattern   = "script -a -F -q %s %s | %s ; exit ${PIPESTATUS[0]}"
	LinuxScriptPattern = "script -a -e -f -q -c \"%s\" %s | %s ; exit ${PIPESTATUS[0]}"

	SetPS1 = "PS1='$ '"
	SourceBashrc = "source ~/.bashrc 2> /dev/null"
	SourceBashProfile = "source ~!/.bash_profile 2> /dev/null"
)

type (
	// LogChannel send out LogItem
	LogChannel chan *domain.LogItem

	RawChannel chan string

	// CmdChannel receive shell script in string
	CmdChannel chan string

	ShellExecutor struct {
		CmdIn          *domain.CmdIn
		EndTerm        string
		Result         *domain.ExecutedCmd
		TimeOutSeconds time.Duration

		EnableInteractMode bool

		channel struct {
			in  CmdChannel
			out LogChannel
		}

		instance struct {
			monitor *CmdInstance
			shell   *CmdInstance
		}

		waitForLogging sync.WaitGroup
	}
)

//====================================================================
//	Create new ShellExecutor
//====================================================================

func NewShellExecutor(cmdIn *domain.CmdIn) *ShellExecutor {
	result := &domain.ExecutedCmd{
		Cmd: domain.Cmd{
			ID:           cmdIn.ID,
			AllowFailure: cmdIn.AllowFailure,
			Plugin:       cmdIn.Plugin,
		},
		Code:   domain.CmdExitCodeUnknown,
		Status: domain.CmdStatusPending,
		Output: make(domain.Variables),
	}

	endTermUUID, _ := uuid.NewRandom()

	executor := &ShellExecutor{
		CmdIn:              cmdIn,
		EndTerm:            fmt.Sprintf("=====EOF-%s=====", endTermUUID),
		Result:             result,
		TimeOutSeconds:     time.Duration(cmdIn.Timeout),
		EnableInteractMode: false,
	}

	// init channel
	executor.channel.in = make(CmdChannel)
	executor.channel.out = make(LogChannel, logBufferSize)

	// init logging wait group
	executor.waitForLogging.Add(2)
	return executor
}

//====================================================================
//	Public
//====================================================================

func (e *ShellExecutor) GetCmdChannel() chan<- string {
	return e.channel.in
}

// GetLogChannel export log channel for consumer
func (e *ShellExecutor) GetLogChannel() <-chan *domain.LogItem {
	return e.channel.out
}

// Run run shell scripts
func (e *ShellExecutor) Run() error {
	defer func() {
		close(e.channel.out)
	}()

	// init work dir
	if !util.IsEmptyString(e.CmdIn.WorkDir) {
		err := os.MkdirAll(e.CmdIn.WorkDir, os.ModePerm)
		if util.HasError(err) {
			return e.toErrorStatus(err)
		}
	}

	// ---- start to execute command ----
	shellInstance := createCommand(e.CmdIn)
	shellInstance.command.Env = getInputs(e.CmdIn)
	shellInstance.command.Env = append(shellInstance.command.Env, "PS1='$ '")

	done := make(chan error)

	if err := shellInstance.command.Start(); err != nil {
		return e.toErrorStatus(err)
	}

	e.instance.shell = shellInstance
	e.Result.Status = domain.CmdStatusRunning
	e.Result.ProcessId = shellInstance.command.Process.Pid
	e.Result.StartAt = time.Now()

	// start to listen output log
	go readStdOut(e, shellInstance.stdOut)
	go readStdOut(e, shellInstance.stdErr)

	// start to consume input channel
	if !e.EnableInteractMode {
		go produceCmd(e)
	}

	go consumeCmd(e, shellInstance.stdIn)

	go func() {
		// wait for cmd finished
		err := shellInstance.command.Wait()
		e.stopMonitorInstance()
		util.LogDebug("[Done]: Shell for %s", e.CmdIn.ID)

		loggingTimeout := 5 * time.Second
		if util.HasError(err) {
			loggingTimeout = 0
		}

		// wait for logging with 5 seconds
		isSuccess := util.Wait(&e.waitForLogging, loggingTimeout)
		util.LogDebug("[Done]: Logging for %s, timeout: %t", e.CmdIn.ID, !isSuccess)

		done <- err
	}()

	return waitForDone(e, done)
}

// Kill to kill executing cmd
// it will jump to 'err := <-done:' on the Run() method
func (e *ShellExecutor) Kill() error {
	shellInstance := e.instance.shell

	if shellInstance == nil {
		return nil
	}

	err := shellInstance.command.Process.Kill()
	e.stopMonitorInstance()

	result := e.Result
	result.FinishAt = time.Now()
	result.Code = domain.CmdExitCodeKilled
	result.Status = domain.CmdStatusKilled
	return err
}

//====================================================================
//	Private
//====================================================================

func (e *ShellExecutor) stopMonitorInstance() {
	monitorInstance := e.instance.monitor

	if monitorInstance == nil {
		return
	}

	_ = monitorInstance.stdIn.Close()
	_ = monitorInstance.stdOut.Close()
}

func (e *ShellExecutor) toErrorStatus(err error) error {
	e.Result.Status = domain.CmdStatusException
	e.Result.Error = err.Error()
	return err
}

//====================================================================
//	Utils
//====================================================================

func waitForDone(e *ShellExecutor, done chan error) error {
	select {
	case err := <-done:
		defer close(done)

		result := e.Result
		result.FinishAt = time.Now()

		// get wait status
		ws := e.instance.shell.command.ProcessState.Sys().(syscall.WaitStatus)
		result.Code = ws.ExitStatus()

		// success status if no err
		if !util.HasError(err) {
			result.Status = domain.CmdStatusSuccess
			return nil
		}

		// return if cmd kill by Kill() method
		if e.Result.Code == domain.CmdExitCodeKilled {
			return nil
		}

		exitError, ok := err.(*exec.ExitError)
		_ = e.toErrorStatus(exitError)

		if ok {
			return nil
		}

		return exitError

	case <-time.After(e.TimeOutSeconds * time.Second):
		util.LogDebug("Cmd killed since timeout")
		err := e.Kill()

		result := e.Result
		result.Code = domain.CmdExitCodeTimeOut
		result.Status = domain.CmdStatusTimeout

		return err
	}
}

func getInputs(cmdIn *domain.CmdIn) []string {
	if !domain.NilOrEmpty(cmdIn.Inputs) {
		return cmdIn.Inputs.ToStringArray()
	}

	return []string{}
}

func produceCmd(e *ShellExecutor) {
	defer close(e.channel.in)

	cmdIn := e.CmdIn
	endTerm := e.EndTerm

	// setup allow failure
	set := "set -e"
	if cmdIn.AllowFailure {
		set = "set +e"
	}

	// setup bash env
	e.channel.in <- SetPS1
	e.channel.in <- SourceBashrc
	e.channel.in <- SourceBashProfile
	e.channel.in <- set

	// write scripts
	for _, script := range e.CmdIn.Scripts {
		e.channel.in <- script
	}

	// write for end term
	if len(cmdIn.EnvFilters) > 0 {
		e.channel.in <- "echo " + endTerm
		e.channel.in <- "env"
	}
}

func consumeCmd(e *ShellExecutor, stdin io.WriteCloser) {
	defer func() {
		_ = stdin.Close()
		util.LogDebug("[Exit]: consumeCmd")
	}()

	channel := e.channel.in

	for {
		cmdToRun, ok := <-channel
		if !ok {
			break
		}

		if cmdToRun == ExitCmd {
			close(channel)
			break
		}

		_, _ = io.WriteString(stdin, appendNewLine(cmdToRun))
	}
}

func readStdOut(e *ShellExecutor, reader io.ReadCloser) {
	var rows int64

	defer func() {
		_ = reader.Close()

		atomic.AddInt64(&e.Result.LogSize, rows)
		util.LogDebug("Log size: === %d", e.Result.LogSize)

		util.LogDebug("[Exit]: %s", "readStdOut")
		e.waitForLogging.Done()
	}()

	scanner := bufio.NewScanner(reader)

	for scanner.Scan() {
		line := scanner.Text()

		// to read system env vars in the end
		if strings.EqualFold(line, e.EndTerm) {
			for scanner.Scan() {
				envLine := scanner.Text()
				index := strings.IndexAny(envLine, "=")
				if index == -1 {
					continue
				}

				key := envLine[0:index]
				value := envLine[index+1:]

				if matchEnvFilter(key, e.CmdIn.EnvFilters) {
					e.Result.Output[key] = value
				}
			}
			return
		}

		// send log item instance to channel
		rows++
		e.channel.out <- &domain.LogItem{CmdID: e.CmdIn.ID, Content: line}
	}
}
