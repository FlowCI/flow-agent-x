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

func newLineForOs(os string) string {
	if os == util.OSWin {
		return util.WinNewLine
	}

	return util.UnixNewLine
}

func appendNewLine(script, os string) string {
	newLine := newLineForOs(os)

	if !strings.HasSuffix(script, newLine) {
		script += newLine
	}

	return script
}
