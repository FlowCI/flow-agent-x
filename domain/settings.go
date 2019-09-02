package domain

import (
	"fmt"
)

// RabbitMQConfig the mq config data
type RabbitMQConfig struct {
	Uri          string
	Callback     string
	LogsExchange string
}

// GetConnectionString rabbitmq server connection string
func (c *RabbitMQConfig) GetConnectionString() string {
	return c.Uri
}

func (c *RabbitMQConfig) String() string {
	return fmt.Sprintf("RABBIT:[uri=%s, callback=%s, logsExchange=%s]", c.Uri, c.Callback, c.LogsExchange)
}

// ZookeeperConfig the zookeeper config data
type ZookeeperConfig struct {
	Host string
	Root string
}

func (zk ZookeeperConfig) String() string {
	return fmt.Sprintf("ZK:[host=%s, root=%s]", zk.Host, zk.Root)
}

// Settings the setting info from server side
type Settings struct {
	Agent     *Agent
	Queue     *RabbitMQConfig
	Zookeeper *ZookeeperConfig
}

func (s Settings) String() string {
	return fmt.Sprintf("%s\n%s\n%s", s.Agent.String(), s.Zookeeper.String(), s.Queue.String())
}
