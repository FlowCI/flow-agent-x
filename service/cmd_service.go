package service

import (
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/streadway/amqp"

	"github/flowci/flow-agent-x/config"
	"github/flowci/flow-agent-x/domain"
	"github/flowci/flow-agent-x/executor"
	"github/flowci/flow-agent-x/util"
)

var (
	singleton *CmdService
	once      sync.Once
)

type (
	// CmdService receive and execute cmd
	CmdService struct {
		executor executor.Executor
		mux      sync.Mutex
	}
)

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

// Execute execute cmd according to the type
func (s *CmdService) Execute(bytes []byte) error {
	var in domain.CmdIn
	err := json.Unmarshal(bytes, &in)
	util.PanicIfErr(err)

	switch in.Type {
	case domain.CmdTypeShell:
		var shell domain.ShellCmd
		err := json.Unmarshal(bytes, &shell)
		util.PanicIfErr(err)
		return s.execShell(&shell)
	case domain.CmdTypeTtyOpen:
		return s.execTtyOpen()
	case domain.CmdTypeTtyOpen:
		// TODO:
		return nil
	case domain.CmdTypeTtyOpen:
		// TODO:
		return nil
	case domain.CmdTypeKill:
		return s.execKill()
	case domain.CmdTypeClose:
		return s.execClose()
	default:
		return ErrorCmdUnsupportedType
	}
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
				err = s.Execute(d.Body)
				if err != nil {
					util.LogDebug(err.Error())
				}
			case <-time.After(time.Second * 10):
				util.LogDebug("...")
			}
		}
	}()
}

func (s *CmdService) release() {
	s.executor = nil
	util.LogDebug("[Exit]: cmd been executed and service is available !")
}

func (s *CmdService) execShell(in *domain.ShellCmd) (out error) {
	defer func() {
		if err := recover(); err != nil {
			out = err.(error)
			s.failureBeforeExecute(in, out)
		}
	}()

	config := config.GetInstance()
	s.mux.Lock()
	defer s.mux.Unlock()

	if s.IsRunning() {
		return ErrorCmdIsRunning
	}

	err := verifyAndInitShellCmd(in)
	util.PanicIfErr(err)

	if in.HasPlugin() {
		plugins := util.NewPlugins(config.PluginDir, config.Server)
		err := plugins.Load(in.Plugin)
		util.PanicIfErr(err)
	}

	s.executor = executor.NewExecutor(executor.Options{
		AgentId:   config.Token,
		Parent:    config.AppCtx,
		Workspace: config.Workspace,
		PluginDir: config.PluginDir,
		Cmd:       in,
		Vars:      s.initEnv(),
		Volumes:   config.Volumes,
	})

	err = s.executor.Init()
	util.PanicIfErr(err)

	go logConsumer(s.executor, config.LoggingDir)

	go func() {
		defer s.release()
		_ = s.executor.Start()

		result := s.executor.GetResult()
		util.LogInfo("Cmd '%s' been executed with exit code %d", result.ID, result.Code)
		saveAndPushBack(result)
	}()

	return nil
}

func (s *CmdService) initEnv() domain.Variables {
	config := config.GetInstance()

	vars := domain.NewVariables()
	vars[domain.VarAgentPluginDir] = config.PluginDir
	vars[domain.VarServerUrl] = config.Server
	vars[domain.VarAgentToken] = config.Token
	vars[domain.VarAgentPort] = strconv.Itoa(config.Port)
	vars[domain.VarAgentWorkspace] = config.Workspace
	vars[domain.VarAgentPluginDir] = config.PluginDir
	vars[domain.VarAgentLogDir] = config.LoggingDir

	return vars
}

func (s *CmdService) execTtyOpen() (err error) {
	if !s.IsRunning() {
		return fmt.Errorf("No running shell script")
	}

	go ttyConsumer(s.executor)

	go func() {
		_ = s.executor.StartTty()
	}()
≠
	return
}

func (s *CmdService) execKill() error {
	if s.IsRunning() {
		s.executor.Kill()
	}
	return nil
}

func (s *CmdService) execClose() error {
	if s.IsRunning() {
		s.executor.Kill()
	}

	config := config.GetInstance()
	config.Cancel()

	return nil
}

func (s *CmdService) failureBeforeExecute(in *domain.ShellCmd, err error) {
	result := &domain.ShellOut{
		ID:      in.ID,
		Status:  domain.CmdStatusException,
		Error:   err.Error(),
		StartAt: time.Now(),
	}

	saveAndPushBack(result)
}

func verifyAndInitShellCmd(in *domain.ShellCmd) error {
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

	return nil
}

// Save result to local db and send back the result to server
func saveAndPushBack(r *domain.ShellOut) {
	config := config.GetInstance()

	// TODO: save to local db

	if !config.HasQueue() {
		return
	}

	queue := config.Queue
	json, _ := json.Marshal(r)
	callback := config.Settings.Queue.Callback

	err := queue.Channel.Publish("", callback, false, false, amqp.Publishing{
		ContentType: util.HttpMimeJson,
		Body:        json,
	})

	if !util.LogIfError(err) {
		util.LogDebug("Result of cmd %s been pushed", r.ID)
	}
}
