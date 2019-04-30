package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"flow-agent-x/config"
	"flow-agent-x/domain"
	"flow-agent-x/util"

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
				"port": 5672,
				"username": "guest",
				"password": "guest"
			},
	
			"zookeeper": {
				"host": "127.0.0.1:2181",
				"root": "/flow-x"
			},
	
			"callbackQueueName": "callback-q-ut",
			"logsExchangeName": "logs-exchange-ut"
		}
	}`)

	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/agents/connect" {
			w.Write(rBody)
		}
	}))

	cmd = &domain.CmdIn{
		Cmd: domain.Cmd{
			ID: "1-1-1",
		},
		Scripts: []string{
			"set -e",
			"echo bbb 1",
			"sleep 2",
			">&2 echo $INPUT_VAR",
			"export FLOW_VVV=flowci",
			"export FLOW_AAA=flow...",
			"echo bbb 2",
		},
		Inputs:     domain.Variables{"INPUT_VAR": "aaa"},
		Timeout:    1800,
		EnvFilters: []string{"FLOW_"},
		Type:       domain.CmdTypeShell,
	}
)

func init() {
	util.EnableDebugLog()

	os.Setenv("FLOWCI_SERVER_URL", ts.URL)
	os.Setenv("FLOWCI_AGENT_TOKEN", "ca9b8be2-c0e5-4b86-8fdc-b92d921597a0")
	os.Setenv("FLOWCI_AGENT_PORT", "8081")
}

func TestShouldReceiveExecutedCmdCallbackMessage(t *testing.T) {
	assert := assert.New(t)

	// init:
	config := config.GetInstance()
	config.Init()

	defer config.Close()
	assert.True(config.HasQueue())

	// create queue consumer
	callbackQueue := config.Settings.CallbackQueueName
	ch := config.Queue.Channel
	ch.QueueDeclare(callbackQueue, false, true, false, false, nil)
	defer ch.QueueDelete(callbackQueue, false, false, true)

	msgs, err := ch.Consume(callbackQueue, "test", true, false, false, false, nil)
	assert.Nil(err)

	service := GetCmdService()
	err = service.Execute(cmd)
	assert.Nil(err)

	select {
	case m, _ := <-msgs:
		util.LogDebug("Result of cmd '%s' been received", m.Body)

		var r domain.ExecutedCmd
		err := json.Unmarshal(m.Body, &r)
		assert.Nil(err)

		assert.Equal(r.ID, cmd.ID)
		assert.Equal(domain.CmdStatusSuccess, r.Status)
	case <-time.After(10 * time.Second):
		assert.Fail("timeout..")
	}
}
