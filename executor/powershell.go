// +build windows

package executor

import (
	"fmt"
	"github/flowci/flow-agent-x/domain"
	"github/flowci/flow-agent-x/util"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type (
	shellExecutor struct {
		BaseExecutor

		powerShellPath string
		command        *exec.Cmd

		workDir string
		binDir  string
		envFile string
	}
)

func (b *shellExecutor) Init() (err error) {
	b.result.StartAt = time.Now()

	b.powerShellPath, err = exec.LookPath(winPowerShell)
	if err != nil {
		return
	}

	if util.IsEmptyString(b.workspace) {
		b.workDir, err = ioutil.TempDir("", "agent_")
		b.vars[domain.VarAgentJobDir] = b.workDir
		return
	}

	// setup bin under workspace
	b.binDir = filepath.Join(b.workspace, "bin")
	err = os.MkdirAll(b.binDir, os.ModePerm)
	for _, f := range binFiles {
		path := filepath.Join(b.binDir, f.name)
		if !util.IsFileExists(path) {
			_ = ioutil.WriteFile(path, f.content, f.permission)
		}
	}

	// setup job dir under workspace
	b.workDir = filepath.Join(b.workspace, util.ParseString(b.inCmd.FlowId))
	b.vars[domain.VarAgentJobDir] = b.workDir
	err = os.MkdirAll(b.workDir, os.ModePerm)

	b.vars.Resolve()
	return
}

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

	command := exec.Command(b.powerShellPath, []string{"-NoProfile", "-NonInteractive"}...)
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

	setupBin := func(in chan string) {
		in <- fmt.Sprintf("$Env:PATH += ;%s", b.binDir)
	}

	writeEnv := func(in chan string) {
		tmpFile, err := ioutil.TempFile("", "agent_env_")

		if err == nil {
			in <- "gci env: > " + tmpFile.Name()
			b.envFile = tmpFile.Name()
		}
	}

	doScript := func(script string) string {
		return script
	}

	b.writeLog(stdout, true, true)
	b.writeLog(stderr, true, true)
	b.writeCmd(stdin, setupBin, writeEnv, doScript)
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
	return nil
}

func (b *shellExecutor) StopTty() {

}

func (b *shellExecutor) handleErrors(err error) {

}

func (b *shellExecutor) exportEnv() {

}