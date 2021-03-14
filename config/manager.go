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
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type (
	// Manager to handle server connection and config
	Manager struct {
		Zk *util.ZkClient

		Status domain.AgentStatus
		Config *domain.AgentConfig

		Debug  bool
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
	// init vars
	if m.Port < 0 {
		m.Port = m.getDefaultPort()
	}

	m.PluginDir = filepath.Join(m.Workspace, ".plugins")
	m.LoggingDir = filepath.Join(m.Workspace, ".logs")

	m.K8sNodeName = os.Getenv(domain.VarK8sNodeName)
	m.K8sPodName = os.Getenv(domain.VarK8sPodName)
	m.K8sPodIp = os.Getenv(domain.VarK8sPodIp)
	m.K8sNamespace = os.Getenv(domain.VarK8sNamespace)

	// init dir
	_ = os.MkdirAll(m.Workspace, os.ModePerm)
	_ = os.MkdirAll(m.LoggingDir, os.ModePerm)
	_ = os.MkdirAll(m.PluginDir, os.ModePerm)

	ctx, cancel := context.WithCancel(context.Background())
	m.AppCtx = ctx
	m.Cancel = cancel
	m.Client = api.NewClient(m.Token, m.Server)

	m.initVolumes()
	util.PanicIfErr(m.connect())

	m.listenReConn()
	m.sendAgentProfile()
}

func (m *Manager) PrintInfo() {
	util.LogInfo("--- [Server URL]: %s", m.Server)
	util.LogInfo("--- [Token]: %s", m.Token)
	util.LogInfo("--- [Port]: %d", m.Port)
	util.LogInfo("--- [Workspace]: %s", m.Workspace)
	util.LogInfo("--- [Plugin Dir]: %s", m.PluginDir)
	util.LogInfo("--- [Log Dir]: %s", m.LoggingDir)
	util.LogInfo("--- [Volume Str]: %s", m.VolumesStr)

	if m.K8sEnabled {
		util.LogInfo("--- [K8s InCluster]: %d", m.K8sCluster)
		util.LogInfo("--- [K8s Node]: %s", m.K8sNodeName)
		util.LogInfo("--- [K8s Namespace]: %s", m.K8sNamespace)
		util.LogInfo("--- [K8s Pod]: %s", m.K8sPodName)
		util.LogInfo("--- [K8s Pod IP]: %s", m.K8sPodIp)
	}
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

func (m *Manager) getDefaultPort() int {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	util.FailOnError(err, "Cannot start listen localhost")
	defer func() {
		_ = listener.Close()
	}()

	addressAndPort := listener.Addr().String()

	i, err := strconv.Atoi(addressAndPort[strings.Index(addressAndPort, ":")+1:])
	util.FailOnError(err, "Invalid port format")
	return i
}

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

	config, err := m.Client.Connect(initData)
	if err != nil {
		return err
	}

	m.Config = config
	return nil
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
