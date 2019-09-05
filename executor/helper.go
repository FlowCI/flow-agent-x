package executor

import (
	"flow-agent-x/domain"
	"flow-agent-x/util"
	"io"
	"os/exec"
	"strings"
)

const (
	linuxBash        = "/bin/bash"
	linuxBashShebang = "#!/bin/bash -i" // add -i enable to source .bashrc
)

// CmdInstance: command with stdin, stdout and stderr
type CmdInstance struct {
	command *exec.Cmd
	stdIn   io.WriteCloser
	stdOut  io.ReadCloser
	stdErr  io.ReadCloser
}

func createCommand(cmdIn *domain.CmdIn) *CmdInstance {
	command := exec.Command(linuxBash)
	command.Dir = cmdIn.WorkDir

	stdin, _ := command.StdinPipe()
	stdout, _ := command.StdoutPipe()
	stderr, _ := command.StderrPipe()

	return &CmdInstance{
		command: command,
		stdIn:   stdin,
		stdOut:  stdout,
		stdErr:  stderr,
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
	if !strings.HasSuffix(script, util.UnixLineBreak) {
		script += util.UnixLineBreak
	}
	return script
}
