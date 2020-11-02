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

func (se *shellExecutor) Init() (err error) {
	se.os = runtime.GOOS
	se.result.StartAt = time.Now()

	if util.IsEmptyString(se.workspace) {
		se.workspace, _ = ioutil.TempDir("", "agent_")
	}

	// setup bin under workspace
	se.binDir = filepath.Join(se.workspace, "bin")
	err = os.MkdirAll(se.binDir, os.ModePerm)
	for _, f := range binFiles {
		path := filepath.Join(se.binDir, f.name)
		if !util.IsFileExists(path) {
			_ = ioutil.WriteFile(path, f.content, f.permission)
		}
	}

	// setup job dir under workspace
	se.workDir = filepath.Join(se.workspace, util.ParseString(se.inCmd.FlowId))
	se.vars[domain.VarAgentJobDir] = se.workDir
	err = os.MkdirAll(se.workDir, os.ModePerm)

	se.vars.Resolve()
	return
}

func (se *shellExecutor) Start() (out error) {
	// handle context error
	go func() {
		<-se.context.Done()
		err := se.context.Err()

		if err != nil {
			se.handleErrors(err)
		}
	}()

	for i := se.inCmd.Retry; i >= 0; i-- {
		out = se.doStart()
		r := se.result

		if r.Status == domain.CmdStatusException || out != nil {
			if i > 0 {
				se.writeSingleLog(">>>>>>> retry >>>>>>>")
			}
			continue
		}

		break
	}
	return
}

func (se *shellExecutor) StopTty() {
	if se.IsInteracting() {
		_ = se.tty.Process.Kill()
	}
}

//====================================================================
//	private
//====================================================================

func (se *shellExecutor) exportEnv() {
	if util.IsEmptyString(se.envFile) {
		return
	}

	file, err := os.Open(se.envFile)
	if err != nil {
		return
	}

	defer file.Close()
	se.result.Output = readEnvFromReader(se.os, file, se.inCmd.EnvFilters)
}

func (se *shellExecutor) handleErrors(err error) {
	kill := func() {
		if se.command != nil {
			_ = se.command.Process.Kill()
		}

		if se.tty != nil {
			_ = se.tty.Process.Kill()
		}
	}

	util.LogWarn("handleError on shell: %s", err.Error())

	if err == context.DeadlineExceeded {
		util.LogDebug("Timeout..")
		kill()
		se.toTimeOutStatus()
		return
	}

	if err == context.Canceled {
		util.LogDebug("Cancel..")
		kill()
		se.toKilledStatus()
		return
	}

	_ = se.toErrorStatus(err)
}
