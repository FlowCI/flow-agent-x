package config

import (
	"context"
	"fmt"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/mem"
	"github/flowci/flow-agent-x/api"
	"github/flowci/flow-agent-x/domain"
	"github/flowci/flow-agent-x/util"
	"os"
	"time"
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
	err := m.connect()
	util.PanicIfErr(err)

	m.listenReConn()
	m.sendAgentProfile()
}

func (m *Manager) FetchProfile() *domain.AgentProfile {
	nCpu, _ := cpu.Counts(true)
	percent, _ := cpu.Percent(time.Second, false)
	vmStat, _ := mem.VirtualMemory()
	diskStat, _ := disk.Usage("/")

	return &domain.AgentProfile{
		CpuNum:      nCpu,
		CpuUsage:    percent[0],
		TotalMemory: util.ByteToMB(vmStat.Total),
		FreeMemory:  util.ByteToMB(vmStat.Available),
		TotalDisk:   util.ByteToMB(diskStat.Total),
		FreeDisk:    util.ByteToMB(diskStat.Free),
	}
}

// Close release resources and connections
func (m *Manager) Close() {
	m.Client.Close()
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

func (m *Manager) connect() error {
	initData := &domain.AgentInit{
		IsK8sCluster: m.K8sCluster,
		Port:         m.Port,
		Os:           util.OS(),
		Status:       string(m.Status),
	}

	return m.Client.Connect(initData)
}

func (m *Manager) listenReConn() {
	go func() {
		for range m.Client.ReConn() {
			util.LogWarn("connection lost from server %s, start reconnecting..", m.Server)
			connected := false

			for i := 0; i < 6; i++ {
				err := m.connect()
				if err == nil {
					connected = true
					break
				}

				util.LogWarn("unable to connect to server %s, retry...", m.Server)
				time.Sleep(10 * time.Second)
			}

			if !connected {
				panic(fmt.Errorf("unable to connect to server %s, exit", m.Server))
			}
		}
	}()
}

func (m *Manager) sendAgentProfile() {
	go func() {
		for {
			select {
			case <-m.AppCtx.Done():
				return
			default:
				time.Sleep(10 * time.Second)
				_ = m.Client.ReportProfile(m.FetchProfile())
			}
		}
	}()
}
