package domain

type AgentStatus string

const (
	OFFLINE AgentStatus = "OFFLINE"
	BUSY    AgentStatus = "BUSY"
	IDLE    AgentStatus = "IDLE"
)

type Agent struct {
	id     string
	name   string
	token  string
	host   string
	tags   []string
	jobID  string
	status AgentStatus
}
