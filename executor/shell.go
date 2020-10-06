package executor

import (
	"context"
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
		command *exec.Cmd
		tty     *exec.Cmd
		workDir string
		binDir  string
		envFile string
	}
)

func (b *shellExecutor) Init() (err error) {
	b.result.StartAt = time.Now()

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

	command, err := createCommand()
	util.PanicIfErr(err)

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

	doScript := func(script string) string {
		return script
	}

	b.writeLog(stdout, true, true)
	b.writeLog(stderr, true, true)
	b.writeCmd(stdin, b.setupBin, b.writeEnv, doScript)
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

func (b *shellExecutor) StopTty() {
	if b.IsInteracting() {
		_ = b.tty.Process.Kill()
	}
}

//====================================================================
//	private
//====================================================================

func (b *shellExecutor) exportEnv() {
	if util.IsEmptyString(b.envFile) {
		return
	}

	file, err := os.Open(b.envFile)
	if err != nil {
		return
	}

	defer file.Close()
	b.result.Output = readEnvFromReader(file, b.inCmd.EnvFilters)
}

func (b *shellExecutor) handleErrors(err error) {
	kill := func() {
		if b.command != nil {
			_ = b.command.Process.Kill()
		}

		if b.tty != nil {
			_ = b.tty.Process.Kill()
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
