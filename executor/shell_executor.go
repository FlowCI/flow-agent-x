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
	logBufferSize = 10000
)

// LogChannel send out LogItem
type LogChannel chan *domain.LogItem

type RawChannel chan string

// CmdChannel receive shell script in string
type CmdChannel chan string

type ShellExecutor struct {
	CmdIn          *domain.CmdIn
	EndTerm        string
	Result         *domain.ExecutedCmd
	TimeOutSeconds time.Duration

	channel struct {
		in     CmdChannel
		out    LogChannel
		outRaw RawChannel
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
	executor.channel.outRaw = make(RawChannel, logBufferSize)

	return executor
}

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
		close(e.channel.outRaw)
	}()

	// ---- start to monitor raw log ----

	//file, _ := os.Create("hello.lo
	// g")
	//defer file.Close()
	//
	//monitor, mIn, mOut, mErr := createCommand(e.CmdIn.WorkDir)
	//_ = monitor.Start()
	//_, _ = io.WriteString(mIn, appendNewLine("tail -f hello.log"))

	// channel for stdout and stderr

	// ---- start to execute command ----
	cmd, cmdIn, cmdStdOut, cmdStdErr := createCommand(e.CmdIn)
	cmd.Env = getInputs(e.CmdIn)

	done := make(chan error)
	defer close(done)

	if err := cmd.Start(); err != nil {
		return err
	}

	e.command = cmd
	e.Result.ProcessId = cmd.Process.Pid
	e.Result.StartAt = time.Now()

	if e.CmdIn.HasScripts() {
		go produceCmd(e.channel.in, e)
	}

	go readStdOut(e, cmdStdOut, e.channel.out)
	go readStdOut(e, cmdStdErr, e.channel.out)

	go consumeCmd(e.channel.in, cmdIn)
	go func() { done <- cmd.Wait() }()

	// wait for done
	select {
	case err := <-done:
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

func createCommand(cmdIn *domain.CmdIn) (command *exec.Cmd, in io.WriteCloser, stdout io.ReadCloser, stderr io.ReadCloser) {
	command = exec.Command(linuxBash)
	command.Dir = cmdIn.WorkDir

	in, _ = command.StdinPipe()
	stdout, _ = command.StdoutPipe()
	stderr, _ = command.StderrPipe()

	return command, in, stdout, stderr
}

func getInputs(cmdIn *domain.CmdIn) []string {
	if !domain.NilOrEmpty(cmdIn.Inputs) {
		return cmdIn.Inputs.ToStringArray()
	}

	return []string{}
}

func produceCmd(channel chan string, e *ShellExecutor) {
	defer close(channel)

	cmdIn := e.CmdIn
	endTerm := e.EndTerm

	set := "set -e"
	if cmdIn.AllowFailure {
		set = "set +e"
	}

	channel <- set

	for _, script := range cmdIn.Scripts {
		channel <- script
	}

	if len(cmdIn.EnvFilters) > 0 {
		channel <- fmt.Sprintf("echo %s%s", endTerm, util.UnixLineBreakStr)
		channel <- fmt.Sprintf("env%s", util.UnixLineBreakStr)
	}
}

func consumeCmd(channel chan string, stdin io.WriteCloser) {
	defer func() {
		_ = stdin.Close()
		util.LogDebug("Exit: stdin thread exited")
	}()

	for {
		str, ok := <-channel
		if !ok {
			break
		}

		if str == ExitCmd {
			close(channel)
			break
		}

		_, _ = io.WriteString(stdin, appendNewLine(str))
	}
}

func readStdOut(e *ShellExecutor, reader io.ReadCloser, channel LogChannel) {
	var rows int64

	defer func() {
		reader.Close()
		atomic.AddInt64(&e.Result.LogSize, rows)
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
		channel <- &domain.LogItem{CmdID: e.CmdIn.ID, Content: line}
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
