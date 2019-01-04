package executor

import "github.com/flowci/flow-agent-x/domain"

// ProcessListener interface for process status callback
type ProcessListener interface {
	OnStart(r *domain.CmdResult)
	OnExecuted(r *domain.CmdResult)
	OnException(err error)
}

// EmptyProcessListener default listener
type EmptyProcessListener struct {
}

func (listener *EmptyProcessListener) OnStart(r *domain.CmdResult) {
}

func (listener *EmptyProcessListener) OnExecuted(r *domain.CmdResult) {
}

func (listener *EmptyProcessListener) OnException(err error) {
}
