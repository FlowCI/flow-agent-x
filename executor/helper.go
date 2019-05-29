package executor

import (
	"bufio"
	"flow-agent-x/domain"
	"flow-agent-x/util"
	"io"
	"os"
	"os/exec"
	"strings"
)

const (
	linuxBash = "/bin/bash"
	linuxBashShebang = "#!/bin/bash"
)

func createCommand(cmdIn *domain.CmdIn) (command *exec.Cmd, in io.WriteCloser, stdout io.ReadCloser, stderr io.ReadCloser) {
	command = exec.Command(linuxBash)
	command.Dir = cmdIn.WorkDir

	in, _ = command.StdinPipe()
	stdout, _ = command.StdoutPipe()
	stderr, _ = command.StderrPipe()

	return command, in, stdout, stderr
}

// Write script into file and make it executable
func writeScriptToFile(e *ShellExecutor) error {
	shellFile, _ := os.Create(e.Path.Shell)
	defer shellFile.Close()

	if !e.CmdIn.HasScripts() {
		return nil
	}

	_, _ = shellFile.WriteString(appendNewLine(linuxBashShebang))

	cmdIn := e.CmdIn
	endTerm := e.EndTerm

	// setup allow failure
	set := "set -e"
	if cmdIn.AllowFailure {
		set = "set +e"
	}
	_, _ = shellFile.WriteString(appendNewLine(set))

	// write scripts
	for _, script := range cmdIn.Scripts {
		_, err := shellFile.WriteString(appendNewLine(script))

		if util.HasError(err) {
			return err
		}
	}

	// write for end term
	if len(cmdIn.EnvFilters) > 0 {
		_, _ = shellFile.WriteString(appendNewLine("echo " + endTerm))
		_, _ = shellFile.WriteString(appendNewLine("env"))
	}

	err := shellFile.Chmod(0777)
	if util.HasError(err) {
		return err
	}

	return nil
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

func writeLogToFile(w *bufio.Writer, log string) {
	_, _ = w.WriteString(log)
	_ = w.WriteByte(util.UnixLineBreak)
}
