package service

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"bufio"
	"encoding/base64"
	"github.com/google/uuid"
	"os"
	"path/filepath"

	"github.com/flowci/flow-agent-x/config"
	"github.com/flowci/flow-agent-x/domain"
	"github.com/flowci/flow-agent-x/executor"
	"github.com/flowci/flow-agent-x/util"
)

type (
	// CmdService receive and execute cmd
	CmdService struct {
		pluginManager *PluginManager
		cacheManager  *CacheManager

		cmdIn <-chan []byte

		executor executor.Executor
		mux      sync.Mutex
	}
)

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
		var shell domain.ShellIn
		err := json.Unmarshal(bytes, &shell)
		util.PanicIfErr(err)
		return s.execShell(&shell)
	case domain.CmdTypeTty:
		s.execTty(bytes)
		return nil
	case domain.CmdTypeKill:
		return s.execKill()
	case domain.CmdTypeClose:
		return s.execClose()
	default:
		return ErrorCmdUnsupportedType
	}
}

func (s *CmdService) start() {
	go func() {
		defer util.LogDebug("[Exit]: Rabbit mq consumer")

		for {
			select {
			case bytes, ok := <-s.cmdIn:
				if !ok {
					break
				}

				util.LogDebug("Received a message: %s", bytes)
				err := s.Execute(bytes)
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
	if s.executor != nil {
		s.executor.Close()
		s.executor = nil

		cm := config.GetInstance()
		cm.FireEvent(domain.EventOnIdle)
	}

	util.LogDebug("[Exit]: cmd been executed and service is available !")
}

func (s *CmdService) execShell(in *domain.ShellIn) (out error) {
	defer func() {
		if err := recover(); err != nil {
			out = err.(error)
			s.failureBeforeExecute(in, out)
			s.release() // release current executor if error
		}
	}()

	cm := config.GetInstance()

	s.mux.Lock()
	defer s.mux.Unlock()

	if s.IsRunning() {
		return ErrorCmdIsRunning
	}

	err := initShellCmd(in)
	util.PanicIfErr(err)

	cm.FireEvent(domain.EventOnBusy)

	if in.HasPlugin() {
		err := s.pluginManager.Load(in.Plugin)
		util.PanicIfErr(err)
	}

	// all cache will move to job dir after started
	cacheSrcDir := ""
	if in.HasCache() {
		cacheSrcDir = s.cacheManager.Download(in)
	}

	s.loadSecretForDocker(in)

	s.executor = executor.NewExecutor(executor.Options{
		K8s: &domain.K8sConfig{
			Enabled:   cm.K8sEnabled,
			InCluster: cm.K8sCluster,
			Namespace: cm.K8sNamespace,
			PodName:   cm.K8sPodName,
			PodIp:     cm.K8sPodIp,
		},
		AgentId:                   cm.Token,
		Parent:                    cm.AppCtx,
		Workspace:                 cm.Workspace,
		WorkspaceFromDockerVolume: cm.IsFromDocker,
		PluginDir:                 cm.PluginDir,
		CacheSrcDir:               cacheSrcDir,
		Cmd:                       in,
		Vars:                      s.initEnv(),
		SecretVars:                s.initSecretEnv(in),
		ConfigVars:                s.initConfigEnv(in),
		Volumes:                   cm.Volumes,
	})

	err = s.executor.Init()
	util.PanicIfErr(err)

	s.startLogConsumer()

	go func() {
		defer func() {
			input, output := s.executor.CacheDir()
			os.RemoveAll(input)
			os.RemoveAll(output)

			s.release()
		}()

		_ = s.executor.Start()

		// write all files in srcCache back to cache
		_, output := s.executor.CacheDir()
		s.cacheManager.Upload(in, output)

		result := s.executor.GetResult()
		util.LogInfo("Cmd '%s' been executed with exit code %d", result.ID, result.Code)
		cm.Client.SendCmdOut(result)
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

	// write env for interface ip on of agent host
	interfaces, err := net.Interfaces()
	if err != nil {
		return vars
	}

	for _, iface := range interfaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			key := fmt.Sprintf(domain.VarAgentIpPattern, iface.Name)
			vars[key] = ip.String()
			break
		}
	}

	return vars
}

// initSecretEnv load secret value as environment variables
func (s *CmdService) initSecretEnv(in *domain.ShellIn) domain.Variables {
	vars := domain.NewVariables()

	if !in.HasSecrets() {
		return vars
	}

	api := config.GetInstance().Client
	for _, name := range in.Secrets {
		secret, err := api.GetSecret(name)
		util.PanicIfErr(err)
		vars.AddMapVars(secret.ToEnvs())
	}

	return vars
}

// initConfigEnv load config value as environment variables
func (s *CmdService) initConfigEnv(in *domain.ShellIn) domain.Variables {
	vars := domain.NewVariables()

	if !in.HasConfigs() {
		return vars
	}

	api := config.GetInstance().Client
	for _, name := range in.Configs {
		config, err := api.GetConfig(name)
		util.PanicIfErr(err)
		vars.AddMapVars(config.ToEnvs())
	}

	return vars
}

func (s *CmdService) execTty(bytes []byte) {
	var in domain.TtyIn
	err := json.Unmarshal(bytes, &in)
	if err != nil {
		util.LogWarn("Unable to decode message to TtyIn")
		return
	}

	response := &domain.TtyOut{
		ID:     in.ID,
		Action: in.Action,
	}

	appConfig := config.GetInstance()

	defer func() {
		if err := recover(); err != nil {
			response.IsSuccess = false
			response.Error = err.(error).Error()
			appConfig.Client.SendCmdOut(response)
		}
	}()

	e := s.executor
	if !s.IsRunning() {
		panic(fmt.Errorf("No running cmd"))
	}

	switch in.Action {
	case domain.TtyActionOpen:
		if e.IsInteracting() {
			response.IsSuccess = true
			appConfig.Client.SendCmdOut(response)
			return
		}

		go func() {
			err = e.StartTty(in.ID, func(ttyId string) {
				response.IsSuccess = true
				appConfig.Client.SendCmdOut(response)
			})

			if err != nil {
				response.IsSuccess = false
				response.Error = err.Error()
				appConfig.Client.SendCmdOut(response)
				return
			}

			// send close action when exit
			response.Action = domain.TtyActionClose
			response.IsSuccess = true
			appConfig.Client.SendCmdOut(response)
		}()
	case domain.TtyActionShell:
		if !e.IsInteracting() {
			panic(fmt.Errorf("Tty not started, please send open cmd"))
		}

		e.TtyIn() <- in.Input
	case domain.TtyActionClose:
		if !e.IsInteracting() {
			panic(fmt.Errorf("Tty not started, please send open cmd"))
		}

		// close action response send on exit
		e.StopTty()
	}
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

func (s *CmdService) failureBeforeExecute(in *domain.ShellIn, err error) {
	result := &domain.ShellOut{
		ID:       in.ID,
		Status:   domain.CmdStatusException,
		Error:    err.Error(),
		StartAt:  time.Now(),
		FinishAt: time.Now(),
	}

	appConfig := config.GetInstance()
	appConfig.Client.SendCmdOut(result)
}

func (s *CmdService) startLogConsumer() {
	apiClient := config.GetInstance().Client
	loggingDir := config.GetInstance().LoggingDir
	executor := s.executor

	consumeShellLog := func() {

		// init path for shell, log and raw log
		logPath := filepath.Join(loggingDir, executor.CmdIn().ID+".log")
		f, _ := os.Create(logPath)
		logFileWriter := bufio.NewWriter(f)

		defer func() {
			// upload log after flush!!
			_ = logFileWriter.Flush()
			_ = f.Close()

			err := apiClient.UploadLog(logPath)
			util.LogIfError(err)
			util.LogDebug("[Exit]: LogConsumer")
		}()

		for b64Log := range executor.Stdout() {

			// write to file
			log, err := base64.StdEncoding.DecodeString(b64Log)
			if err == nil {
				_, _ = logFileWriter.Write(log)
				util.LogDebug("[ShellLog]: %s", string(log))
			}

			jobId := executor.CmdIn().JobId
			stepId := executor.CmdIn().ID
			apiClient.SendShellLog(jobId, stepId, b64Log)
		}
	}

	consumeTtyLog := func() {
		apiClient := config.GetInstance().Client
		for b64Log := range executor.TtyOut() {
			apiClient.SendTtyLog(executor.TtyId(), b64Log)
		}
	}

	go consumeShellLog()
	go consumeTtyLog()
}

func (s *CmdService) loadSecretForDocker(in *domain.ShellIn) {
	if !in.HasDockerOption() {
		return
	}

	cm := config.GetInstance()
	for _, option := range in.Dockers {
		if option.HasAuth() {
			secret, err := cm.Client.GetSecret(option.Auth)
			util.PanicIfErr(err)

			auth, ok := secret.(*domain.AuthSecret)
			if !ok {
				panic(fmt.Errorf("the secret '%s' is invalid, the secret category should be 'Auth pair'", option.Auth))
			}

			option.AuthContent = auth.Pair
		}
	}
}

// ---------------------------------
// 	Utils
// ---------------------------------

func initShellCmd(in *domain.ShellIn) error {
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
