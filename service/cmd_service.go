package service

import (
	"sync"

	"github.com/flowci/flow-agent-x/config"
	"github.com/flowci/flow-agent-x/domain"
	"github.com/flowci/flow-agent-x/executor"
	log "github.com/sirupsen/logrus"
)

const (
	// VarAgentWorkspace default var
	VarAgentWorkspace = "FLOWCI_AGENT_WORKSPACE"

	// VarAgentPluginPath default var
	VarAgentPluginPath = "FLOWCI_AGENT_PLUGIN_PATH"
)

// CmdService receive and execute cmd
type CmdService struct {
	executor *executor.ShellExecutor
	mux      sync.Mutex
}

// Execute execute cmd accroding the type
func (s *CmdService) Execute(in *domain.CmdIn) error {
	if in.Type == domain.CmdTypeShell {
		s.mux.Lock()
		defer s.mux.Unlock()

		config := config.GetInstance()

		// check has running command
		if s.executor != nil {
			log.Info("Cannot start cmd since is running")
			return nil
		}

		// git clone required plugin
		if in.HasPlugin() {
			// TODO: git clone plugin from server
		}

		// start command via shell executor
		go func() {
			in.WorkDir = config.Workspace
			in.Inputs[VarAgentPluginPath] = config.PluginDir
			in.Inputs[VarAgentWorkspace] = config.Workspace

			s.executor = executor.NewShellExecutor(in)
			s.executor.Run()
		}()

		return nil
	}

	if in.Type == domain.CmdTypeKill {
		return nil
	}

	if in.Type == domain.CmdTypeClose {
		return nil
	}

	return nil
}
