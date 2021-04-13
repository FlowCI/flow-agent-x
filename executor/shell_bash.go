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
func (se *shellExecutor) doStart() (out error) {
	defer func() {
		if err := recover(); err != nil {
			out = err.(error)
			se.handleErrors(out)
		}
	}()

	// init wait group fro StdOut and StdErr
	se.stdOutWg.Add(2)

	command := exec.Command(linuxBash)
	command.Dir = se.jobDir
	command.Env = append(os.Environ(), se.vars.ToStringArray()...)
	command.Env = append(command.Env, se.secretVars.ToStringArray()...)

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

	se.command = command

	// start command
	if err := command.Start(); err != nil {
		return se.toErrorStatus(err)
	}

	se.writeLog(stdout, true, true)
	se.writeLog(stderr, true, true)
	se.writeCmd(stdin, se.setupBin, se.writeEnv, func(script string) string {
		return script
	})
	se.toStartStatus(command.Process.Pid)

	// wait or timeout
	_ = command.Wait()
	util.LogDebug("[Done]: Shell for %s", se.inCmd.ID)

	se.exportEnv()

	// wait for tty if it's running
	if se.IsInteracting() {
		util.LogDebug("Tty is running, wait..")
		<-se.ttyCtx.Done()
	}

	if se.result.IsFinishStatus() {
		return nil
	}

	// to finish status
	se.toFinishStatus(getExitCode(command))
	return se.context.Err()
}

func (se *shellExecutor) StartTty(ttyId string, onStarted func(ttyId string)) (out error) {
	defer func() {
		if err := recover(); err != nil {
			out = err.(error)
		}

		se.tty = nil
		se.ttyId = ""
	}()

	if se.IsInteracting() {
		panic(fmt.Errorf("interaction is ongoning"))
	}

	c := exec.Command(linuxBash)
	c.Dir = se.jobDir
	c.Env = append(os.Environ(), se.vars.ToStringArray()...)
	c.Env = append(c.Env, se.secretVars.ToStringArray()...)

	ptmx, err := pty.Start(c)
	util.PanicIfErr(err)

	se.tty = c
	se.ttyId = ttyId
	se.ttyCtx, se.ttyCancel = context.WithCancel(se.context)

	defer func() {
		_ = ptmx.Close()
		se.ttyCancel()
		se.ttyCtx = nil
		se.ttyCancel = nil
	}()

	onStarted(ttyId)

	go se.writeTtyIn(ptmx)
	go se.writeTtyOut(ptmx)

	_ = c.Wait()
	return
}

func (se *shellExecutor) setupBin() []string {
	return []string{fmt.Sprintf("export PATH=%s:$PATH", se.binDir)}
}

func (se *shellExecutor) writeEnv() []string {
	tmpFile, err := ioutil.TempFile("", "agent_env_")
	util.PanicIfErr(err)

	defer tmpFile.Close()

	se.envFile = tmpFile.Name()
	return []string{"env > " + tmpFile.Name()}
}
