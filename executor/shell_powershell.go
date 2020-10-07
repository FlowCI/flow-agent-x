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

func (b *shellExecutor) Start() (out error) {
	defer func() {
		if err := recover(); err != nil {
			out = err.(error)
			b.handleErrors(out)
		}

		b.closeChannels()
	}()

	path, err := exec.LookPath(winPowerShell)
	util.PanicIfErr(err)

	ps1File := b.writeScriptToTmpFile()
	defer func() {
		_ = os.Remove(ps1File)
	}()

	// init wait group fro StdOut and StdErr
	b.stdOutWg.Add(2)

	command := exec.Command(path, "-NoLogo", "-NoProfile", "-NonInteractive", "-File", ps1File)
	command.Dir = b.workDir
	command.Env = append(os.Environ(), b.vars.ToStringArray()...)

	stdout, err := command.StdoutPipe()
	util.PanicIfErr(err)

	stderr, err := command.StderrPipe()
	util.PanicIfErr(err)

	b.command = command

	defer func() {
		_ = stdout.Close()
		_ = stderr.Close()
	}()

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

	path, err := exec.LookPath(winPowerShell)
	util.PanicIfErr(err)

	c := exec.Command(path, "-NoLogo", "-NoProfile")
	c.Dir = b.workDir
	c.Env = append(os.Environ(), b.vars.ToStringArray()...)

	stdin, err := c.StdinPipe()
	util.PanicIfErr(err)

	stdout, err := c.StdoutPipe()
	util.PanicIfErr(err)

	stderr, err := c.StderrPipe()
	util.PanicIfErr(err)

	b.tty = c
	b.ttyId = ttyId
	b.ttyCtx, b.ttyCancel = context.WithCancel(b.context)

	defer func() {
		_ = stdin.Close()
		_ = stdout.Close()
		_ = stderr.Close()

		b.ttyCancel()
		b.ttyCtx = nil
		b.ttyCancel = nil
	}()

	if err := c.Start(); err != nil {
		return b.toErrorStatus(err)
	}

	onStarted(ttyId)

	go b.writeTtyIn(stdin)
	go b.writeTtyOut(io.MultiReader(stdout, stderr))

	_ = c.Wait()
	return
}

func (b *shellExecutor) setupBin() []string {
	return []string{fmt.Sprintf("$Env:PATH += \";%s\"", b.binDir)}
}

func (b *shellExecutor) writeEnv() []string {
	tmpFile, err := ioutil.TempFile("", "agent_env_")
	util.PanicIfErr(err)

	defer tmpFile.Close()

	b.envFile = tmpFile.Name()
	return []string{"gci env: > " + tmpFile.Name()}
}

func (b *shellExecutor) writeScriptToTmpFile() string {
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

	b.writeCmd(file, b.setupBin, b.writeEnv, doScript)

	_ = file.Close()
	return file.Name()
}
