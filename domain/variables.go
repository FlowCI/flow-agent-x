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

	VariablesTypeField  = "_TYPE_"
	VariablesStringType = "_string_"
)

// Variables applied for environment variable as key, value
type Variables map[string]string

// NilOrEmpty detect variable is nil or empty
func NilOrEmpty(v Variables) bool {
	return v == nil || v.IsEmpty()
}

func (v Variables) Copy() Variables {
	copied := make(Variables, len(v))
	for k, val := range v {
		copied[k] = val
	}
	return copied
}

// ToStringArray convert variables map to key=value string array
func (v Variables) ToStringArray() []string {
	// build env variables map
	envs := make(map[string]string, len(v))
	for key, val := range v {
		val = util.ParseString(val)
		envs[key] = val
	}

	array := make([]string, len(v))
	index := 0
	for key, val := range envs {
		val = util.ParseStringWithSource(val, envs)
		array[index] = fmt.Sprintf("%s=%s", key, val)
		index++
	}

	return array
}

// IsEmpty to check is empty variables
func (v Variables) IsEmpty() bool {
	return len(v) == 0
}
