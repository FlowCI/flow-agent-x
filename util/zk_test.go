package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShouldCreateNode(t *testing.T) {
	assert := assert.New(t)

	client := new(ZkClient)
	defer client.Close()

	connErr := client.Connect("127.0.0.1:2181")
	assert.Nil(connErr)

	// create
	targetPath := "/flow-test"
	path, nodeErr := client.Create(targetPath, ZkNodeTypeEphemeral, "hello")
	assert.Nil(nodeErr)
	assert.Equal(targetPath, path)

	// exist
	exist, existErr := client.Exist(targetPath)
	assert.Nil(existErr)
	assert.True(exist)

	// get data
	data, dataErr := client.Data(targetPath)
	assert.Nil(dataErr)
	assert.Equal("hello", data)

	// delete
	delErr := client.Delete(targetPath)
	assert.Nil(delErr)
}
