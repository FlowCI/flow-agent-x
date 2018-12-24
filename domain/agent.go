package domain

type AgentStatus string

const (
	OFFLINE AgentStatus = "OFFLINE"
	BUSY    AgentStatus = "BUSY"
	IDLE    AgentStatus = "IDLE"
)

type Agent struct {
	Id     string
	Name   string
	Token  string
	Host   string
	Tags   []string
	Status AgentStatus
	JobId  string
}

func (a *Agent) HasHost() bool {
	return a.Host != ""
}

func (a *Agent) IsBusy() bool {
	return a.Status == BUSY
}

func (a *Agent) IsIdle() bool {
	return a.Status == IDLE
}

func (a *Agent) IsOffline() bool {
	return a.Status == OFFLINE
}

func (a *Agent) IsOnline() bool {
	return a.Status != OFFLINE
}

func (a *Agent) GetQueueName() string {
	return "queue.agent." + a.Id
}
