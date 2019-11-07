package domain

import (
	"fmt"
)

type (
	// ZookeeperConfig the zookeeper config data
	ZookeeperConfig struct {
		Host string
		Root string
	}

	// RabbitMQConfig the mq config data
	RabbitMQConfig struct {
		Uri          string
		Callback     string
		LogsExchange string
	}

	// Settings the setting info from server side
	Settings struct {
		Agent     *Agent
		Queue     *RabbitMQConfig
		Zookeeper *ZookeeperConfig
	}
)

// ===================================
//		RabbitMQConfig Methods
// ===================================

// GetConnectionString rabbitmq server connection string
func (c *RabbitMQConfig) GetConnectionString() string {
	return c.Uri
}

func (c *RabbitMQConfig) String() string {
	return fmt.Sprintf("RABBIT:[uri=%s, callback=%s, logsExchange=%s]", c.Uri, c.Callback, c.LogsExchange)
}

// ===================================
//		ZookeeperConfig Methods
// ===================================

func (zk ZookeeperConfig) String() string {
	return fmt.Sprintf("ZK:[host=%s, root=%s]", zk.Host, zk.Root)
}

// ===================================
//		Settings Methods
// ===================================

func (s Settings) String() string {
	return fmt.Sprintf("%s\n%s\n%s", s.Agent.String(), s.Zookeeper.String(), s.Queue.String())
}
