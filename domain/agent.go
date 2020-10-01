package domain

// AgentStatus string of agent status
type AgentStatus string

const (
	// AgentOffline offline status
	AgentOffline AgentStatus = "OFFLINE"

	// AgentBusy busy status
	AgentBusy AgentStatus = "BUSY"

	// AgentIdle idle status
	AgentIdle AgentStatus = "IDLE"
)

type (
	// Resource agent resource data
	Resource struct {
		Cpu         int    `json:"cpu"`
		TotalMemory uint64 `json:"totalMemory"`
		FreeMemory  uint64 `json:"freeMemory"`
		TotalDisk   uint64 `json:"totalDisk"`
		FreeDisk    uint64 `json:"freeDisk"`
	}

	// AgentConnect request data to get settings from server
	AgentInit struct {
		IsK8sCluster bool      `json:"k8sCluster"`
		Token        string    `json:"token"`
		Port         int       `json:"port"`
		Os           string    `json:"os"`
		Status       string    `json:"status"`
		Resource     *Resource `json:"resource"`
	}
)
