package config

import (
	"github/flowci/flow-agent-x/domain"
	"sync"
)

var (
	singleton *Manager
	once      sync.Once
)

// GetInstance get singleton of config manager
func GetInstance() *Manager {
	once.Do(func() {
		singleton = &Manager{
			Status: domain.AgentIdle,
		}
	})
	return singleton
}
