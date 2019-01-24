package service

import (
	"encoding/json"
	"sync"
	"time"

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

const (
	defaultChannelBufferSize = 1000
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
		singleton.start()
	})
	return singleton
}

// IsRunning check is available to run cmd
func (s *CmdService) IsRunning() bool {
	return s.executor != nil
}

// Execute execute cmd accroding the type
func (s *CmdService) Execute(in *domain.CmdIn) error {
	config := config.GetInstance()

	if in.Type == domain.CmdTypeShell {
		s.mux.Lock()
		defer s.mux.Unlock()

		if s.IsRunning() {
			return ErrorCmdIsRunning
		}

		verifyAndInitCmdIn(in)

		// git clone required plugin
		if in.HasPlugin() && !config.IsOffline {
			plugins := util.NewPlugins(config.PluginDir, config.Server)
			err := plugins.Load(in.Plugin)

			if util.LogIfError(err) {
				result := &domain.ExecutedCmd{
					Status: domain.CmdStatusException,
					Error:  err.Error(),
				}

				saveAndPushBack(result)
				return nil
			}
		}

		// init and start executor
		s.executor = executor.NewShellExecutor(in)
		s.executor.LogChannel = make(chan *domain.LogItem, defaultChannelBufferSize)
		go logConsumer(in, s.executor.LogChannel)

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
		if s.IsRunning() {
			return s.executor.Kill()
		}

		return nil
	}

	if in.Type == domain.CmdTypeClose {
		if s.IsRunning() {
			return s.executor.Kill()
		}

		config.Quit <- true
		return nil
	}

	return ErrorCmdUnsupportedType
}

// new thread to consume rabbitmq message
func (s *CmdService) start() {
	config := config.GetInstance()

	if !config.HasQueue() {
		return
	}

	channel := config.Queue.Channel
	queue := config.Queue.JobQueue

	msgs, err := channel.Consume(queue.Name, "", true, false, false, false, nil)
	if util.HasError(err) {
		util.LogIfError(err)
		return
	}

	go func() {
		defer util.LogDebug("Exit: rabbitmq consumer")

		for {
			select {
			case d, ok := <-msgs:
				if !ok {
					break
				}

				util.LogDebug("Received a message: %s", d.Body)

				var cmdIn domain.CmdIn
				err := json.Unmarshal(d.Body, &cmdIn)

				if util.LogIfError(err) {
					continue
				}

				s.Execute(&cmdIn)

			case <-time.After(time.Second * 10):
				util.LogDebug("No more messages in queue. Timing out...")
			}
		}
	}()
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

	if util.IsEmptyString(in.WorkDir) {
		in.WorkDir = config.Workspace
	}

	in.WorkDir = util.ParseString(in.WorkDir)

	in.Inputs[VarAgentPluginPath] = config.PluginDir
	in.Inputs[VarAgentWorkspace] = config.Workspace

	return nil
}

// Save result to local database and push it back to server
func saveAndPushBack(r *domain.ExecutedCmd) {
	config := config.GetInstance()
	if !config.HasQueue() {
		return
	}

	queue := config.Queue
	json, _ := json.Marshal(r)
	callback := config.Settings.CallbackQueueName

	err := queue.Channel.Publish("", callback, false, false, amqp.Publishing{
		ContentType: util.HttpMimeJson,
		Body:        json,
	})

	if !util.LogIfError(err) {
		util.LogDebug("Result of cmd %s been pushed", r.ID)
	}
}
