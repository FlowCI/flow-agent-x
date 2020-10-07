package executor

import (
	"bufio"
	"encoding/binary"
	"github/flowci/flow-agent-x/domain"
	"github/flowci/flow-agent-x/util"
	"io"
	"strings"
)

var (
	winUTF16Dash = []byte{0, 45, 0, 45, 0, 45, 0, 45} // ----
	winUTF16CR   = []byte{0, 13}                      // \r
	winNUL       = []byte{0}
)

func readEnvFromReaderForWin(r io.Reader, filters []string) domain.Variables {
	reader := bufio.NewReaderSize(r, 1024*8)
	output := domain.NewVariables()
	process := false

	for {
		line, _, err := reader.ReadLine()
		if err != nil {
			return output
		}

		if util.IsByteStartWith(line, winUTF16Dash) {
			process = true
			continue
		}

		if process {
			line = util.BytesTrimLeft(line, winNUL)
			line = util.BytesTrimLeft(line, winUTF16CR)
			if len(line) == 0 {
				continue
			}

			strLine := util.UTF16BytesToString(line, binary.BigEndian)

			if ok, key, val := getEnvKeyAndValForWin(strLine); ok {
				if matchEnvFilter(key, filters) {
					output[key] = val
				}
			}
		}
	}
}

func getEnvKeyAndValForWin(line string) (ok bool, key, val string) {
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
