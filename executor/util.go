package executor

import (
	"bufio"
	"github/flowci/flow-agent-x/util"
	"strings"
)

func readLine(r *bufio.Reader, builder strings.Builder) (string, error) {
	var prefix bool

	line, prefix, err := r.ReadLine()
	builder.Write(line)

	if err != nil {
		return builder.String(), err
	}

	for prefix {
		var rest []byte
		rest, prefix, err = r.ReadLine()
		builder.Write(rest)

		if err != nil {
			return builder.String(), err
		}
	}

	return builder.String(), nil
}

func appendNewLine(script string) string {
	if !strings.HasSuffix(script, util.UnixLineBreak) {
		script += util.UnixLineBreak
	}
	return script
}