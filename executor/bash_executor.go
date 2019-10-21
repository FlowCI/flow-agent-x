package executor

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github/flowci/flow-agent-x/domain"
)

var (
	defaultLogChannelBufferSize = 10000
)

type (
	BashExecutor struct {
		context    context.Context
		cancelFunc context.CancelFunc
		endTag     string
		inCmd      *domain.CmdIn
		inVars     domain.Variables

		BashChannel chan string // bash script comes from
		LogChannel  chan string // output log
		CmdResult   *domain.ExecutedCmd
	}
)

// NewBashExecutor create new instance of bash executor
func NewBashExecutor(parent context.Context, inCmd *domain.CmdIn, vars domain.Variables) *BashExecutor {
	instance := &BashExecutor{
		BashChannel: make(chan string),
		LogChannel:  make(chan string, defaultLogChannelBufferSize),
		inCmd:       inCmd,
		inVars:      vars,
		CmdResult: 	 domain.NewExecutedCmd(inCmd),
	}

	endUUID, _ := uuid.NewRandom()
	instance.endTag = fmt.Sprintf("=====EOF-%s=====", endUUID)

	return instance
}

// Start run the cmd from domain.CmdIn
func (b *BashExecutor) Start() {

}

// Stop stop current running script
func (b *BashExecutor) Kill() {
	b.cancelFunc()
}
