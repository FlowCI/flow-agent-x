package config

import (
	"github.com/flowci/flow-agent-x/domain"
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
			status: domain.AgentIdle,
			events: map[domain.AppEvent]func(){},
		}
	})
	return singleton
}
