package executor

import (
	"context"
	"fmt"
	"github.com/creack/pty"
	"github/flowci/flow-agent-x/domain"
	"github/flowci/flow-agent-x/util"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

type (
	BashExecutor struct {
		BaseExecutor
		command *exec.Cmd
		tty     *exec.Cmd
		ttyWait sync.WaitGroup
		workDir string
		envFile string
	}
)

func (b *BashExecutor) Init() (out error) {
	if util.IsEmptyString(b.workspace) {
		b.workDir, out = ioutil.TempDir("", "agent_")
		b.vars[domain.VarAgentJobDir] = b.workDir
		return
	}

	b.workDir = filepath.Join(b.workspace, util.ParseString(b.inCmd.FlowId))
	b.vars[domain.VarAgentJobDir] = b.workDir
	out = os.MkdirAll(b.workDir, os.ModePerm)
	return
}

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
	b.startToHandleContext()

	// start command
	if err := command.Start(); err != nil {
		return b.toErrorStatus(err)
	}

	writeEnv := func(in chan string) {
		tmpFile, err := ioutil.TempFile("", "agent_env_")

		if err == nil {
			in <- "env > " + tmpFile.Name()
			b.envFile = tmpFile.Name()
		}
	}

	b.writeLog(stdout, true)
	b.writeLog(stderr, true)
	b.writeCmd(stdin, nil, writeEnv)
	b.toStartStatus(command.Process.Pid)

	// wait or timeout
	_ = command.Wait()
	util.LogDebug("[Done]: Shell for %s", b.inCmd.ID)

	b.exportEnv()

	// wait for tty if it's running
	if b.IsInteracting() {
		util.LogDebug("Tty is running, wait..")
		b.ttyWait.Wait()
	}

	if b.result.IsFinishStatus() {
		return nil
	}

	// to finish status
	b.toFinishStatus(getExitCode(command))
	return b.context.Err()
}

func (b *BashExecutor) StartTty(ttyId string, onStarted func(ttyId string)) (out error) {
	defer func() {
		if err := recover(); err != nil {
			out = err.(error)
		}

		b.tty = nil
		b.ttyId = ""
		b.ttyWait.Done()
	}()

	if b.IsInteracting() {
		panic(fmt.Errorf("interaction is ongoning"))
	}

	c := exec.Command(linuxBash)
	c.Dir = b.workDir
	c.Env = append(os.Environ(), b.vars.ToStringArray()...)

	b.tty = c
	b.ttyId = ttyId
	b.ttyWait.Add(1)

	ptmx, err := pty.Start(c)
	util.PanicIfErr(err)
	onStarted(ttyId)

	defer func() {
		_ = ptmx.Close()
	}()

	go b.writeTtyIn(ptmx)
	go b.writeTtyOut(ptmx)

	_ = c.Wait()
	return
}

func (b *BashExecutor) StopTty() {
	if b.tty != nil {
		_ = b.tty.Process.Kill()
	}
}

func (b *BashExecutor) IsInteracting() bool {
	return b.tty != nil
}

//====================================================================
//	private
//====================================================================

func (b *BashExecutor) exportEnv() {
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
