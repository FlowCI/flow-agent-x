// +build !windows

package executor

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
