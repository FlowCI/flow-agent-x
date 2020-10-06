// +build windows

package executor

import (
	"fmt"
	"github/flowci/flow-agent-x/util"
	"io/ioutil"
)

func (b *shellExecutor) StartTty(ttyId string, onStarted func(ttyId string)) (out error) {
	return nil
}

func (b *shellExecutor) setupBin(in chan string) {
	in <- fmt.Sprintf("$Env:PATH += \";%s\"", b.binDir)
}

func (b *shellExecutor) writeEnv(in chan string) {
	tmpFile, err := ioutil.TempFile("", "agent_env_")
	util.PanicIfErr(err)

	defer tmpFile.Close()

	in <- "gci env: > " + tmpFile.Name()
	b.envFile = tmpFile.Name()
}
