package executor

import (
	"bytes"
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

func appendNewLine(script string) string {
	if !strings.HasSuffix(script, util.NewLine) {
		script += util.NewLine
	}
	return script
}

func getEnvKeyAndVal(line string) (ok bool, key, val string) {
	index := strings.IndexAny(line, "=")
	if index == -1 {
		ok = false
		return
	}

	key = line[0:index]
	val = line[index+1:]
	ok = true
	return
}

func trimByte(in []byte) (out []byte) {
	out = bytes.TrimLeft(in, "\x00")
	out = bytes.TrimRight(out, util.NewLine)
	return
}