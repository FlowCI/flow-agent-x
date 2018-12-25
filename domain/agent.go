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

// AgentConnect request data to get settings from server
type AgentConnect struct {
	Token string `json:"token"`
	Port  int    `json:"port"`
}

// Agent Class
type Agent struct {
	ID     string
	Name   string
	Token  string
	Host   string
	Tags   []string
	Status AgentStatus
	JobID  string
}

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
