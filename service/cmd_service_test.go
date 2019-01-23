package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/flowci/flow-agent-x/domain"

	"github.com/stretchr/testify/assert"

	"github.com/flowci/flow-agent-x/util"

	"github.com/flowci/flow-agent-x/config"
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
			"echo bbb",
			"sleep 5",
			">&2 echo $INPUT_VAR",
			"export FLOW_VVV=flowci",
			"export FLOW_AAA=flow...",
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

	config := config.GetInstance()
	config.Init()
}

func TestShouldReceiveExecutedCmdCallbackMessage(t *testing.T) {
	assert := assert.New(t)

	// init:
	config := config.GetInstance()
	defer config.Close()
	assert.True(config.HasQueue())

	// create queue consumer
	callbackQueue := config.Queue.CallbackQueue
	msgs, err := config.Queue.Channel.Consume(callbackQueue.Name, "test", true, false, false, false, nil)
	assert.Nil(err)

	block := make(chan domain.ExecutedCmd)

	go func() {
		for d := range msgs {
			var exectedCmd domain.ExecutedCmd
			err := json.Unmarshal(d.Body, &exectedCmd)
			assert.Nil(err)

			block <- exectedCmd
			util.LogDebug("Result of cmd %s been received", exectedCmd.ID)
		}
	}()

	// when: execute cmd
	cmdService := GetCmdService()
	err = cmdService.Execute(cmd)
	assert.Nil(err)

	// then: ensure result been executed and queue received
	select {
	case exectued := <-block:
		assert.Equal(cmd.ID, exectued.ID)
		assert.Equal(domain.CmdExitCodeSuccess, exectued.Code)
	case <-time.After(10 * time.Second):
		assert.Fail("timeout..")
	}
}
