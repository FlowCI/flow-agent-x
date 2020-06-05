package executor

import (
	"context"
	"github.com/creack/pty"
	"github/flowci/flow-agent-x/domain"
	"github/flowci/flow-agent-x/util"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
)

type (
	BashExecutor struct {
		BaseExecutor
		command  *exec.Cmd
		interact *exec.Cmd
		workDir  string
		envFile  string
	}
)

func (b *BashExecutor) Init() (out error) {
	if util.IsEmptyString(b.workspace) {
		b.workDir, out = ioutil.TempDir("", "agent_")
		b.vars[domain.VarAgentJobDir] = b.workDir
		return
	}

	b.workDir = filepath.Join(b.workspace, util.ParseString(b.FlowId()))
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
	util.LogDebug("[Done]: Shell for %s", b.CmdId())

	b.exportEnv()

	if b.result.IsFinishStatus() {
		return nil
	}

	// to finish status
	b.toFinishStatus(getExitCode(command))
	return b.context.Err()
}

func (b *BashExecutor) StartInteract() (out error) {
	defer func() {
		if err := recover(); err != nil {
			out = err.(error)
		}
	}()

	c := exec.Command(linuxBash)
	c.Dir = b.workDir
	c.Env = append(os.Environ(), b.vars.ToStringArray()...)
	b.interact = c

	ptmx, err := pty.Start(c)
	util.PanicIfErr(err)

	defer func() {
		_ = ptmx.Close()
	}()

	startInput := func(writer io.Writer) {
		for {
			select {
			case <-b.context.Done():
				return
			case input := <-b.streamIn:
				_, _ = writer.Write([]byte(input))
			}
		}
	}

	startOutput := func(reader io.Reader) {
		buffer := make([]byte, defaultReaderBufferSize)
		for {
			select {
			case <-b.context.Done():
				return
			default:
				n, err := reader.Read(buffer)
				if err != nil {
					return
				}
				b.streamOut <- string(buffer[0:n])
			}
		}
	}

	go startInput(ptmx)
	go startOutput(ptmx)

	_ = c.Wait()
	b.interact = nil
	return
}

func (b *BashExecutor) IsInteracting() bool {
	return b.interact != nil
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

		if b.interact != nil {
			_ = b.interact.Process.Kill()
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
