// +build windows

package executor

import (
	"context"
	"fmt"
	"github/flowci/flow-agent-x/util"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
)

func (se *shellExecutor) doStart() (out error) {
	defer func() {
		if err := recover(); err != nil {
			out = err.(error)
			se.handleErrors(out)
		}
	}()

	path, err := exec.LookPath(winPowerShell)
	util.PanicIfErr(err)

	ps1File := se.writeScriptToTmpFile()
	defer func() {
		_ = os.Remove(ps1File)
	}()

	// init wait group fro StdOut and StdErr
	se.stdOutWg.Add(2)

	command := exec.Command(path, "-NoLogo", "-NoProfile", "-NonInteractive", "-File", ps1File)
	command.Dir = se.workDir
	command.Env = append(os.Environ(), se.vars.ToStringArray()...)

	stdout, err := command.StdoutPipe()
	util.PanicIfErr(err)

	stderr, err := command.StderrPipe()
	util.PanicIfErr(err)

	se.command = command

	defer func() {
		_ = stdout.Close()
		_ = stderr.Close()
	}()

	// handle context error
	go func() {
		<-se.context.Done()
		err := se.context.Err()

		if err != nil {
			se.handleErrors(err)
		}
	}()

	// start command
	if err := command.Start(); err != nil {
		return se.toErrorStatus(err)
	}

	se.writeLog(stdout, true, true)
	se.writeLog(stderr, true, true)
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

	path, err := exec.LookPath(winPowerShell)
	util.PanicIfErr(err)

	c := exec.Command(path, "-NoLogo", "-NoProfile")
	c.Dir = se.workDir
	c.Env = append(os.Environ(), se.vars.ToStringArray()...)

	stdin, err := c.StdinPipe()
	util.PanicIfErr(err)

	stdout, err := c.StdoutPipe()
	util.PanicIfErr(err)

	stderr, err := c.StderrPipe()
	util.PanicIfErr(err)

	se.tty = c
	se.ttyId = ttyId
	se.ttyCtx, se.ttyCancel = context.WithCancel(se.context)

	defer func() {
		_ = stdin.Close()
		_ = stdout.Close()
		_ = stderr.Close()

		se.ttyCancel()
		se.ttyCtx = nil
		se.ttyCancel = nil
	}()

	if err := c.Start(); err != nil {
		return se.toErrorStatus(err)
	}

	onStarted(ttyId)

	go se.writeTtyIn(stdin)
	go se.writeTtyOut(io.MultiReader(stdout, stderr))

	_ = c.Wait()
	return
}

func (se *shellExecutor) setupBin() []string {
	return []string{fmt.Sprintf("$Env:PATH += \";%s\"", se.binDir)}
}

func (se *shellExecutor) writeEnv() []string {
	tmpFile, err := ioutil.TempFile("", "agent_env_")
	util.PanicIfErr(err)

	defer tmpFile.Close()

	se.envFile = tmpFile.Name()
	return []string{"gci env: > " + tmpFile.Name()}
}

func (se *shellExecutor) writeScriptToTmpFile() string {
	tempScriptFile, err := ioutil.TempFile("", "agent_tmp_script_")
	util.PanicIfErr(err)
	_ = tempScriptFile.Close()

	psTmpFile := tempScriptFile.Name() + ".ps1"

	err = os.Rename(tempScriptFile.Name(), psTmpFile)
	util.PanicIfErr(err)

	// open tmp ps file
	file, err := os.OpenFile(psTmpFile, os.O_RDWR, 0)
	util.PanicIfErr(err)

	doScript := func(script string) string {
		return script
	}

	se.writeCmd(file, se.setupBin, se.writeEnv, doScript)

	_ = file.Close()
	return file.Name()
}
