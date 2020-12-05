package domain

import "github.com/docker/docker/api/types/container"

type DockerConfig struct {
	Name     string
	Auth     *SimpleAuthPair
	Config   *container.Config
	Host     *container.HostConfig
	IsStop   bool
	IsDelete bool

	ContainerID string // try to resume if container id is existed
}

func (c *DockerConfig) HasEntrypoint() bool {
	return c.Config.Entrypoint != nil && len(c.Config.Entrypoint) > 0
}
