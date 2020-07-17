package domain

import (
	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"github/flowci/flow-agent-x/util"
)

type (
	DockerOption struct {
		Image             string    `json:"image"`
		Entrypoint        []string  `json:"entrypoint"` // host:container
		Ports             []string  `json:"ports"`
		NetworkMode       string    `json:"networkMode"`
		Environment       Variables `json:"environment"`
		User              string    `json:"user"`
		IsRuntime         bool      `json:"isRuntime"`
		IsStopContainer   bool      `json:"isStopContainer"`
		IsDeleteContainer bool      `json:"isDeleteContainer"`
	}

	DockerConfig struct {
		Config   *container.Config
		Host     *container.HostConfig
		IsStop   bool
		IsDelete bool

		ContainerID string
	}
)

func (d *DockerOption) ToRuntimeConfig(vars Variables, workingDir string, binds []string) *DockerConfig {
	return d.toConfig(vars, workingDir, binds, true)
}

func (d *DockerOption) ToConfig() *DockerConfig {
	return d.toConfig(nil, "", nil, false)
}

func (d *DockerOption) toConfig(vars Variables, workingDir string, binds []string, enableInput bool) (config *DockerConfig) {
	portSet, portMap, err := nat.ParsePortSpecs(d.Ports)
	util.PanicIfErr(err)

	if vars == nil {
		vars = d.Environment
	} else {
		vars = ConnectVars(vars.Copy(), d.Environment)
	}

	config = &DockerConfig{
		Config: &container.Config{
			Image:        util.ParseStringWithSource(d.Image, vars),
			Env:          vars.ToStringArray(),
			Entrypoint:   d.Entrypoint,
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
			NetworkMode:  container.NetworkMode(d.NetworkMode),
			PortBindings: portMap,
		},
		IsStop:   d.IsStopContainer,
		IsDelete: d.IsDeleteContainer,
	}

	if util.HasString(workingDir) {
		config.Config.WorkingDir = workingDir
	}

	if binds != nil {
		config.Host.Binds = binds
	}

	return
}
