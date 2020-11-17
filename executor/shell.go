package executor

import (
	"context"
	"fmt"
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
		binDir  string
		envFile string
	}
)

func (se *shellExecutor) Init() (out error) {
	defer util.RecoverPanic(func(e error) {
		out = e
	})

	se.os = runtime.GOOS
	se.result.StartAt = time.Now()

	if util.IsEmptyString(se.workspace) {
		se.workspace, _ = ioutil.TempDir("", "agent_")
	}

	// setup bin under workspace
	se.binDir = filepath.Join(se.workspace, "bin")
	err := os.MkdirAll(se.binDir, os.ModePerm)
	util.PanicIfErr(err)

	for _, f := range binFiles {
		path := filepath.Join(se.binDir, f.name)
		if !util.IsFileExists(path) {
			_ = ioutil.WriteFile(path, f.content, f.permission)
		}
	}

	// setup job dir under workspace
	se.jobDir = filepath.Join(se.workspace, util.ParseString(se.inCmd.FlowId))
	se.vars[domain.VarAgentJobDir] = se.jobDir

	err = os.MkdirAll(se.jobDir, os.ModePerm)
	util.PanicIfErr(err)

	se.vars.Resolve()
	se.copyCache()
	return nil
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

	se.writeCache()
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

// copy cache to job dir if cache defined in cacheSrcDir
func (se *shellExecutor) copyCache() {
	if !util.HasString(se.cacheSrcDir) {
		return
	}

	files, err := ioutil.ReadDir(se.cacheSrcDir)
	util.PanicIfErr(err)

	for _, f := range files {
		oldPath := filepath.Join(se.cacheSrcDir, f.Name())
		newPath := filepath.Join(se.jobDir, f.Name())

		// move cache from src dir to job dir
		// error when file or dir exist
		err = os.Rename(oldPath, newPath)

		if err == nil {
			se.writeSingleLog(fmt.Sprintf("cache %s has been applied", f.Name()))
		} else {
			se.writeSingleLog(fmt.Sprintf("cache %s not applied since file or dir existed, ", f.Name()))
		}

		// remove cache from cache dir anyway
		_ = os.RemoveAll(oldPath)
	}
}

// write cache back to cacheSrcDir
func (se *shellExecutor) writeCache() {
	if !se.inCmd.HasCache() {
		return
	}

	defer util.RecoverPanic(func(e error) {
		util.LogWarn(e.Error())
	})

	cache := se.inCmd.Cache
	for _, path := range cache.Paths {
		path = filepath.Clean(path)
		fullPath := filepath.Join(se.jobDir, path)

		info, exist := util.IsFileExistsAndReturnFileInfo(fullPath)
		if !exist {
			continue
		}

		newPath := filepath.Join(se.cacheSrcDir, path)

		if info.IsDir() {
			err := util.CopyDir(fullPath, newPath)
			util.PanicIfErr(err)

			util.LogDebug("dir %s write back to cache dir", newPath)
			continue
		}

		err := util.CopyFile(fullPath, newPath)
		util.PanicIfErr(err)
		util.LogDebug("file %s write back to cache dir", newPath)
	}
}

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
