package executor

import (
	"github/flowci/flow-agent-x/domain"
	"time"
)

const (
	linuxBash = "/bin/bash"
	//linuxBashShebang = "#!/bin/bash -i" // add -i enable to source .bashrc

	defaultLogChannelBufferSize = 10000
	defaultLogWaitingDuration   = 5 * time.Second
	defaultReaderBufferSize     = 8 * 1024
)

type Executor interface {
	BashChannel() chan<- string

	LogChannel() <-chan *domain.LogItem

	Start() error

	Kill()
}
