// +build !windows

package executor

import (
	"bufio"
	"github/flowci/flow-agent-x/domain"
	"github/flowci/flow-agent-x/util"
	"io"
	"strings"
)

func scriptForExitOnError() []string {
	return []string{"set -e"}
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
