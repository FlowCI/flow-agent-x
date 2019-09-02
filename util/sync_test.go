package util

import (
	"github.com/stretchr/testify/assert"
	"sync"
	"testing"
	"time"
)

func TestShouldWaitForSyncGroup(t *testing.T) {
	assert := assert.New(t)

	// init
	var g sync.WaitGroup
	g.Add(1)

	// when: start routing with 1s
	go func() {
		time.Sleep(1 * time.Second)
		g.Done()
	}()

	// then: should return true if not timeout
	r := Wait(&g, 5 * time.Second)
	assert.True(r)
}

func TestShouldTimeoutForSyncGroup(t *testing.T) {
	assert := assert.New(t)

	// init
	var g sync.WaitGroup
	g.Add(1)

	// when: start routing with 5s
	go func() {
		time.Sleep(5 * time.Second)
		g.Done()
	}()

	// then: should return false for timeout
	r := Wait(&g, 1 * time.Second)
	assert.False(r)
}
