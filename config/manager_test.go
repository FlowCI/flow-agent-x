package config

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	rBody = []byte(`{
		"code": 200,
		"message": "mock",
		"data": {
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
		}
	}`)

	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/agents/connect" {
			w.Write(rBody)
		}
	}))
)

func init() {
	os.Setenv("FLOWCI_SERVER_URL", ts.URL)
	os.Setenv("FLOWCI_AGENT_TOKEN", "ca9b8be2-c0e5-4b86-8fdc-b92d921597a0")
	os.Setenv("FLOWCI_AGENT_PORT", "8081")
}

func TestShouldConnectServerAndGetSettings(t *testing.T) {
	assert := assert.New(t)
	defer ts.Close()

	m := GetInstance()
	err := m.Connect()

	assert.Nil(err)
	assert.NotNil(m.Settings)

	assert.Equal("1", m.Settings.Agent.ID)
	assert.Equal("xxx-xxx", m.Settings.Agent.Token)
}
