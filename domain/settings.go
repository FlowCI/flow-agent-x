package domain

// RabbitMQConfig the mq config data
type RabbitMQConfig struct {
	Host     string
	Port     int
	Username string
	Password string
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
