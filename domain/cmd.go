package domain

import "time"

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
	DockerDesc struct {
		Image      string   `json:"image"`
		Entrypoint []string `json:"entrypoint"`
		Ports      []string `json:"ports"`
	}

	Cmd struct {
		ID           string `json:"id"`
		AllowFailure bool   `json:"allowFailure"`
		Plugin       string `json:"plugin"`
	}

	CmdIn struct {
		Cmd
		Type       CmdType     `json:"type"`
		Docker     *DockerDesc `json:"docker"`
		Scripts    []string    `json:"scripts"`
		FlowId     string      `json:"flowId"`
		Timeout    int         `json:"timeout"`
		Inputs     Variables   `json:"inputs"`
		EnvFilters []string    `json:"envFilters"`
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
			AllowFailure: in.AllowFailure,
			Plugin:       in.Plugin,
		},
		Code:   CmdExitCodeUnknown,
		Status: CmdStatusPending,
		Output: NewVariables(),
	}
}
