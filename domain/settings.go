package domain

import (
	"fmt"
)

// RabbitMQConfig the mq config data
type RabbitMQConfig struct {
	Host     string
	Port     int
	Username string
	Password string
}

// GetConnectionString rabbitmq server connection string
func (c *RabbitMQConfig) GetConnectionString() string {
	return fmt.Sprintf("amqp://%s:%s@%s:%d", c.Username, c.Password, c.Host, c.Port)
}

// ZookeeperConfig the zookeeper config data
type ZookeeperConfig struct {
	Host string
	Root string
}

// Settings the setting info from server side
type Settings struct {
	Agent             Agent
	Queue             RabbitMQConfig
	Zookeeper         ZookeeperConfig
	CallbackQueueName string
	LogsExchangeName  string
}
