package executor

import (
	"github/flowci/flow-agent-x/util"
	"os/exec"
	"strings"
	"syscall"
)

func getExitCode(cmd *exec.Cmd) int {
	ws := cmd.ProcessState.Sys().(syscall.WaitStatus)
	return ws.ExitStatus()
}

func matchEnvFilter(env string, filters []string) bool {
	for _, filter := range filters {
		if strings.HasPrefix(env, filter) {
			return true
		}
	}
	return false
}

func appendNewLine(script string, inDocker bool) string {
	if inDocker {
		if !strings.HasSuffix(script, util.UnixNewLine) {
			script += util.UnixNewLine
		}
		return script
	}

	if !strings.HasSuffix(script, util.NewLine) {
		script += util.NewLine
	}
	return script
}