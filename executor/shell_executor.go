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

//====================================================================
//	Const
//====================================================================

const (
	// ExitCmd script 'exit'
	ExitCmd = "exit"

	linuxBash     = "/bin/bash"
	logBufferSize = 10000

	// s/\x1b\[[0-9;]*m//g
	// s/\x1b\[[0-9;]*[a-zA-Z]//g
	StripColor = "perl -pe 's/\\x1b\\[[0-9;]*[a-zA-Z]//g'"

	MacScriptPattern = "script -a -F -q %s %s | %s ; exit ${PIPESTATUS[0]}"
	LinuxScriptPattern = "script -a -e -f -q -c \"%s\" %s | %s ; exit ${PIPESTATUS[0]}"
)

//====================================================================
//	Definition
//====================================================================

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

	EnableRawLog       bool // using 'script' to record raw print out
	EnableInteractMode bool

	Path struct {
		Shell  string
		Log    string
		RawLog string
	}

	channel struct {
		in  CmdChannel
		out LogChannel
		raw RawChannel
	}

	runner struct {
		monitor *exec.Cmd
		shell   *exec.Cmd
	}

	waitForLogging sync.WaitGroup
}

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
		Code:   domain.CmdExitCodeUnknow,
		Status: domain.CmdStatusPending,
		Output: make(domain.Variables),
	}

	endTermUUID, _ := uuid.NewRandom()

	executor := &ShellExecutor{
		CmdIn:              cmdIn,
		EndTerm:            fmt.Sprintf("=====EOF-%s=====", endTermUUID),
		Result:             result,
		TimeOutSeconds:     time.Duration(cmdIn.Timeout),
		EnableRawLog:       false,
		EnableInteractMode: false,
	}

	// init path for shell, log and raw log
	executor.Path.Shell = getShellFilePath(cmdIn.ID)
	executor.Path.Log = getLogFilePath(cmdIn.ID)
	executor.Path.RawLog = getRawLogFilePath(cmdIn.ID)

	// init channel
	executor.channel.in = make(CmdChannel)
	executor.channel.out = make(LogChannel, logBufferSize)
	executor.channel.raw = make(RawChannel, logBufferSize)

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

func (e *ShellExecutor) GetRawChannel() <-chan string {
	return e.channel.raw
}

// Run run shell scripts
func (e *ShellExecutor) Run() error {
	defer func() {
		close(e.channel.out)
		close(e.channel.raw)
	}()

	// --- write script into {cmd id}.sh and make it executable
	err := writeScriptToFile(e)
	if util.HasError(err) {
		return err
	}

	// ---- start to monitor raw log ----
	if e.EnableRawLog {
		e.waitForLogging.Add(1)

		// create raw log output file
		rawLogFile, _ := os.Create(e.Path.RawLog)
		_ = rawLogFile.Close()

		// tail -f the raw log file
		monitor, mIn, mOut, _ := createCommand(e.CmdIn)
		e.setMonitorRunner(monitor)

		go readRawOut(e, mOut)
		_ = monitor.Start()
		_, _ = io.WriteString(mIn, appendNewLine(fmt.Sprintf("tail -f %s", e.Path.RawLog)))
	}

	// ---- start to execute command ----
	cmd, cmdIn, cmdStdOut, cmdStdErr := createCommand(e.CmdIn)
	cmd.Env = getInputs(e.CmdIn)

	done := make(chan error)

	if err := cmd.Start(); err != nil {
		return err
	}

	e.setShellRunner(cmd)
	e.Result.ProcessId = cmd.Process.Pid
	e.Result.StartAt = time.Now()

	// start to listen output log
	go readStdOut(e, cmdStdOut)
	go readStdOut(e, cmdStdErr)

	// start to consume input channel
	if !e.EnableInteractMode {
		go produceCmd(e, e.Path.Shell)
	}
	go consumeCmd(e, cmdIn)

	go func() {
		// wait for cmd finished
		err := cmd.Wait()
		util.LogDebug("[Done]: Shell for %s", e.CmdIn.ID)

		loggingTimeout := 5 * time.Second
		if util.HasError(err) {
			loggingTimeout = 0
		}

		// wait for logging with 5 seconds
		util.Wait(&e.waitForLogging, loggingTimeout)
		util.LogDebug("[Done]: Logging for %s", e.CmdIn.ID)

		done <- err
	}()

	return waitForDone(e, done)
}

// Kill to kill executing cmd
// it will jump to 'err := <-done:' on the Run() method
func (e *ShellExecutor) Kill() error {
	shellRunner := e.getShellRunner()

	if shellRunner == nil {
		return nil
	}

	err := shellRunner.Process.Kill()

	result := e.Result
	result.FinishAt = time.Now()
	result.Code = domain.CmdExitCodeKilled
	result.Status = domain.CmdStatusKilled

	monitorRunner := e.getMonitorRunner()

	if monitorRunner == nil {
		return err
	}

	_ = monitorRunner.Process.Kill()
	return nil
}

//====================================================================
//	Private
//====================================================================

func (e *ShellExecutor) setMonitorRunner(cmd *exec.Cmd) {
	e.runner.monitor = cmd
}

func (e *ShellExecutor) getMonitorRunner() *exec.Cmd {
	return e.runner.monitor
}

func (e *ShellExecutor) setShellRunner(cmd *exec.Cmd) {
	e.runner.shell = cmd
}

func (e *ShellExecutor) getShellRunner() *exec.Cmd {
	return e.runner.shell
}

//====================================================================
//	Utils
//====================================================================

func createCommand(cmdIn *domain.CmdIn) (command *exec.Cmd, in io.WriteCloser, stdout io.ReadCloser, stderr io.ReadCloser) {
	command = exec.Command(linuxBash)
	command.Dir = cmdIn.WorkDir

	in, _ = command.StdinPipe()
	stdout, _ = command.StdoutPipe()
	stderr, _ = command.StderrPipe()

	return command, in, stdout, stderr
}

func waitForDone(e *ShellExecutor, done chan error) error {
	select {
	case err := <-done:
		defer close(done)

		result := e.Result
		result.FinishAt = time.Now()

		// get wait status
		ws := e.getShellRunner().ProcessState.Sys().(syscall.WaitStatus)
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

func getInputs(cmdIn *domain.CmdIn) []string {
	if !domain.NilOrEmpty(cmdIn.Inputs) {
		return cmdIn.Inputs.ToStringArray()
	}

	return []string{}
}

func produceCmd(e *ShellExecutor, script string) {
	defer close(e.channel.in)
	e.channel.in <- script
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

		if e.EnableRawLog {
			if util.IsMac() {
				cmdToRun = fmt.Sprintf(MacScriptPattern, e.Path.RawLog, cmdToRun, StripColor)
			}

			if util.IsLinux() {
				cmdToRun = fmt.Sprintf(LinuxScriptPattern, cmdToRun, e.Path.RawLog, StripColor)
			}

			if util.IsWindows() {
				// unsupported
			}
		}

		_, _ = io.WriteString(stdin, appendNewLine(cmdToRun))
	}
}

func readRawOut(e *ShellExecutor, reader io.ReadCloser) {
	var rows int64

	defer func() {
		_ = reader.Close()
		atomic.AddInt64(&e.Result.LogSize, rows)
		util.LogDebug("[Exit]: %s", "readRawOut")
		e.waitForLogging.Done()
	}()

	scanner := bufio.NewScanner(reader)

	for scanner.Scan() {
		line := scanner.Text()

		if strings.EqualFold(line, e.EndTerm) {
			return
		}

		rows++
		e.channel.raw <- line
	}
}

func readStdOut(e *ShellExecutor, reader io.ReadCloser) {
	var rows int64
	f, _ := os.Create(e.Path.Log)
	writer := bufio.NewWriter(f)

	defer func() {
		_ = writer.Flush()
		_ = reader.Close()
		_ = f.Close()

		if !e.EnableRawLog {
			atomic.AddInt64(&e.Result.LogSize, rows)
			util.LogDebug("Log size: === %d", e.Result.LogSize)
		}

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

		// write to file and send log item instance to channel
		rows++
		writeLogToFile(writer, line)
		e.channel.out <- &domain.LogItem{CmdID: e.CmdIn.ID, Content: line}
	}
}
