package config

import (
	"context"
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
		Zk *util.ZkClient

		Status domain.AgentStatus

		Server string
		Token  string
		Port   int

		K8sEnabled   bool
		K8sCluster   bool
		K8sNodeName  string
		K8sPodName   string
		K8sPodIp     string
		K8sNamespace string

		Workspace  string
		LoggingDir string
		PluginDir  string

		Client api.Client

		VolumesStr string
		Volumes    []*domain.DockerVolume

		AppCtx context.Context
		Cancel context.CancelFunc
	}
)

// GetInstance get singleton of config manager
func GetInstance() *Manager {
	once.Do(func() {
		singleton = &Manager{
			Status: domain.AgentIdle,
		}
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
	m.connect()
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
}

func (m *Manager) connect() {
	initData := &domain.AgentInit{
		IsK8sCluster: m.K8sCluster,
		Port:         m.Port,
		Os:           util.OS(),
		Resource:     m.FetchProfile(),
		Status:       string(m.Status),
	}

	err := m.Client.Connect(initData)
	util.PanicIfErr(err)
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
