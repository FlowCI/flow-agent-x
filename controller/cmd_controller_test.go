package controller

import (
	"github.com/streadway/amqp"
	"github/flowci/flow-agent-x/api"
	"github/flowci/flow-agent-x/config"
	"github/flowci/flow-agent-x/mocks"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestShouldInitCmdController(t *testing.T) {
	assert := assert.New(t)

	router := gin.Default()
	appConfig := config.GetInstance()
	appConfig.Client = mockClient()

	cmdController := NewCmdController(router)
	assert.NotNil(cmdController)
}

func mockClient() api.Client {
	mockClient := &mocks.Client{}
	deliveries := make(chan amqp.Delivery)
	mockClient.On("GetCmdIn").Return(<-deliveries, nil)
	return mockClient
}
