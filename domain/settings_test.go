package domain

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRabbitMQConfigConnectionString(t *testing.T) {
	assert := assert.New(t)

	mq := &RabbitMQConfig{
		Host:     "aaa",
		Port:     1234,
		Username: "guest",
		Password: "guest",
	}

	assert.Equal("amqp://guest:guest@aaa:1234", mq.GetConnectionString())
}

func TestSettingsShouldParseFromJson(t *testing.T) {
	// init:
	assert := assert.New(t)

	// given: json data
	raw := []byte(`{
		"agent": {
			"id": "1",
			"name": "local",
			"token": "xxx-xxx",
			"host": "test",
			"tags": ["ios", "mac"],
			"status": "OFFLINE",
			"jobid": "job-id"
		},

		"queue": {
			"host": "127.0.0.1",
			"port": 15671,
			"username": "guest",
			"password": "guest"
		},

		"zookeeper": {
			"host": "127.0.0.1:2181",
			"root": "/flow-x"
		},

		"callbackQueueName": "callback-q",
		"logsExchangeName": "logs-exchange"
	}`)

	// when: parse
	var settings Settings
	err := json.Unmarshal(raw, &settings)

	assert.Nil(err)
	assert.NotNil(settings)

	// then: verify settings data
	assert.Equal("callback-q", settings.CallbackQueueName)
	assert.Equal("logs-exchange", settings.LogsExchangeName)

	// then: verify queue data
	assert.NotNil(settings.Queue)
	assert.Equal("127.0.0.1", settings.Queue.Host)
	assert.Equal(15671, settings.Queue.Port)
	assert.Equal("guest", settings.Queue.Username)
	assert.Equal("guest", settings.Queue.Password)

	// then: verify zookeeper data
	assert.NotNil(settings.Zookeeper)
	assert.Equal("127.0.0.1:2181", settings.Zookeeper.Host)
	assert.Equal("/flow-x", settings.Zookeeper.Root)

}
