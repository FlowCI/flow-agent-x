package service

import (
	"github/flowci/flow-agent-x/api"
	"github/flowci/flow-agent-x/domain"
)

type CacheManager struct {
	client api.Client
}

func (cm *CacheManager) Download(jobId string, cache *domain.Cache) {

}
