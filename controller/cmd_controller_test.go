package controller

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestShouldInitCmdController(t *testing.T) {
	assert := assert.New(t)

	router := gin.Default()

	cmdController := NewCmdController(router)
	assert.NotNil(cmdController)
}
