package executor

import (
	"bufio"
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