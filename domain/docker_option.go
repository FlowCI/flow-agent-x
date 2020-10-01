package domain

import (
	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"github/flowci/flow-agent-x/util"
)

type (
	DockerOption struct {
		Image             string    `json:"image"`
		Name              string    `json:"name"`
		Entrypoint        []string  `json:"entrypoint"` // host:container
		Command           []string  `json:"command"`
		Ports             []string  `json:"ports"`
		Network           string    `json:"network"`
		Environment       Variables `json:"environment"`
		User              string    `json:"user"`
		IsRuntime         bool      `json:"isRuntime"`
		IsStopContainer   bool      `json:"isStopContainer"`
		IsDeleteContainer bool      `json:"isDeleteContainer"`
		ContainerID       string    // try to resume if container id is existed
	}
)

func (d *DockerOption) ToRuntimeConfig(vars Variables, workingDir string, binds []string) *DockerConfig {
	return d.toConfig(vars, workingDir, binds, true)
}

func (d *DockerOption) ToConfig() *DockerConfig {
	return d.toConfig(nil, "", nil, false)
}

func (d *DockerOption) SetDefaultNetwork(network string) {
	if d.Network == "bridge" || d.Network == "" {
		d.Network = network
	}
}

func (d *DockerOption) toConfig(vars Variables, workingDir string, binds []string, enableInput bool) (config *DockerConfig) {
	portSet, portMap, err := nat.ParsePortSpecs(d.Ports)
	util.PanicIfErr(err)

	vars = ConnectVars(vars, d.Environment)

	config = &DockerConfig{
		Name: d.Name,
		Config: &container.Config{
			Image:        util.ParseStringWithSource(d.Image, vars),
			Env:          vars.ToStringArray(),
			Entrypoint:   d.Entrypoint,
			Cmd:          d.Command,
			ExposedPorts: portSet,
			User:         d.User,
			Tty:          false,
			AttachStdin:  enableInput,
			AttachStderr: enableInput,
			AttachStdout: enableInput,
			OpenStdin:    enableInput,
			StdinOnce:    enableInput,
		},
		Host: &container.HostConfig{
			NetworkMode:  container.NetworkMode(d.Network),
			PortBindings: portMap,
		},
		IsStop:      d.IsStopContainer,
		IsDelete:    d.IsDeleteContainer,
		ContainerID: d.ContainerID,
	}

	if util.HasString(workingDir) {
		config.Config.WorkingDir = workingDir
	}

	if binds != nil {
		config.Host.Binds = binds
	}

	return
}
