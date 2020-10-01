package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShouldFetchSystemResource(t *testing.T) {
	assert := assert.New(t)

	m := GetInstance()
	assert.NotNil(m)

	resource := m.FetchProfile()
	assert.NotNil(resource)

	assert.True(resource.Cpu > 0)
	assert.True(resource.TotalMemory > 0)
	assert.True(resource.FreeMemory > 0)
	assert.True(resource.TotalDisk > 0)
	assert.True(resource.FreeDisk > 0)
}
