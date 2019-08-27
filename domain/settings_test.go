package domain

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

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
			"uri": "amqp://guest:guest@127.0.0.1:5671",
			"callback": "callback-q",
			"logsExchange": "logs-exchange"
		},

		"zookeeper": {
			"host": "127.0.0.1:2181",
			"root": "/flow-x"
		}
	}`)

	// when: parse
	var settings Settings
	err := json.Unmarshal(raw, &settings)

	assert.Nil(err)
	assert.NotNil(settings)

	// then: verify queue data
	assert.NotNil(settings.Queue)
	assert.Equal("amqp://guest:guest@127.0.0.1:5671", settings.Queue.Uri)
	assert.Equal("callback-q", settings.Queue.Callback)
	assert.Equal("logs-exchange", settings.Queue.LogsExchange)

	// then: verify zookeeper data
	assert.NotNil(settings.Zookeeper)
	assert.Equal("127.0.0.1:2181", settings.Zookeeper.Host)
	assert.Equal("/flow-x", settings.Zookeeper.Root)

}
