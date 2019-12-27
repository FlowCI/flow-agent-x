package config

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	rBody, _ = ioutil.ReadFile("../_testdata/agent_connect_response.json")

	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/agents/connect" {
			_, _ = w.Write(rBody)

		}
	}))
)

func TestShouldFetchSystemResource(t *testing.T)  {
	assert := assert.New(t)

	m := GetInstance()
	assert.NotNil(m)

	resource := m.FetchResource()
	assert.NotNil(resource)

	assert.True(resource.Cpu > 0)
	assert.True(resource.TotalMemory > 0)
	assert.True(resource.FreeMemory > 0)
	assert.True(resource.TotalDisk > 0)
	assert.True(resource.FreeDisk > 0)
}

func TestShouldConnectServerAndGetSettings(t *testing.T) {
	assert := assert.New(t)
	defer ts.Close()

	m := GetInstance()
	m.Server = ts.URL
	m.Token = "ca9b8be2-c0e5-4b86-8fdc-b92d921597a0"
	m.Port = 8081
	m.Init()
	defer m.Close()

	assert.NotNil(m.Settings)

	assert.Equal("1", m.Settings.Agent.ID)
	assert.Equal("xxx-xxx", m.Settings.Agent.Token)
}
