package service

import (
	"encoding/json"
	"path/filepath"
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
	CmdInteractSession map[string]*executor.BashExecutor

	// CmdService receive and execute cmd
	CmdService struct {
		executor *executor.BashExecutor
		mux      sync.Mutex
		session  CmdInteractSession
	}
)

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

				err = s.Execute(&cmdIn)
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
				Cmd: domain.Cmd{
					ID: in.ID,
				},
				Status:  domain.CmdStatusException,
				Error:   err.Error(),
				StartAt: time.Now(),
			}

			saveAndPushBack(result)
			return nil
		}
	}

	// init and start executor
	vars := config.Vars.Copy()
	vars[domain.VarAgentJobDir] = in.WorkDir
	s.executor = executor.NewBashExecutor(config.AppCtx, in, vars)

	go logConsumer(s.executor, config.LoggingDir)

	go func() {
		defer s.release()
		_ = s.executor.Start()

		result := s.executor.CmdResult
		util.LogInfo("Cmd '%s' been executed with exit code %d", result.ID, result.Code)
		saveAndPushBack(result)
	}()

	return nil
}

func execKillCmd(s *CmdService, in *domain.CmdIn) error {
	if s.IsRunning() {
		s.executor.Kill()
	}

	return nil
}

func execCloseCmd(s *CmdService, in *domain.CmdIn) error {
	if s.IsRunning() {
		s.executor.Kill()
	}

	config := config.GetInstance()
	config.Cancel()

	return nil
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
	in.WorkDir = filepath.Join(config.Workspace, util.ParseString(in.WorkDir))
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
	callback := config.Settings.Queue.Callback

	err := queue.Channel.Publish("", callback, false, false, amqp.Publishing{
		ContentType: util.HttpMimeJson,
		Body:        json,
	})

	if !util.LogIfError(err) {
		util.LogDebug("Result of cmd %s been pushed", r.ID)
	}
}
