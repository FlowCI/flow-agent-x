package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"encoding/json"
)

func TestAgentQueueName(t *testing.T) {
	agent := Agent{Id: "1"}

	if agent.GetQueueName() != "queue.agent.1" {
		t.Errorf("Unexpected agent queue name")
	}
}

func TestAgentParesedFromJson(t *testing.T) {
	assert := assert.New(t)
	raw := []byte(`{
		"id": "1",
		"name": "local",
		"token": "xxx-xxx",
		"host": "test",
		"tags": ["ios", "mac"],
		"status": "OFFLINE"
	}`)

	var agent Agent
	err := json.Unmarshal(raw, &agent)

	assert.Nil(err)
	assert.Equal("1", agent.Id)
	assert.Equal("local", agent.Name)
	assert.Equal("xxx-xxx", agent.Token)
	assert.Equal("test", agent.Host)

	assert.Equal(2, len(agent.Tags))
	assert.Equal("ios", agent.Tags[0])
	assert.Equal("mac", agent.Tags[1])

	assert.Equal(AgentStatus("OFFLINE"), agent.Status)
}
