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
	// CmdExitCodeUnknow default exit code
	CmdExitCodeUnknow = -1

	// CmdExitCodeTimeOut exit code for timeout
	CmdExitCodeTimeOut = -100

	// CmdExitCodeKilled exit code for killed
	CmdExitCodeKilled = -1

	// CmdExitCodeSuccess exit code for command executed successfuly
	CmdExitCodeSuccess = 0
)

type Cmd struct {
	ID           string `json:"id"`
	AllowFailure bool   `json:"allowFailure"`
	Plugin       string `json:"plugin"`
	Session      string `json:"session"`
}

func (cmd *Cmd) HasPlugin() bool {
	return cmd.Plugin != ""
}

type CmdIn struct {
	Cmd
	Type       CmdType   `json:"type"`
	Scripts    []string  `json:"scripts"`
	WorkDir    string    `json:"workDir"`
	Timeout    int64     `json:"timeout"`
	Inputs     Variables `json:"inputs"`
	EnvFilters []string  `json:"envFilters"`
}

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

type ExecutedCmd struct {
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
