package executor

import (
	"context"
	"github/flowci/flow-agent-x/domain"
	"github/flowci/flow-agent-x/util"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
)

type (
	BashExecutor struct {
		BaseExecutor
		command *exec.Cmd
		workDir string
		envFile string
	}
)

func (b *BashExecutor) Init() (out error) {
	cmd := b.inCmd

	if util.IsEmptyString(b.workspace) {
		b.workDir, out = ioutil.TempDir("", "agent_")
		b.vars[domain.VarAgentJobDir] = b.workDir
		return
	}

	b.workDir = filepath.Join(b.workspace, util.ParseString(cmd.FlowId))
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

	stdin, _ := command.StdinPipe()
	stdout, _ := command.StdoutPipe()
	stderr, _ := command.StderrPipe()

	b.command = command
	b.startToHandleContext()

	// start command
	if err := command.Start(); err != nil {
		return b.toErrorStatus(err)
	}

	b.writeLog(stdout)
	b.writeLog(stderr)
	b.writeCmd(stdin, func(in chan string) {
		tmpFile, err := ioutil.TempFile("", "agent_env_")

		if err == nil {
			in <- "env > " + tmpFile.Name()
			b.envFile = tmpFile.Name()
		}
	})

	b.toStartStatus(command.Process.Pid)

	// wait or timeout
	_ = command.Wait()
	util.LogDebug("[Done]: Shell for %s", b.CmdId())

	b.exportEnv()

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

func (b *BashExecutor) exportEnv() {
	if util.IsEmptyString(b.envFile) {
		return
	}

	file, err := os.Open(b.envFile)
	if err != nil {
		return
	}

	defer file.Close()
	b.CmdResult.Output = readEnvFromReader(file, b.inCmd.EnvFilters)
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
