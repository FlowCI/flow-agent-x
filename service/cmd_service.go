package service

import (
	"encoding/json"
	"sync"

	"github.com/google/uuid"

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

		if !s.IsAvailable() {
			util.LogInfo("Cannot start cmd since is running")
			return nil
		}

		verifyAndInitCmdIn(in)

		// git clone required plugin
		if in.HasPlugin() {
			// TODO: git clone plugin from server
		}

		// init and start executor
		s.executor = executor.NewShellExecutor(in)
		s.executor.LogChannel = make(chan *domain.LogItem, 1000)
		go logPushConsumer(s.executor.LogChannel)

		go func() {
			defer s.release()

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

func (s *CmdService) release() {
	s.executor = nil
	util.LogDebug("Exit: cmd been executed and service is available !")
}

func verifyAndInitCmdIn(in *domain.CmdIn) error {
	if !in.HasScripts() {
		return ErrorCmdMissingScripts
	}

	// init cmd id if undefined
	if in.ID == "" {
		in.ID = uuid.New().String()
	}

	// init inputs if undefined
	if in.Inputs == nil {
		in.Inputs = make(domain.Variables, 10)
	}

	config := config.GetInstance()
	in.WorkDir = config.Workspace
	in.Inputs[VarAgentPluginPath] = config.PluginDir
	in.Inputs[VarAgentWorkspace] = config.Workspace

	return nil
}

// Push stdout, stderr log back to server
func logPushConsumer(channel executor.LogChannel) {
	defer util.LogDebug("Release: log push consumer")

	config := config.GetInstance()

	for {
		item, ok := <-channel
		if !ok {
			return
		}

		if !config.HasQueue() {
			util.LogDebug(item.String())
			continue
		}

		logsExchange := config.Settings.LogsExchangeName
		queue := config.Queue

		msg := amqp.Publishing{
			ContentType: util.HttpTextPlain,
			Body:        []byte(item.String()),
		}

		queue.Channel.Publish(logsExchange, "", false, false, msg)
	}
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
