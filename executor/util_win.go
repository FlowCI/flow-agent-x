// +build windows

package executor

import (
	"bufio"
	"encoding/binary"
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
	reader := bufio.NewReaderSize(r, 1024*8)
	output := domain.NewVariables()
	process := false

	utf16Dash := []byte{0, 45, 0, 45, 0, 45, 0, 45} // ----
	utf16EndWith := []byte{0, 13}

	for {
		line, _, err := reader.ReadLine()
		if err != nil {
			return output
		}

		if util.IsByteStartWith(line, utf16Dash) {
			process = true
			continue
		}

		if process {
			line = util.BytesTrimLeft(line, []byte{0})
			line = util.BytesTrimLeft(line, utf16EndWith)
			if len(line) == 0 {
				continue
			}

			strLine := util.UTF16BytesToString(line, binary.BigEndian)
			util.LogDebug(strLine)

			if ok, key, val := getEnvKeyAndVal(strLine); ok {
				if matchEnvFilter(key, filters) {
					output[key] = val
				}
			}
		}
	}
}

func getEnvKeyAndVal(line string) (ok bool, key, val string) {
	index := strings.IndexByte(line, ' ')
	if index == -1 {
		ok = false
		return
	}

	key = line[0:index]
	val = strings.Trim(line[index+1:], " ")
	ok = true
	return
}
