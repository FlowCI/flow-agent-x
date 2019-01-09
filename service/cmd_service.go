package service

import (
	"encoding/json"
	"sync"

	"github.com/streadway/amqp"

	"github.com/flowci/flow-agent-x/config"
	"github.com/flowci/flow-agent-x/domain"
	"github.com/flowci/flow-agent-x/executor"
	"github.com/flowci/flow-agent-x/util"
)

const (
	// VarAgentWorkspace default var
	VarAgentWorkspace = "FLOWCI_AGENT_WORKSPACE"

	// VarAgentPluginPath default var
	VarAgentPluginPath = "FLOWCI_AGENT_PLUGIN_PATH"
)

var (
	singleton *CmdService
	once      sync.Once
)

// CmdService receive and execute cmd
type CmdService struct {
	executor *executor.ShellExecutor
	mux      sync.Mutex
}

// GetCmdService get singleton of cmd service
func GetCmdService() *CmdService {
	once.Do(func() {
		singleton = new(CmdService)
	})
	return singleton
}

// IsAvailable check is available to run cmd
func (s *CmdService) IsAvailable() bool {
	return s.executor == nil
}

// Execute execute cmd accroding the type
func (s *CmdService) Execute(in *domain.CmdIn) error {
	if in.Type == domain.CmdTypeShell {
		s.mux.Lock()
		defer s.mux.Unlock()

		verifyCmdIn(in)

		if !s.IsAvailable() {
			util.LogInfo("Cannot start cmd since is running")
			return nil
		}

		config := config.GetInstance()

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

			result := s.executor.Result
			util.LogInfo("Cmd '%s' been executed with exit code %d", result.ID, result.Code)
			saveAndPushBack(result)
		}()

		return nil
	}

	if in.Type == domain.CmdTypeKill {
		return nil
	}

	if in.Type == domain.CmdTypeClose {
		return nil
	}

	return ErrorCmdUnsupportedType
}

func verifyCmdIn(in *domain.CmdIn) error {
	if in.Inputs == nil {
		in.Inputs = make(domain.Variables, 10)
	}

	if !in.HasScripts() {
		return ErrorCmdMissingScripts
	}

	return nil
}

// Save result to local database and push it back to server
func saveAndPushBack(r *domain.ExecutedCmd) {
	config := config.GetInstance()

	if config.HasQueue() {
		queue := config.Queue
		callbackQueue := queue.CallbackQueue

		json, _ := json.Marshal(r)

		msg := amqp.Publishing{
			ContentType: util.HttpMimeJson,
			Body:        json,
		}

		err := queue.Channel.Publish("", callbackQueue.Name, false, false, msg)
		util.LogIfError(err)
		util.LogDebug("Result of cmd %s been pushed", r.ID)
	}
}
