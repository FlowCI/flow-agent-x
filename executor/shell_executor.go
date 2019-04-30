package executor

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"strings"
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

	linuxBash     = "/bin/bash"
	logBufferSize = 1000
)

// LogChannel send out LogItem
type LogChannel chan *domain.LogItem

// CmdChannel recevie shell script in string
type CmdChannel chan string

type ShellExecutor struct {
	CmdIn          *domain.CmdIn
	EndTerm        string
	Result         *domain.ExecutedCmd
	TimeOutSeconds time.Duration

	channel struct {
		in  CmdChannel
		out LogChannel
	}

	command *exec.Cmd
}

// NewShellExecutor new instance of shell executor
func NewShellExecutor(cmdIn *domain.CmdIn) *ShellExecutor {
	result := &domain.ExecutedCmd{
		Cmd: domain.Cmd{
			ID:           cmdIn.ID,
			AllowFailure: cmdIn.AllowFailure,
			Plugin:       cmdIn.Plugin,
		},
		Code:   domain.CmdExitCodeUnknow,
		Status: domain.CmdStatusPending,
		Output: make(domain.Variables),
	}

	uuid, _ := uuid.NewRandom()

	executor := &ShellExecutor{
		CmdIn:          cmdIn,
		EndTerm:        fmt.Sprintf("=====EOF-%s=====", uuid),
		Result:         result,
		TimeOutSeconds: time.Duration(cmdIn.Timeout),
	}

	executor.channel.in = make(CmdChannel)
	executor.channel.out = make(LogChannel, logBufferSize)

	return executor
}

func (e *ShellExecutor) GetCmdChannel() chan<- string {
	return e.channel.in
}

// GetLogChannel receive only log channel
func (e *ShellExecutor) GetLogChannel() <-chan *domain.LogItem {
	return e.channel.out
}

// Run run shell scripts
func (e *ShellExecutor) Run() error {
	defer close(e.channel.out)

	cmd := exec.Command(linuxBash)
	cmd.Dir = e.CmdIn.WorkDir
	cmd.Env = getInputs(e.CmdIn)

	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	e.command = cmd

	// channel for stdout and stderr
	stdOutChannel := make(LogChannel, logBufferSize)
	stdErrChannel := make(LogChannel, logBufferSize)

	// channel for cmd wait
	done := make(chan error)

	if err := cmd.Start(); err != nil {
		return err
	}

	e.Result.ProcessId = cmd.Process.Pid
	e.Result.StartAt = time.Now()

	if e.CmdIn.HasScripts() {
		go func() {
			defer close(e.channel.in)
			for _, script := range createExecScripts(e) {
				e.channel.in <- script
			}
		}()
	}

	go handleCmdChannel(e.channel.in, stdin)
	go pushToTotalChannel(e, stdOutChannel, stdErrChannel, e.channel.out)
	go handleStdOut(e, stdout, stdOutChannel, domain.LogTypeOut)
	go handleStdOut(e, stderr, stdErrChannel, domain.LogTypeErr)
	go func() { done <- cmd.Wait() }()

	// wait for done
	select {
	case err := <-done:
		defer close(done)

		result := e.Result
		result.FinishAt = time.Now()

		// get wait status
		ws := cmd.ProcessState.Sys().(syscall.WaitStatus)
		result.Code = ws.ExitStatus()

		// success status if no err
		if err == nil {
			result.Status = domain.CmdStatusSuccess
			return nil
		}

		// return if cmd kill by Kill() method
		if e.Result.Code == domain.CmdExitCodeKilled {
			return nil
		}

		exitError, ok := err.(*exec.ExitError)
		result.Status = domain.CmdStatusException
		result.Error = err.Error()

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

// Kill to kill executing cmd
// it will jump to 'err := <-done:' on the Run() method
func (e *ShellExecutor) Kill() error {
	if e.command == nil {
		return nil
	}

	err := e.command.Process.Kill()

	result := e.Result
	result.FinishAt = time.Now()
	result.Code = domain.CmdExitCodeKilled
	result.Status = domain.CmdStatusKilled

	return err
}

func createExecScripts(e *ShellExecutor) []string {
	if len(e.CmdIn.EnvFilters) == 0 {
		return e.CmdIn.Scripts
	}

	echoEndTerm := fmt.Sprintf("echo %s%s", e.EndTerm, util.UnixLineBreakStr)
	printEnvs := fmt.Sprintf("env%s", util.UnixLineBreakStr)
	return append(e.CmdIn.Scripts, echoEndTerm, printEnvs)
}

func getWorkDir(cmdIn *domain.CmdIn) string {
	if util.IsEmptyString(cmdIn.WorkDir) {
		return "$HOME"
	}

	return cmdIn.WorkDir
}

func getInputs(cmdIn *domain.CmdIn) []string {
	if !domain.NilOrEmpty(cmdIn.Inputs) {
		return cmdIn.Inputs.ToStringArray()
	}

	return []string{}
}

func handleCmdChannel(scripts chan string, stdin io.WriteCloser) {
	defer func() {
		stdin.Close()
		util.LogDebug("Exit: stdin thread exited")
	}()

	for {
		str, ok := <-scripts
		if !ok {
			break
		}

		if str == ExitCmd {
			close(scripts)
			break
		}

		io.WriteString(stdin, appendNewLine(str))
	}
}

func handleStdOut(e *ShellExecutor, reader io.ReadCloser, channel LogChannel, t domain.LogType) {
	defer func() {
		reader.Close()
		close(channel)
		util.LogDebug("Exit: stdout thread exited for %s", t)
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
		channel <- &domain.LogItem{
			Type:    t,
			Content: line,
		}
	}
}

func pushToTotalChannel(e *ShellExecutor, out LogChannel, err LogChannel, total LogChannel) {
	defer util.LogDebug("Exit: log channel producer exited")

	var counter uint32
	var numOfLine int64

	push := func(item *domain.LogItem, ok bool) {
		if !ok {
			atomic.AddUint32(&counter, 1)
			return
		}

		item.CmdID = e.CmdIn.ID
		item.Number = atomic.AddInt64(&numOfLine, 1)
		total <- item

		e.Result.LogSize = item.Number
	}

	for counter < 2 {
		select {
		case outLog, ok := <-out:
			push(outLog, ok)

		case errLog, ok := <-err:
			push(errLog, ok)
		}
	}
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
	if !strings.HasSuffix(script, util.UnixLineBreakStr) {
		script += util.UnixLineBreakStr
	}
	return script
}
