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

const (
	dockerHeaderSize = 8
	dockerHeaderPrefixSize = 4 // [STREAM_TYPE, 0, 0 ,0, ....]
)

var (
	dockerStdInHeaderPrefix  = []byte{1, 0, 0, 0}
	dockerStdErrHeaderPrefix = []byte{2, 0, 0, 0}
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

func removeDockerHeader (in []byte) []byte {
	if len(in) < dockerHeaderSize {
		return in
	}

	if bytes.Compare(in[:dockerHeaderPrefixSize], dockerStdInHeaderPrefix) == 0 {
		return in[dockerHeaderSize:]
	}

	if bytes.Compare(in[:dockerHeaderPrefixSize], dockerStdErrHeaderPrefix) == 0 {
		return in[dockerHeaderSize:]
	}

	return in
}
