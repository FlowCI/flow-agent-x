package api

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestShouldCacheFile(t *testing.T) {
	t.SkipNow()

	assert := assert.New(t)

	c := NewClient("277a35ad-30d7-47ea-a317-70670fb27306", "http://localhost:8080")

	jobId := "5f9935af5875dd0b92db014b"
	workspace := "/ws"
	cacheName := "test_cache"

	c.CachePut(jobId, cacheName, workspace, []string{
		"/Users/yang/Desktop/cache_1/test",
		"/Users/yang/Desktop/cache_2",
	})

	jobCache := c.CacheGet(jobId, cacheName)
	assert.NotNil(jobCache)

	c.CacheDownload(jobCache.Id, "/ws/out", "Y2FjaGVfMg==", nil)
	c.CacheDownload(jobCache.Id, "/ws/out", "Y2FjaGVfMS90ZXN0", nil)
}
