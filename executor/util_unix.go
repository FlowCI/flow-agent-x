package executor

import (
	"bufio"
	"github.com/flowci/flow-agent-x/domain"
	"github.com/flowci/flow-agent-x/util"
	"io"
	"strings"
)

func readEnvFromReaderForUnix(r io.Reader, filters []string) domain.Variables {
	reader := bufio.NewReader(r)
	output := domain.NewVariables()

	for {
		line, err := reader.ReadString(util.LineBreak)
		if err != nil {
			return output
		}

		line = strings.TrimSpace(line)
		if ok, key, val := getEnvKeyAndValForUnix(line); ok {
			if matchEnvFilter(key, filters) {
				output[key] = val
			}
		}
	}
}

func getEnvKeyAndValForUnix(line string) (ok bool, key, val string) {
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
