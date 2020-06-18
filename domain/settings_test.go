package domain

import (
	"encoding/json"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSettingsShouldParseFromJson(t *testing.T) {
	// init:
	assert := assert.New(t)

	// given: json data
	raw, _ := ioutil.ReadFile("../_testdata/agent_settings.json")

	// when: parse
	var settings Settings
	err := json.Unmarshal(raw, &settings)

	assert.Nil(err)
	assert.NotNil(settings)

	// then: verify queue data
	assert.NotNil(settings.Queue)
	assert.Equal("amqp://guest:guest@127.0.0.1:5672", settings.Queue.Uri)
	assert.Equal("callback-q", settings.Queue.Callback)
	assert.Equal("shelllog-exchange", settings.Queue.ShellLogEx)
	assert.Equal("ttylog-exchange", settings.Queue.TtyLogEx)

	// then: verify zookeeper data
	assert.NotNil(settings.Zookeeper)
	assert.Equal("127.0.0.1:2181", settings.Zookeeper.Host)
	assert.Equal("/flow-x", settings.Zookeeper.Root)
}
