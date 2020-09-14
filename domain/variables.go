package domain

import (
	"fmt"
	"github/flowci/flow-agent-x/util"
)

const (
	VarServerUrl = "FLOWCI_SERVER_URL"

	VarAgentToken     = "FLOWCI_AGENT_TOKEN"
	VarAgentPort      = "FLOWCI_AGENT_PORT"
	VarAgentWorkspace = "FLOWCI_AGENT_WORKSPACE"
	VarAgentJobDir    = "FLOWCI_AGENT_JOB_DIR"
	VarAgentPluginDir = "FLOWCI_AGENT_PLUGIN_DIR"
	VarAgentLogDir    = "FLOWCI_AGENT_LOG_DIR"
	VarAgentVolumes   = "FLOWCI_AGENT_VOLUMES"

	VarK8sEnabled   = "FLOWCI_AGENT_K8S_ENABLED"    // boolean
	VarK8sInCluster = "FLOWCI_AGENT_K8S_IN_CLUSTER" // boolean

	VarK8sNodeName  = "K8S_NODE_NAME"
	VarK8sPodName   = "K8S_POD_NAME"
	VarK8sPodIp     = "K8S_POD_IP"
	VarK8sNamespace = "K8S_NAMESPACE"

	VarAgentIpPattern           = "FLOWCI_AGENT_IP_%s"        // ip address of agent host
	VarExportContainerIdPattern = "export CONTAINER_ID_%d=%s" // container id , d=index of dockers
	VarExportContainerIpPattern = "export CONTAINER_IP_%d=%s" // container ip , d=index of dockers
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
	if a == nil {
		a = Variables{}
	}

	if b == nil {
		b = Variables{}
	}

	vars := make(Variables, a.Size()+b.Size())
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
func (v Variables) Resolve() Variables {
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

	return v
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
