package executor

import (
	"bufio"
	"bytes"
	"github/flowci/flow-agent-x/domain"
	"github/flowci/flow-agent-x/util"
	"io"
	"os/exec"
	"strings"
	"syscall"
)

func getExitCode(cmd *exec.Cmd) int {
	ws := cmd.ProcessState.Sys().(syscall.WaitStatus)
	return ws.ExitStatus()
}

func readEnvFromReader(r io.Reader, filters []string) domain.Variables {
	reader := bufio.NewReader(r)
	output := domain.NewVariables()

	for {
		line, err := reader.ReadString(util.UnixLineBreak)
		if err != nil {
			return output
		}

		line = strings.TrimSpace(line)
		if ok, key, val := getEnvKeyAndVal(line); ok {
			if matchEnvFilter(key, filters) {
				output[key] = val
			}
		}
	}
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
	if !strings.HasSuffix(script, util.UnixLineBreakStr) {
		script += util.UnixLineBreakStr
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
	out = bytes.TrimRight(out, util.UnixLineBreakStr)
	return
}