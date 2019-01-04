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

	"github.com/flowci/flow-agent-x/domain"
	"github.com/flowci/flow-agent-x/util"
	"github.com/google/uuid"
)

const (
	linuxBash      = "/bin/bash"
	lineBreakLinux = "\n"
	logBufferSize  = 1000
)

type LogChannel chan *domain.LogItem

type ShellExecutor struct {
	CmdIn          *domain.CmdIn
	EndTerm        string
	Result         *domain.ExecutedCmd
	TimeOutSeconds time.Duration
	LogChannel     LogChannel
	Command        *exec.Cmd
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

	return &ShellExecutor{
		CmdIn:          cmdIn,
		EndTerm:        fmt.Sprintf("=====EOF-%s=====", uuid),
		Result:         result,
		LogChannel:     make(chan *domain.LogItem, logBufferSize),
		TimeOutSeconds: time.Duration(cmdIn.Timeout),
	}
}

// Run run shell scripts
func (e *ShellExecutor) Run() error {
	defer cleanUp(e)

	cmd := exec.Command(linuxBash)
	e.Command = cmd

	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	// set env varaibles for cmd
	inputs := e.CmdIn.Inputs
	if !domain.NilOrEmpty(inputs) {
		cmd.Env = inputs.ToStringArray()
	}

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

	go pushToTotalChannel(e, stdOutChannel, stdErrChannel, e.LogChannel)
	go handleStdIn(createExecScripts(e), stdin)
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
	if e.Command == nil {
		return nil
	}

	err := e.Command.Process.Kill()

	result := e.Result
	result.FinishAt = time.Now()
	result.Code = domain.CmdExitCodeKilled
	result.Status = domain.CmdStatusKilled

	return err
}

func cleanUp(e *ShellExecutor) {
	close(e.LogChannel)
}

func createExecScripts(e *ShellExecutor) []string {
	if len(e.CmdIn.EnvFilters) == 0 {
		return e.CmdIn.Scripts
	}

	echoEndTerm := fmt.Sprintf("echo %s%s", e.EndTerm, lineBreakLinux)
	printEnvs := fmt.Sprintf("env%s", lineBreakLinux)
	return append(e.CmdIn.Scripts, echoEndTerm, printEnvs)
}

func handleStdIn(scripts []string, stdin io.WriteCloser) {
	defer func() {
		stdin.Close()
		util.LogDebug("Stdin: thread exited")
	}()

	for _, script := range scripts {
		if !strings.HasSuffix(script, lineBreakLinux) {
			script += lineBreakLinux
		}
		io.WriteString(stdin, script)
	}
}

func handleStdOut(e *ShellExecutor, reader io.ReadCloser, channel LogChannel, t domain.LogType) {
	defer func() {
		reader.Close()
		close(channel)
		util.LogDebug("Stdout: thread exited for %s", t)
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
	defer util.LogDebug("Log channel producer exited")

	var counter uint32
	var numOfLine int64

	for counter < 2 {
		select {
		case outLog, ok := <-out:
			if !ok {
				atomic.AddUint32(&counter, 1)
				continue
			}

			outLog.CmdID = e.CmdIn.ID
			outLog.Number = atomic.AddInt64(&numOfLine, 1)
			total <- outLog

		case errLog, ok := <-err:
			if !ok {
				atomic.AddUint32(&counter, 1)
				continue
			}

			errLog.CmdID = e.CmdIn.ID
			errLog.Number = atomic.AddInt64(&numOfLine, 1)
			total <- errLog
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
