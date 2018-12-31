package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync/atomic"

	"github.com/flowci/flow-agent-x/domain"
	"github.com/google/uuid"
)

const (
	linuxBash      = "/bin/bash"
	lineBreakLinux = "\n"
	logBufferSize  = 1000
)

type LogChannel chan *domain.LogItem

type ShellExecutor struct {
	CmdIn      *domain.CmdIn
	EndTerm    string
	Result     *domain.CmdResult
	LogChannel LogChannel
}

// NewShellExecutor new instance of shell executor
func NewShellExecutor(cmdIn *domain.CmdIn) *ShellExecutor {
	result := &domain.CmdResult{}
	result.ID = cmdIn.ID
	result.AllowFailure = cmdIn.AllowFailure
	result.Plugin = cmdIn.Plugin

	uuid, _ := uuid.NewRandom()

	return &ShellExecutor{
		CmdIn:      cmdIn,
		EndTerm:    fmt.Sprintf("=====EOF-%s=====", uuid),
		Result:     result,
		LogChannel: make(chan *domain.LogItem, logBufferSize),
	}
}

// Close to release resource of executor
func (e *ShellExecutor) Close() {
	close(e.LogChannel)
}

// Run run shell scripts
func (e *ShellExecutor) Run() error {
	cmd := exec.Command(linuxBash)
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	// channel for stdout and stderr
	stdOutChannel := make(LogChannel, logBufferSize)
	stdErrChannel := make(LogChannel, logBufferSize)

	err := cmd.Start()

	go handleStdIn(e.CmdIn.Scripts, stdin)
	go handleStdOut(stdout, stdOutChannel, domain.LogTypeOut)
	go handleStdOut(stderr, stdErrChannel, domain.LogTypeErr)
	go pushToTotalChannel(e.CmdIn.ID, stdOutChannel, stdErrChannel, e.LogChannel)

	err = cmd.Wait()
	return err
}

func handleStdIn(scripts []string, stdin io.WriteCloser) {
	defer stdin.Close()

	for _, script := range scripts {
		if !strings.HasSuffix(script, lineBreakLinux) {
			script += lineBreakLinux
		}
		io.WriteString(stdin, script)
	}
}

func handleStdOut(reader io.ReadCloser, channel LogChannel, t domain.LogType) {
	defer reader.Close()
	defer close(channel)

	scanner := bufio.NewScanner(reader)

	for scanner.Scan() {
		line := scanner.Text()
		channel <- &domain.LogItem{
			Type:    t,
			Content: line,
		}
	}
}

func pushToTotalChannel(cmdID string, out LogChannel, err LogChannel, total LogChannel) {
	var counter uint32
	var numOfLine int64

	for counter < 2 {
		select {
		case outLog, ok := <-out:
			if !ok {
				atomic.AddUint32(&counter, 1)
				continue
			}

			outLog.CmdID = cmdID
			outLog.Number = atomic.AddInt64(&numOfLine, 1)
			total <- outLog

		case errLog, ok := <-err:
			if !ok {
				atomic.AddUint32(&counter, 1)
				continue
			}

			errLog.CmdID = cmdID
			errLog.Number = atomic.AddInt64(&numOfLine, 1)
			total <- errLog
		}
	}
}
