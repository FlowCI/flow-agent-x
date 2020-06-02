package domain

import (
	"time"
)

type CmdType string

type CmdStatus string

const (
	// CmdTypeShell shell command
	CmdTypeShell CmdType = "SHELL"

	// CmdTypeKill kill command
	CmdTypeKill CmdType = "KILL"

	// CmdTypeClose close command
	CmdTypeClose CmdType = "CLOSE"

	// CmdTypeSessionOpen open new session to interact mode
	CmdTypeSessionOpen CmdType = "SESSION_OPEN"

	// CmdTypeSessionShell send cmd with interact mode
	CmdTypeSessionShell CmdType = "SESSION_SHELL"

	// CmdTypeSessionClose close session of interact mode
	CmdTypeSessionClose CmdType = "SESSION_CLOSE"
)

const (
	CmdStatusPending CmdStatus = "PENDING"

	CmdStatusRunning CmdStatus = "RUNNING"

	CmdStatusSuccess CmdStatus = "SUCCESS"

	CmdStatusSkipped CmdStatus = "SKIPPED"

	CmdStatusException CmdStatus = "EXCEPTION"

	CmdStatusKilled CmdStatus = "KILLED"

	CmdStatusTimeout CmdStatus = "TIMEOUT"
)

const (
	// CmdExitCodeUnknown default exit code
	CmdExitCodeUnknown = -1

	// CmdExitCodeTimeOut exit code for timeout
	CmdExitCodeTimeOut = -100

	// CmdExitCodeKilled exit code for killed
	CmdExitCodeKilled = -1

	// CmdExitCodeSuccess exit code for command executed successfully
	CmdExitCodeSuccess = 0
)

type (
	DockerOption struct {
		Image             string   `json:"image"`
		Entrypoint        []string `json:"entrypoint"` // host:container
		Ports             []string `json:"ports"`
		NetworkMode       string   `json:"networkMode"`
		User              string   `json:"user"`
		IsStopContainer   bool     `json:"isStopContainer"`
		IsDeleteContainer bool     `json:"isDeleteContainer"`
	}

	Cmd struct {
		ID           string        `json:"id"`
		FlowId       string        `json:"flowId"`
		JobId        string        `json:"jobId"`
		NodePath     string        `json:"nodePath"`
		BuildNumber  int           `json:"buildNumber"`
		ContainerId  string        `json:"containerId"` // container id prefer to reuse
		AllowFailure bool          `json:"allowFailure"`
		Plugin       string        `json:"plugin"`
		Docker       *DockerOption `json:"docker"`
	}

	CmdIn struct {
		Cmd
		Type       CmdType   `json:"type"`
		Scripts    []string  `json:"scripts"`
		Timeout    int       `json:"timeout"`
		Inputs     Variables `json:"inputs"`
		EnvFilters []string  `json:"envFilters"`
	}

	ExecutedCmd struct {
		Cmd
		ProcessId int       `json:"processId"`
		Status    CmdStatus `json:"status"`
		Code      int       `json:"code"`
		Output    Variables `json:"output"`
		StartAt   time.Time `json:"startAt"`
		FinishAt  time.Time `json:"finishAt"`
		Error     string    `json:"error"`
		LogSize   int64     `json:"logSize"`
	}
)

// ===================================
//		Cmd Methods
// ===================================

func (cmd *Cmd) HasPlugin() bool {
	return cmd.Plugin != ""
}

func (cmd *Cmd) HasDockerOption() bool {
	return cmd.Docker != nil
}

// ===================================
//		CmdIn Methods
// ===================================
func (in *CmdIn) HasScripts() bool {
	if in.Scripts == nil {
		return false
	}

	return len(in.Scripts) != 0
}

func (in *CmdIn) HasEnvFilters() bool {
	if in.EnvFilters == nil {
		return false
	}

	return len(in.EnvFilters) != 0
}

func (in *CmdIn) VarsToStringArray() []string {
	if !NilOrEmpty(in.Inputs) {
		return in.Inputs.ToStringArray()
	}

	return []string{}
}

// ===================================
//		ExecutedCmd Methods
// ===================================

func NewExecutedCmd(in *CmdIn) *ExecutedCmd {
	return &ExecutedCmd{
		Cmd: Cmd{
			ID:           in.ID,
			FlowId:       in.FlowId,
			JobId:        in.JobId,
			NodePath:     in.NodePath,
			BuildNumber:  in.BuildNumber,
			AllowFailure: in.AllowFailure,
			ContainerId:  in.ContainerId,
			Plugin:       in.Plugin,
			Docker:       in.Docker,
		},
		Code:   CmdExitCodeUnknown,
		Status: CmdStatusPending,
		Output: NewVariables(),
	}
}

func (e *ExecutedCmd) IsFinishStatus() bool {
	switch e.Status {
	case CmdStatusKilled:
		return true
	case CmdStatusTimeout:
		return true
	case CmdStatusException:
		return true
	case CmdStatusSuccess:
		return true
	default:
		return false
	}
}
