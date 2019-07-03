package executor

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
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

	logBufferSize = 10000

	// s/\x1b\[[0-9;]*m//g
	// s/\x1b\[[0-9;]*[a-zA-Z]//g
	StripColor = "perl -pe 's/\\x1b\\[[0-9;]*[a-zA-Z]//g'"

	MacScriptPattern   = "script -a -F -q %s %s | %s ; exit ${PIPESTATUS[0]}"
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
	LogDir         string

	EnableRawLog       bool // using 'script' to record raw print out
	EnableInteractMode bool

	Path struct {
		Shell string
		Log   string
		Raw   string
		Tmp   string
	}

	channel struct {
		in  CmdChannel
		out LogChannel
		raw RawChannel
	}

	instance struct {
		monitor *CmdInstance
		shell   *CmdInstance
	}

	waitForLogging sync.WaitGroup
}

//====================================================================
//	Create new ShellExecutor
//====================================================================

func NewShellExecutor(cmdIn *domain.CmdIn, logDir string) *ShellExecutor {
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
		EnableRawLog:       false,
		EnableInteractMode: false,
		LogDir:             logDir,
	}

	// init path for shell, log and raw log
	cmdId := executor.CmdIn.ID
	executor.Path.Shell = filepath.Join(executor.LogDir, cmdId+".sh")
	executor.Path.Log = filepath.Join(executor.LogDir, cmdId+".log")
	executor.Path.Raw = filepath.Join(executor.LogDir, cmdId+".raw.log")
	executor.Path.Tmp = filepath.Join(executor.LogDir, cmdId+".raw.tmp")

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

		_ = os.Remove(e.Path.Tmp)
		//_ = os.Remove(e.Path.Shell)
	}()

	// --- write script into {cmd id}.sh and make it executable
	err := writeScriptToFile(e)
	if util.HasError(err) {
		return e.toErrorStatus(err)
	}

	// ---- start to monitor raw log ----
	if e.EnableRawLog {
		e.waitForLogging.Add(1)

		// create tmp log output file
		tmpLogFile, _ := os.Create(e.Path.Tmp)
		_ = tmpLogFile.Close()

		// tail -f the raw log file
		monitorInstance := createCommand(e.CmdIn)
		e.instance.monitor = monitorInstance

		go readRawOut(e, monitorInstance.stdOut)
		_ = monitorInstance.command.Start()
		_, _ = io.WriteString(monitorInstance.stdIn, appendNewLine(fmt.Sprintf("tail -f %s", tmpLogFile.Name())))
	}

	// ---- start to execute command ----
	shellInstance := createCommand(e.CmdIn)
	shellInstance.command.Env = getInputs(e.CmdIn)

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
		go produceCmd(e, e.Path.Shell)
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
				cmdToRun = fmt.Sprintf(MacScriptPattern, e.Path.Tmp, cmdToRun, StripColor)
			}

			if util.IsLinux() {
				cmdToRun = fmt.Sprintf(LinuxScriptPattern, cmdToRun, e.Path.Tmp, StripColor)
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
	f, _ := os.Create(e.Path.Raw)
	writer := bufio.NewWriter(f)

	defer func() {
		_ = writer.Flush()
		_ = reader.Close()
		_ = f.Close()

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
		writeLogToFile(writer, line)
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
