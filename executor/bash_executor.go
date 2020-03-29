package executor

import (
	"context"
	"github/flowci/flow-agent-x/util"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

type (
	BashExecutor struct {
		BaseExecutor
		command *exec.Cmd
	}
)

// Start run the cmd from domain.CmdIn
func (b *BashExecutor) Start() (out error) {
	defer func() {
		if err := recover(); err != nil {
			out = err.(error)
			b.handleErrors(out)
		}

		b.closeChannels()
	}()

	// init wait group fro StdOut and StdErr
	b.stdOutWg.Add(2)

	command := exec.Command(linuxBash)
	command.Dir = b.workDir

	stdin, _ := command.StdinPipe()
	stdout, _ := command.StdoutPipe()
	stderr, _ := command.StderrPipe()

	command.Env = append(os.Environ(), b.inCmd.VarsToStringArray()...)
	command.Env = append(command.Env, b.inVars.ToStringArray()...)

	b.command = command
	b.startToHandleContext()

	// start command
	if err := command.Start(); err != nil {
		return b.toErrorStatus(err)
	}

	onStdOutExit := func() {
		b.stdOutWg.Done()
		util.LogDebug("[Exit]: StdOut/Err, log size = %d", b.CmdResult.LogSize)
	}

	cancelForStdOut := b.startConsumeStdOut(stdout, onStdOutExit)
	cancelForStdErr := b.startConsumeStdOut(stderr, onStdOutExit)
	cancelForStdIn := b.startConsumeStdIn(stdin)

	defer func() {
		cancelForStdOut()
		cancelForStdErr()
		cancelForStdIn()
	}()

	b.toStartStatus(command.Process.Pid)

	// wait or timeout
	_ = command.Wait()
	util.LogDebug("[Done]: Shell for %s", b.CmdID())

	if b.CmdResult.IsFinishStatus() {
		return nil
	}

	// to finish status
	b.toFinishStatus(getExitCode(command))
	return b.context.Err()
}

//====================================================================
//	private
//====================================================================

func (b *BashExecutor) startToHandleContext() {
	go func() {
		<-b.context.Done()
		err := b.context.Err()

		if err != nil {
			b.handleErrors(err)
		}
	}()
}

func (b *BashExecutor) handleErrors(err error) {
	kill := func() {
		if b.command != nil {
			_ = b.command.Process.Kill()
		}
	}

	if err == context.DeadlineExceeded {
		util.LogDebug("Timeout..")
		kill()
		b.toTimeOutStatus()
		return
	}

	if err == context.Canceled {
		util.LogDebug("Cancel..")
		kill()
		b.toKilledStatus()
		return
	}

	_ = b.toErrorStatus(err)
}

//====================================================================
//	util
//====================================================================

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
