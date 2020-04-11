package service

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github/flowci/flow-agent-x/config"
	"github/flowci/flow-agent-x/domain"
	"github/flowci/flow-agent-x/util"

	"github.com/stretchr/testify/assert"
)

var (
	rBody, _ = ioutil.ReadFile("../_testdata/agent_connect_response.json")

	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/agents/connect" {
			_, _ = w.Write(rBody)
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
}

func TestShouldReceiveExecutedCmdCallbackMessage(t *testing.T) {
	assert := assert.New(t)

	// init:
	config := config.GetInstance()
	config.Server = ts.URL
	config.Token = "ca9b8be2-c0e5-4b86-8fdc-b92d921597a0"
	config.Port = 8081
	config.Init()

	defer config.Close()
	assert.True(config.HasQueue())
	assert.NotNil(config.Queue)

	// create queue consumer
	callbackQueue := config.Settings.Queue.Callback
	ch := config.Queue.Channel
	_, _ = ch.QueueDeclare(callbackQueue, false, true, false, false, nil)
	defer func() {
		_, err := ch.QueueDelete(callbackQueue, false, false, true)
		assert.NoError(err)
	}()

	msgs, err := ch.Consume(callbackQueue, "test", true, false, false, false, nil)
	assert.Nil(err)

	service := GetCmdService()
	err = service.Execute(cmd)
	assert.Nil(err)

	select {
	case m, ok := <-msgs:
		if !ok {
			return
		}

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
