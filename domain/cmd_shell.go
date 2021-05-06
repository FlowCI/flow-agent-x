package domain

import (
	"encoding/json"
	"time"
)

type (
	Cache struct {
		Key   string   `json:"key"`
		Paths []string `json:"paths"`
	}

	ShellIn struct {
		CmdIn
		ID           string          `json:"id"`
		FlowId       string          `json:"flowId"`
		JobId        string          `json:"jobId"`
		AllowFailure bool            `json:"allowFailure"`
		Plugin       string          `json:"plugin"`
		Cache        *Cache          `json:"cache"`
		Dockers      []*DockerOption `json:"dockers"`
		Bash         []string        `json:"bash"`
		Pwsh         []string        `json:"pwsh"`
		Retry        int             `json:"retry"`
		Timeout      int             `json:"timeout"`
		Inputs       Variables       `json:"inputs"`
		EnvFilters   []string        `json:"envFilters"`
		Secrets      []string        `json:"secrets"`
	}

	ShellOut struct {
		ID         string    `json:"id"`
		ProcessId  int       `json:"processId"`
		Containers []string  `json:"containers"` // container ids applied for shell
		Status     CmdStatus `json:"status"`
		Code       int       `json:"code"`
		Output     Variables `json:"output"`
		StartAt    time.Time `json:"startAt"`
		FinishAt   time.Time `json:"finishAt"`
		Error      string    `json:"error"`
		LogSize    int64     `json:"logSize"`
	}

	ShellLog struct {
		JobId  string `json:"jobId"`
		StepId string `json:"stepId"`
		Log    string `json:"log"` // b64
	}
)

// ===================================
//		ShellIn Methods
// ===================================

func (in *ShellIn) HasSecrets() bool {
	return in.Secrets != nil && len(in.Secrets) > 0
}

func (in *ShellIn) HasCache() bool {
	return in.Cache != nil
}

func (in *ShellIn) HasPlugin() bool {
	return in.Plugin != ""
}

func (in *ShellIn) HasDockerOption() bool {
	return in.Dockers != nil && len(in.Dockers) > 0
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
