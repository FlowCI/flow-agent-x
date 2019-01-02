package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/flowci/flow-agent-x/domain"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

const (
	linuxBash      = "/bin/bash"
	lineBreakLinux = "\n"
	logBufferSize  = 1000
)

type LogChannel chan *domain.LogItem

type ShellExecutor struct {
	CmdIn          *domain.CmdIn
	EndTerm        string
	Result         *domain.CmdResult
	TimeOutSeconds time.Duration
	LogChannel     LogChannel
}

// NewShellExecutor new instance of shell executor
func NewShellExecutor(cmdIn *domain.CmdIn) *ShellExecutor {
	result := &domain.CmdResult{
		Cmd: domain.Cmd{
			ID:           cmdIn.ID,
			AllowFailure: cmdIn.AllowFailure,
			Plugin:       cmdIn.Plugin,
		},
		Code:   domain.CmdExitCodeUnknow,
		Status: domain.CmdStatusPending,
	}

	uuid, _ := uuid.NewRandom()

	return &ShellExecutor{
		CmdIn:          cmdIn,
		EndTerm:        fmt.Sprintf("=====EOF-%s=====", uuid),
		Result:         result,
		LogChannel:     make(chan *domain.LogItem, logBufferSize),
		TimeOutSeconds: time.Duration(cmdIn.Timeout),
	}
}

// Run run shell scripts
func (e *ShellExecutor) Run() error {
	defer cleanUp(e)

	cmd := exec.Command(linuxBash)
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	// channel for stdout and stderr
	stdOutChannel := make(LogChannel, logBufferSize)
	stdErrChannel := make(LogChannel, logBufferSize)

	// channel for cmd wait
	done := make(chan error)

	if err := cmd.Start(); err != nil {
		return err
	}

	e.Result.ProcessId = cmd.Process.Pid
	e.Result.StartAt = time.Now()

	go pushToTotalChannel(e.CmdIn.ID, stdOutChannel, stdErrChannel, e.LogChannel)
	go handleStdIn(e.CmdIn.Scripts, stdin)
	go handleStdOut(stdout, stdOutChannel, domain.LogTypeOut)
	go handleStdOut(stderr, stdErrChannel, domain.LogTypeErr)
	go func() { done <- cmd.Wait() }()

	// wait for done
	select {
	case err := <-done:
		result := e.Result
		result.FinishAt = time.Now()

		// get wait status
		ws := cmd.ProcessState.Sys().(syscall.WaitStatus)

		// success status if no err
		if err == nil {
			result.Code = ws.ExitStatus()
			result.Status = domain.CmdStatusSuccess
			return nil
		}

		exitError, ok := err.(*exec.ExitError)
		result.Status = domain.CmdStatusException

		if ok {
			result.Code = ws.ExitStatus()
			return err
		}

		return exitError

	case <-time.After(e.TimeOutSeconds * time.Second):
		log.Info("Cmd killed since timeout")
		err := cmd.Process.Kill()

		result := e.Result
		result.Code = domain.CmdExitCodeTimeOut
		result.FinishAt = time.Now()
		result.Status = domain.CmdStatusKilled

		return err
	}
}

func cleanUp(e *ShellExecutor) {
	close(e.LogChannel)
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
