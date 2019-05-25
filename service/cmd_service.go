package service

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/streadway/amqp"

	"flow-agent-x/config"
	"flow-agent-x/domain"
	"flow-agent-x/executor"
	"flow-agent-x/util"
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

type CmdInteractSession map[string]*executor.ShellExecutor

// CmdService receive and execute cmd
type CmdService struct {
	executor *executor.ShellExecutor
	mux      sync.Mutex
	session  CmdInteractSession
}

// GetCmdService get singleton of cmd service
func GetCmdService() *CmdService {
	once.Do(func() {
		singleton = new(CmdService)
		singleton.session = make(CmdInteractSession, 10)
		singleton.start()
	})
	return singleton
}

// IsRunning check is available to run cmd
func (s *CmdService) IsRunning() bool {
	return s.executor != nil
}

// Execute execute cmd according to the type
func (s *CmdService) Execute(in *domain.CmdIn) error {
	if in.Type == domain.CmdTypeShell {
		return execShellCmd(s, in)
	}

	if in.Type == domain.CmdTypeKill {
		return execKillCmd(s, in)
	}

	if in.Type == domain.CmdTypeClose {
		return execCloseCmd(s, in)
	}

	if in.Type == domain.CmdTypeSessionOpen {
		return execSessionOpenCmd(s, in)
	}

	if in.Type == domain.CmdTypeSessionShell {
		return execSessionShellCmd(s, in)
	}

	if in.Type == domain.CmdTypeSessionClose {
		return execSessionCloseCmd(s, in)
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
		defer util.LogDebug("[Exit]: Rabbit mq consumer")

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
	util.LogDebug("[Exit]: cmd been executed and service is available !")
}

func execShellCmd(s *CmdService, in *domain.CmdIn) error {
	config := config.GetInstance()

	s.mux.Lock()
	defer s.mux.Unlock()

	if s.IsRunning() {
		return ErrorCmdIsRunning
	}

	if err := verifyAndInitShellCmd(in); util.HasError(err) {
		return err
	}

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
	s.executor.EnableRawLog = true

	go logConsumer(s.executor)

	go func() {
		defer s.release()
		s.executor.Run()

		result := s.executor.Result
		util.LogInfo("Cmd '%s' been executed with exit code %d", result.ID, result.Code)
		saveAndPushBack(result)
	}()

	return nil
}

func execKillCmd(s *CmdService, in *domain.CmdIn) error {
	if s.IsRunning() {
		return s.executor.Kill()
	}

	return nil
}

func execCloseCmd(s *CmdService, in *domain.CmdIn) error {
	if s.IsRunning() {
		return s.executor.Kill()
	}

	config := config.GetInstance()
	config.Quit <- true

	return nil
}

func execSessionOpenCmd(s *CmdService, in *domain.CmdIn) error {
	s.mux.Lock()
	defer s.mux.Unlock()

	if err := verifyAndInitOpenSessionCmd(in); util.HasError(err) {
		return err
	}

	shellExecutor := executor.NewShellExecutor(in)
	go logConsumer(shellExecutor)

	s.session[in.ID] = shellExecutor

	// start to run executor by thread
	go func() {
		shellExecutor.Run()
		delete(s.session, in.ID)
		util.LogDebug("agent: session '%s' is exited", in.ID)
	}()

	return nil
}

func execSessionShellCmd(s *CmdService, in *domain.CmdIn) error {
	exec, err := verifyAndGetExecutor(s, in)

	if !util.HasError(err) {
		return err
	}

	// ensure scripts array is not empty
	if len(in.Scripts) == 0 {
		return ErrorCmdSessionMissingScripts
	}

	script := in.Scripts[0]
	channel := exec.GetCmdChannel()
	channel <- script
	return nil
}

func execSessionCloseCmd(s *CmdService, in *domain.CmdIn) error {
	exec, err := verifyAndGetExecutor(s, in)

	if util.HasError(err) {
		return exec.Kill()
	}

	return nil
}

func verifyAndGetExecutor(s *CmdService, in *domain.CmdIn) (*executor.ShellExecutor, error) {
	if util.IsEmptyString(in.ID) {
		return nil, ErrorCmdMissingSessionID
	}

	exec := s.session[in.ID]

	if exec == nil {
		return nil, ErrorCmdSessionNotFound
	}

	return exec, nil
}

func verifyAndInitShellCmd(in *domain.CmdIn) error {
	if !in.HasScripts() {
		return ErrorCmdMissingScripts
	}

	// init cmd id if undefined
	if util.IsEmptyString(in.ID) {
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

func verifyAndInitOpenSessionCmd(in *domain.CmdIn) error {
	if in.HasScripts() {
		return ErrorCmdScriptIsPersented
	}

	in.ID = uuid.New().String()

	return nil
}

// Save result to local db and send back the result to server
func saveAndPushBack(r *domain.ExecutedCmd) {
	config := config.GetInstance()

	// TODO: save to local db

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
