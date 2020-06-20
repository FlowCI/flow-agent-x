package domain

import (
	"encoding/json"
	"time"
)

type (
	ShellIn struct {
		CmdIn
		ID           string        `json:"id"`
		FlowId       string        `json:"flowId"`
		JobId        string        `json:"jobId"`
		ContainerId  string        `json:"containerId"` // container id prefer to reuse
		AllowFailure bool          `json:"allowFailure"`
		Plugin       string        `json:"plugin"`
		Docker       *DockerOption `json:"docker"`
		Scripts      []string      `json:"scripts"`
		Timeout      int           `json:"timeout"`
		Inputs       Variables     `json:"inputs"`
		EnvFilters   []string      `json:"envFilters"`
	}

	ShellOut struct {
		ID          string    `json:"id"`
		ProcessId   int       `json:"processId"`
		ContainerId string    `json:"containerId"` // container id prefer to reuse
		Status      CmdStatus `json:"status"`
		Code        int       `json:"code"`
		Output      Variables `json:"output"`
		StartAt     time.Time `json:"startAt"`
		FinishAt    time.Time `json:"finishAt"`
		Error       string    `json:"error"`
		LogSize     int64     `json:"logSize"`
	}
)

// ===================================
//		ShellIn Methods
// ===================================

func (in *ShellIn) HasPlugin() bool {
	return in.Plugin != ""
}

func (in *ShellIn) HasDockerOption() bool {
	return in.Docker != nil
}

func (in *ShellIn) HasScripts() bool {
	if in.Scripts == nil {
		return false
	}

	return len(in.Scripts) != 0
}

func (in *ShellIn) HasEnvFilters() bool {
	if in.EnvFilters == nil {
		return false
	}

	return len(in.EnvFilters) != 0
}

func (in *ShellIn) VarsToStringArray() []string {
	if !NilOrEmpty(in.Inputs) {
		return in.Inputs.ToStringArray()
	}

	return []string{}
}

func NewShellOutput(in *ShellIn) *ShellOut {
	return &ShellOut{
		ID:     in.ID,
		Code:   CmdExitCodeUnknown,
		Status: CmdStatusPending,
		Output: NewVariables(),
	}
}

// ===================================
//		ShellOut Methods
// ===================================

func (e *ShellOut) IsFinishStatus() bool {
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

func (e *ShellOut) ToBytes() []byte {
	data, _ := json.Marshal(e)
	return append(shellOutInd, data...)
}
