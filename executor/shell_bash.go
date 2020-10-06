// +build !windows

package executor

import (
	"context"
	"fmt"
	"github.com/creack/pty"
	"github/flowci/flow-agent-x/util"
	"os"
	"os/exec"
)

func (b *shellExecutor) StartTty(ttyId string, onStarted func(ttyId string)) (out error) {
	defer func() {
		if err := recover(); err != nil {
			out = err.(error)
		}

		b.tty = nil
		b.ttyId = ""
	}()

	if b.IsInteracting() {
		panic(fmt.Errorf("interaction is ongoning"))
	}

	c := exec.Command(linuxBash)
	c.Dir = b.workDir
	c.Env = append(os.Environ(), b.vars.ToStringArray()...)

	ptmx, err := pty.Start(c)
	util.PanicIfErr(err)

	b.tty = c
	b.ttyId = ttyId
	b.ttyCtx, b.ttyCancel = context.WithCancel(b.context)

	defer func() {
		_ = ptmx.Close()
		b.ttyCancel()
		b.ttyCtx = nil
		b.ttyCancel = nil
	}()

	onStarted(ttyId)

	go b.writeTtyIn(ptmx)
	go b.writeTtyOut(ptmx)

	_ = c.Wait()
	return
}

func (b *shellExecutor) setupBin(in chan string) {
	in <- fmt.Sprintf("export PATH=%s:$PATH", b.binDir)
}

func (b *shellExecutor) writeEnv(in chan string) {
	tmpFile, err := ioutil.TempFile("", "agent_env_")
	util.PanicIfErr(err)

	defer tmpFile.Close()

	in <- "env > " + tmpFile.Name()
	b.envFile = tmpFile.Name()

}
