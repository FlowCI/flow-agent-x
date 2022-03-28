package executor

import (
	"github.com/flowci/flow-agent-x/domain"
	"github.com/flowci/flow-agent-x/util"
	"io"
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

func scriptForExitOnError(os string) []string {
	if os == util.OSWin {
		return []string{"$ErrorActionPreference = \"Stop\""}
	}

	return []string{"set -e"}
}

func appendNewLine(script, os string) string {
	newLine := newLineForOs(os)

	if !strings.HasSuffix(script, newLine) {
		script += newLine
	}

	return script
}

func newLineForOs(os string) string {
	if os == util.OSWin {
		return util.WinNewLine
	}

	return util.UnixNewLine
}

func readEnvFromReader(os string, r io.Reader, filters []string) domain.Variables {
	if os == util.OSWin {
		return readEnvFromReaderForWin(r, filters)
	}

	return readEnvFromReaderForUnix(r, filters)
}
