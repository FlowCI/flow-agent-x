package domain

import (
	"fmt"
	"github/flowci/flow-agent-x/util"
)

const (
	VarServerUrl      = "FLOWCI_SERVER_URL"

	VarAgentToken     = "FLOWCI_AGENT_TOKEN"
	VarAgentPort      = "FLOWCI_AGENT_PORT"
	VarAgentWorkspace = "FLOWCI_AGENT_WORKSPACE"
	VarAgentJobDir    = "FLOWCI_AGENT_JOB_DIR"
	VarAgentPluginDir = "FLOWCI_AGENT_PLUGIN_DIR"
	VarAgentLogDir    = "FLOWCI_AGENT_LOG_DIR"
	VarAgentVolumes    = "FLOWCI_AGENT_VOLUMES"
)

// Variables applied for environment variable as key, value
type Variables map[string]string

func NewVariables() Variables {
	return Variables{
		"_TYPE_": "_string_",
	}
}

// NilOrEmpty detect variable is nil or empty
func NilOrEmpty(v Variables) bool {
	return v == nil || v.IsEmpty()
}

func ConnectVars(a Variables, b Variables) Variables {
	vars := make(Variables, a.Size() + b.Size())
	for k, val := range a {
		vars[k] = val
	}

	for k, val := range b {
		vars[k] = val
	}
	return vars
}

func (v Variables) Copy() Variables {
	copied := make(Variables, v.Size())
	for k, val := range v {
		copied[k] = val
	}
	return copied
}

func (v Variables) Size() int {
	return len(v)
}

// Resolve to gain actual value from env variables
func (v Variables) Resolve() {
	// resolve from system env vars
	for key, val := range v {
		val = util.ParseString(val)
		v[key] = val
	}

	// resolve from current env vars
	for key, val := range v {
		val = util.ParseStringWithSource(val, v)
		v[key] = val
	}
}

// ToStringArray convert variables map to key=value string array
func (v Variables) ToStringArray() []string {
	array := make([]string, v.Size())
	index := 0
	for key, val := range v {
		array[index] = fmt.Sprintf("%s=%s", key, val)
		index++
	}

	return array
}

// IsEmpty to check is empty variables
func (v Variables) IsEmpty() bool {
	return len(v) == 0
}
