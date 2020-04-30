package config

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/mem"
	"github.com/streadway/amqp"
	"github/flowci/flow-agent-x/domain"
	"github/flowci/flow-agent-x/util"
	"io/ioutil"
	"net/http"
	"os"
	"sync"
	"time"
)

var (
	singleton *Manager
	once      sync.Once
)

type (
	QueueConfig struct {
		Conn       *amqp.Connection
		Channel    *amqp.Channel
		LogChannel *amqp.Channel
		JobQueue   *amqp.Queue
	}

	// Manager to handle server connection and config
	Manager struct {
		Settings *domain.Settings
		Queue    *QueueConfig
		Zk       *util.ZkClient

		Server string
		Token  string
		Port   int

		Workspace  string
		LoggingDir string
		PluginDir  string

		VolumesStr string
		Volumes    []*domain.DockerVolume

		AppCtx context.Context
		Cancel context.CancelFunc
	}
)

// GetInstance get singleton of config manager
func GetInstance() *Manager {
	once.Do(func() {
		singleton = new(Manager)
	})
	return singleton
}

func (m *Manager) Init() {
	// init dir
	_ = os.MkdirAll(m.Workspace, os.ModePerm)
	_ = os.MkdirAll(m.LoggingDir, os.ModePerm)
	_ = os.MkdirAll(m.PluginDir, os.ModePerm)

	ctx, cancel := context.WithCancel(context.Background())
	m.AppCtx = ctx
	m.Cancel = cancel

	m.initVolumes()
	m.loadSettings()
	m.initRabbitMQ()
	m.initZookeeper()
	m.sendAgentProfile()
}

// HasQueue has rabbit mq connected
func (m *Manager) HasQueue() bool {
	return m.Queue != nil
}

// HasZookeeper has zookeeper connected
func (m *Manager) HasZookeeper() bool {
	return m.Zk != nil
}

func (m *Manager) FetchProfile() *domain.Resource {
	nCpu, _ := cpu.Counts(true)
	vmStat, _ := mem.VirtualMemory()
	diskStat, _ := disk.Usage("/")

	return &domain.Resource{
		Cpu:         nCpu,
		TotalMemory: util.ByteToMB(vmStat.Total),
		FreeMemory:  util.ByteToMB(vmStat.Available),
		TotalDisk:   util.ByteToMB(diskStat.Total),
		FreeDisk:    util.ByteToMB(diskStat.Free),
	}
}

// Close release resources and connections
func (m *Manager) Close() {
	if m.HasQueue() {
		_ = m.Queue.Channel.Close()
		_ = m.Queue.LogChannel.Close()
		_ = m.Queue.Conn.Close()
	}

	if m.HasZookeeper() {
		m.Zk.Close()
	}
}

// --------------------------------
//		Private Functions
// --------------------------------

func (m *Manager) initVolumes() {
	if util.IsEmptyString(m.VolumesStr) {
		return
	}

	volumes := domain.NewVolumesFromString(m.VolumesStr)

	cli, err := client.NewEnvClient()
	util.PanicIfErr(err)

	for _, vol := range volumes {
		filter := filters.NewArgs()
		filter.Add("name", vol.Name)

		list, err := cli.VolumeList(m.AppCtx, filter)
		util.PanicIfErr(err)

		if len(list.Volumes) == 0 {
			panic(fmt.Errorf("docker volume '%s' not found", vol.Name))
		}
	}
}

func (m *Manager) loadSettings() {
	uri := m.Server + "/agents/connect"
	body, _ := json.Marshal(domain.AgentInit{
		Port:     m.Port,
		Os:       util.OS(),
		Resource: m.FetchProfile(),
	})

	request, _ := http.NewRequest("POST", uri, bytes.NewBuffer(body))
	request.Header.Set(util.HttpHeaderContentType, util.HttpMimeJson)
	request.Header.Set(util.HttpHeaderAgentToken, m.Token)

	resp, errFromReq := http.DefaultClient.Do(request)
	util.PanicIfErr(errFromReq)

	defer resp.Body.Close()
	raw, _ := ioutil.ReadAll(resp.Body)

	var message domain.SettingsResponse
	errFromJSON := json.Unmarshal(raw, &message)
	util.PanicIfErr(errFromJSON)

	if !message.IsOk() {
		panic(fmt.Errorf(message.Message))
	}

	m.Settings = message.Data
	util.LogDebug("Settings been loaded from server: \n%v", m.Settings)
}

func (m *Manager) initRabbitMQ() {
	if m.Settings == nil {
		panic(ErrSettingsNotBeenLoaded)
	}

	// get connection
	connStr := m.Settings.Queue.GetConnectionString()
	conn, err := amqp.Dial(connStr)
	util.PanicIfErr(err)

	// create channel for job queue and send back the result
	ch, err := conn.Channel()
	util.PanicIfErr(err)

	// create channel for push log to server
	logCh, err := conn.Channel()
	util.PanicIfErr(err)

	// init queue config
	qc := new(QueueConfig)
	qc.Conn = conn
	qc.Channel = ch
	qc.LogChannel = logCh

	// init queue to receive job
	jobQueue, err := ch.QueueDeclare(m.Settings.Agent.GetQueueName(), false, false, false, false, nil)
	util.PanicIfErr(err)

	qc.JobQueue = &jobQueue
	m.Queue = qc
}

func (m *Manager) initZookeeper() {
	if m.Settings == nil {
		panic(ErrSettingsNotBeenLoaded)
	}

	zkConfig := m.Settings.Zookeeper

	// make connection of zk
	client := new(util.ZkClient)
	err := client.Connect(zkConfig.Host)
	if err != nil {
		panic(err)
	}

	m.Zk = client

	// register agent on zk
	agentPath := getZkPath(m.Settings)
	_, nodeErr := client.Create(agentPath, util.ZkNodeTypeEphemeral, string(domain.AgentIdle))

	if nodeErr != nil {
		panic(nodeErr)
	}

	util.LogInfo("The zk node '%s' has been registered", agentPath)
}

func (m *Manager) sendAgentProfile() {
	uri := m.Server + "/agents/resource"
	ctx, cancel := context.WithCancel(m.AppCtx)

	go func() {
		defer cancel()

		for {
			select {
			case <-ctx.Done(): // if cancel() execute
				return
			default:
				time.Sleep(1 * time.Minute)
			}

			body, err := json.Marshal(m.FetchProfile())
			if err != nil {
				continue
			}

			request, _ := http.NewRequest("POST", uri, bytes.NewBuffer(body))
			request.Header.Set(util.HttpHeaderContentType, util.HttpMimeJson)
			request.Header.Set(util.HttpHeaderAgentToken, m.Token)

			_, _ = http.DefaultClient.Do(request)
		}
	}()
}

func getZkPath(s *domain.Settings) string {
	return s.Zookeeper.Root + "/" + s.Agent.ID
}