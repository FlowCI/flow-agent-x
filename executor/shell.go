package executor

import (
	"context"
	"github/flowci/flow-agent-x/domain"
	"github/flowci/flow-agent-x/util"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
	b.os = runtime.GOOS
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
