// +build windows

package executor

import (
	"bufio"
	"github/flowci/flow-agent-x/domain"
	"github/flowci/flow-agent-x/util"
	"io"
	"os/exec"
	"strings"
)

func createCommand() (*exec.Cmd, error) {
	path, err := exec.LookPath(winPowerShell)

	if err != nil {
		return nil, err
	}

	return exec.Command(path, []string{"-NoProfile", "-NonInteractive"}...), nil
}

func scriptForExitOnError() []string {
	return []string{"Set-StrictMode -Version Latest", "$ErrorActionPreference = \"Stop\""}
}

func readEnvFromReader(r io.Reader, filters []string) domain.Variables {
	reader := bufio.NewReader(r)
	output := domain.NewVariables()

	for {
		line, err := reader.ReadString(util.LineBreak)
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
