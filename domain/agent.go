package domain

import (
	"fmt"
)

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
		Resource     *Resource `json:"resource"`
	}

	// Agent Class
	Agent struct {
		ID       string
		Name     string
		Token    string
		Host     string
		Tags     []string
		Status   AgentStatus
		JobID    string
		Resource *Resource
	}
)

func (a *Agent) HasHost() bool {
	return a.Host != ""
}

func (a *Agent) IsBusy() bool {
	return a.Status == AgentBusy
}

func (a *Agent) IsIdle() bool {
	return a.Status == AgentIdle
}

func (a *Agent) IsOffline() bool {
	return a.Status == AgentOffline
}

func (a *Agent) IsOnline() bool {
	return a.Status != AgentOffline
}

func (a *Agent) GetQueueName() string {
	return "queue.agent." + a.ID
}

func (a Agent) String() string {
	return fmt.Sprintf("Agent:[id=%s, name=%s, token=%s]", a.ID, a.Name, a.Token)
}
