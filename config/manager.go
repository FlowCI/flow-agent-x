package config

import (
	"context"
	"fmt"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/mem"
	"github/flowci/flow-agent-x/api"
	"github/flowci/flow-agent-x/domain"
	"github/flowci/flow-agent-x/util"
	"os"
	"sync"
	"time"
)

var (
	singleton *Manager
	once      sync.Once
)

type (
	// Manager to handle server connection and config
	Manager struct {
		Settings *domain.Settings
		Zk       *util.ZkClient

		Server string
		Token  string
		Port   int

		Workspace  string
		LoggingDir string
		PluginDir  string

		Client *api.Client

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
	m.Client = api.NewClient(m.Token, m.Server)

	m.initVolumes()
	m.loadSettings()
	m.initRabbitMQ()
	m.initZookeeper()
	m.sendAgentProfile()
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
	m.Client.Close()

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

	m.Volumes = domain.NewVolumesFromString(m.VolumesStr)

	cli, err := client.NewEnvClient()
	util.PanicIfErr(err)

	for _, vol := range m.Volumes {
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
	initData := &domain.AgentInit{
		Port:     m.Port,
		Os:       util.OS(),
		Resource: m.FetchProfile(),
	}

	settings, err := m.Client.GetSettings(initData)
	util.PanicIfErr(err)

	m.Settings = settings
	util.LogDebug("Settings been loaded from server: \n%v", m.Settings)
}

func (m *Manager) initRabbitMQ() {
	if m.Settings == nil {
		panic(ErrSettingsNotBeenLoaded)
	}

	agentQ := m.Settings.Agent.GetQueueName()
	err := m.Client.SetQueue(m.Settings.Queue, agentQ)
	util.PanicIfErr(err)
}

func (m *Manager) initZookeeper() {
	if m.Settings == nil {
		panic(ErrSettingsNotBeenLoaded)
	}

	zkConfig := m.Settings.Zookeeper

	// make connection of zk
	zk := util.NewZkClient()
	zk.Callbacks.OnDisconnected = func() {
		m.Cancel()
	}

	err := zk.Connect(zkConfig.Host)
	if err != nil {
		panic(err)
	}

	m.Zk = zk

	// register agent on zk
	_, _ = zk.Create(m.Settings.Zookeeper.Root, util.ZkNodeTypePersistent, "")
	agentPath := getZkPath(m.Settings)
	_, nodeErr := zk.Create(agentPath, util.ZkNodeTypeEphemeral, string(domain.AgentIdle))

	if nodeErr != nil {
		panic(nodeErr)
	}

	util.LogInfo("The zk node '%s' has been registered", agentPath)
}

func (m *Manager) sendAgentProfile() {
	go func() {
		for {
			select {
			case <-m.AppCtx.Done():
				return
			default:
				time.Sleep(1 * time.Minute)
				_ = m.Client.ReportProfile(m.FetchProfile())
			}
		}
	}()
}

func getZkPath(s *domain.Settings) string {
	return s.Zookeeper.Root + "/" + s.Agent.ID
}
