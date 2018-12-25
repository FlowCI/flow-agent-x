package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func init() {
	os.Setenv("FLOWCI_SERVER_URL", "http://localhost:8080")
	os.Setenv("FLOWCI_AGENT_TOKEN", "ca9b8be2-c0e5-4b86-8fdc-b92d921597a0")
	os.Setenv("FLOWCI_AGENT_PORT", "8081")
}

func TestShouldConnectServerAndGetSettings(t *testing.T) {
	assert := assert.New(t)

	configManager := Manager{}
	settings, err := configManager.Connect()

	assert.Nil(err)
	assert.NotNil(settings)
}
