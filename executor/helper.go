package executor

import (
	"flow-agent-x/config"
	"flow-agent-x/util"
	"fmt"
	"os"
	"path/filepath"
)

// Write script into file and make it executable
func writeScriptToFile(e *ShellExecutor) error {
	shellFile, _ := os.Create(e.Path.Shell)
	defer shellFile.Close()

	if !e.CmdIn.HasScripts() {
		return nil
	}

	cmdIn := e.CmdIn
	endTerm := e.EndTerm

	// setup allow failure
	set := "set -e"
	if cmdIn.AllowFailure {
		set = "set +e"
	}
	_, _ = shellFile.WriteString(appendNewLine(set))

	// write scripts
	for _, script := range cmdIn.Scripts {
		_, err := shellFile.WriteString(appendNewLine(script))

		if util.HasError(err) {
			return err
		}
	}

	// write for end term
	if len(cmdIn.EnvFilters) > 0 {
		_, _ = shellFile.WriteString(fmt.Sprintf("echo %s%s", endTerm, util.UnixLineBreakStr))
		_, _ = shellFile.WriteString(fmt.Sprintf("env%s", util.UnixLineBreakStr))
	}

	err := shellFile.Chmod(0777)
	if util.HasError(err) {
		return err
	}

	return nil
}

func getShellFilePath(cmdId string) string {
	c := config.GetInstance()
	return filepath.Join(c.LoggingDir, cmdId+".sh")
}

func getLogFilePath(cmdId string) string {
	c := config.GetInstance()
	return filepath.Join(c.LoggingDir, cmdId+".log")
}

func getRawLogFilePath(cmdId string) string {
	c := config.GetInstance()
	return filepath.Join(c.LoggingDir, cmdId+".raw.log")
}
