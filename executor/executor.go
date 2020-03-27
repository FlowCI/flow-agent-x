package executor

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github/flowci/flow-agent-x/config"
	"github/flowci/flow-agent-x/domain"
	"github/flowci/flow-agent-x/util"
	"path/filepath"
	"sync"
	"time"
)

const (
	linuxBash = "/bin/bash"
	//linuxBashShebang = "#!/bin/bash -i" // add -i enable to source .bashrc

	defaultLogChannelBufferSize = 10000
	defaultLogWaitingDuration   = 5 * time.Second
	defaultReaderBufferSize     = 8 * 1024

	Bash   = TypeOfExecutor(1)
	Docker = TypeOfExecutor(2)
)

type TypeOfExecutor int

type Executor interface {
	CmdID() string

	BashChannel() chan<- string

	LogChannel() <-chan *domain.LogItem

	Start() error

	Kill()

	GetResult() *domain.ExecutedCmd
}

type BaseExecutor struct {
	workspace   string // agent workspace
	workDir     string //
	context     context.Context
	cancelFunc  context.CancelFunc
	inCmd       *domain.CmdIn
	inVars      domain.Variables
	bashChannel chan string          // bash script comes from
	logChannel  chan *domain.LogItem // output log
	stdOutWg    sync.WaitGroup
	endTag      string
	CmdResult   *domain.ExecutedCmd
}

func NewExecutor(t TypeOfExecutor, parent context.Context, inCmd *domain.CmdIn, vars domain.Variables) Executor {
	app := config.GetInstance()

	base := BaseExecutor{
		workspace:   app.Workspace,
		workDir:     filepath.Join(app.Workspace, util.ParseString(inCmd.FlowId)),
		bashChannel: make(chan string),
		logChannel:  make(chan *domain.LogItem, defaultLogChannelBufferSize),
		inCmd:       inCmd,
		inVars:      vars,
		CmdResult:   domain.NewExecutedCmd(inCmd),
	}

	if vars == nil {
		base.inVars = make(domain.Variables)
	}
	base.inVars[domain.VarAgentJobDir] = base.workDir

	ctx, cancel := context.WithTimeout(parent, time.Duration(inCmd.Timeout)*time.Second)
	base.context = ctx
	base.cancelFunc = cancel

	endUUID, _ := uuid.NewRandom()
	base.endTag = fmt.Sprintf("=====EOF-%s=====", endUUID)
	base.stdOutWg.Add(2)

	switch t {
	case Bash:
		return &BashExecutor{
			BaseExecutor: base,
		}
	case Docker:
		return &DockerExecutor{
			BaseExecutor: base,
		}
	default:
		panic("Invalid executor type")
	}
}

// CmdID current bash executor cmd id
func (b *BaseExecutor) CmdID() string {
	return b.inCmd.ID
}

// BashChannel for input bash script
func (b *BaseExecutor) BashChannel() chan<- string {
	return b.bashChannel
}

// LogChannel for output log from stdout, stdin
func (b *BaseExecutor) LogChannel() <-chan *domain.LogItem {
	return b.logChannel
}

func (b *BaseExecutor) GetResult() *domain.ExecutedCmd {
	return b.CmdResult
}
