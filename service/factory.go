package service

import (
	"github/flowci/flow-agent-x/config"
	"strings"
	"sync"
)

var (
	singleton *CmdService
	once      sync.Once
)

// GetCmdService get singleton of cmd service
func GetCmdService() *CmdService {
	appConfig := config.GetInstance()
	cmdIn := appConfig.Client.GetCmdIn()

	once.Do(func() {
		singleton = &CmdService{
			pluginManager: NewPluginManager(appConfig.PluginDir, appConfig.Server),
			cacheManager:  NewCacheManager(),
			cmdIn:         cmdIn,
		}
		singleton.start()
	})

	return singleton
}

func NewCacheManager() *CacheManager {
	return &CacheManager{}
}

func NewPluginManager(dir, server string) *PluginManager {
	return &PluginManager{
		dir:    dir,
		server: strings.TrimRight(server, "/"),
	}
}
