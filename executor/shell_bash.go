// +build !windows

package executor

import (
	"context"
	"fmt"
	"github.com/creack/pty"
	"github/flowci/flow-agent-x/util"
	"io/ioutil"
	"os"
	"os/exec"
)

// Start run the cmd from domain.CmdIn
func (b *shellExecutor) Start() (out error) {
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
	command.Env = append(os.Environ(), b.vars.ToStringArray()...)

	stdin, err := command.StdinPipe()
	util.PanicIfErr(err)

	stdout, err := command.StdoutPipe()
	util.PanicIfErr(err)

	stderr, err := command.StderrPipe()
	util.PanicIfErr(err)

	defer func() {
		_ = stdin.Close()
		_ = stdout.Close()
		_ = stderr.Close()
	}()

	b.command = command

	// handle context error
	go func() {
		<-b.context.Done()
		err := b.context.Err()

		if err != nil {
			b.handleErrors(err)
		}
	}()

	// start command
	if err := command.Start(); err != nil {
		return b.toErrorStatus(err)
	}

	b.writeLog(stdout, true, true)
	b.writeLog(stderr, true, true)
	b.writeCmd(stdin, b.setupBin, b.writeEnv, func(script string) string {
		return script
	})
	b.toStartStatus(command.Process.Pid)

	// wait or timeout
	_ = command.Wait()
	util.LogDebug("[Done]: Shell for %s", b.inCmd.ID)

	b.exportEnv()

	// wait for tty if it's running
	if b.IsInteracting() {
		util.LogDebug("Tty is running, wait..")
		<-b.ttyCtx.Done()
	}

	if b.result.IsFinishStatus() {
		return nil
	}

	// to finish status
	b.toFinishStatus(getExitCode(command))
	return b.context.Err()
}

func (b *shellExecutor) StartTty(ttyId string, onStarted func(ttyId string)) (out error) {
	defer func() {
		if err := recover(); err != nil {
			out = err.(error)
		}

		b.tty = nil
		b.ttyId = ""
	}()

	if b.IsInteracting() {
		panic(fmt.Errorf("interaction is ongoning"))
	}

	c := exec.Command(linuxBash)
	c.Dir = b.workDir
	c.Env = append(os.Environ(), b.vars.ToStringArray()...)

	ptmx, err := pty.Start(c)
	util.PanicIfErr(err)

	b.tty = c
	b.ttyId = ttyId
	b.ttyCtx, b.ttyCancel = context.WithCancel(b.context)

	defer func() {
		_ = ptmx.Close()
		b.ttyCancel()
		b.ttyCtx = nil
		b.ttyCancel = nil
	}()

	onStarted(ttyId)

	go b.writeTtyIn(ptmx)
	go b.writeTtyOut(ptmx)

	_ = c.Wait()
	return
}

func (b *shellExecutor) setupBin() []string {
	return []string{fmt.Sprintf("export PATH=%s:$PATH", b.binDir)}
}

func (b *shellExecutor) writeEnv() []string {
	tmpFile, err := ioutil.TempFile("", "agent_env_")
	util.PanicIfErr(err)

	defer tmpFile.Close()

	b.envFile = tmpFile.Name()
	return []string{"env > " + tmpFile.Name()}
}
