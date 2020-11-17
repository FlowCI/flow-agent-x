package service

import (
	"encoding/base64"
	"fmt"
	"github.com/dustin/go-humanize"
	"github/flowci/flow-agent-x/api"
	"github/flowci/flow-agent-x/domain"
	"github/flowci/flow-agent-x/util"
	"io/ioutil"
	"path/filepath"
)

type CacheManager struct {
	client api.Client
}

type progressWriter struct {
	total  uint64
	client api.Client
	cmdIn  *domain.ShellIn
}

// Download download cache into a temp dir and return
func (cm *CacheManager) Download(cmdIn *domain.ShellIn) string {
	defer util.RecoverPanic(func(e error) {
		util.LogWarn(e.Error())
	})

	cm.Resolve(cmdIn)

	cache := cm.client.CacheGet(cmdIn.JobId, cmdIn.Cache.Key)
	sendLog(cm.client, cmdIn, fmt.Sprintf("Start to download cache.. %s", cache.Key))

	writer := &progressWriter{
		client: cm.client,
		cmdIn:  cmdIn,
	}

	cacheDir, err := ioutil.TempDir("", "cache_")
	util.PanicIfErr(err)

	for _, file := range cache.Files {
		sendLog(cm.client, cmdIn, fmt.Sprintf("---> cache %s", file))
		cm.client.CacheDownload(cache.Id, cacheDir, file, writer)
	}

	sendLog(cm.client, cmdIn, "All cached files downloaded")
	util.LogDebug("cache src file loaded at %s", cacheDir)
	return cacheDir
}

// Upload upload all files/dirs from cache dir
func (cm *CacheManager) Upload(cmdIn *domain.ShellIn, cacheDir string) {
	fileInfos, err := ioutil.ReadDir(cacheDir)
	if err != nil {
		util.LogWarn(err.Error())
		return
	}

	files := make([]string, len(fileInfos))
	for i, fileInfo := range fileInfos {
		files[i] = filepath.Join(cacheDir, fileInfo.Name())
	}

	cm.client.CachePut(cmdIn.JobId, cmdIn.Cache.Key, cacheDir, files)
}

// Resolve resolve env vars in cache key and paths
func (cm *CacheManager) Resolve(cmdIn *domain.ShellIn) {
	cache := cmdIn.Cache
	cache.Key = util.ParseStringWithSource(cache.Key, cmdIn.Inputs)

	for i, p := range cache.Paths {
		cache.Paths[i] = util.ParseStringWithSource(p, cmdIn.Inputs)
	}
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n := len(p)
	pw.total += uint64(n)

	text := fmt.Sprintf("Downloading... %s complete", humanize.Bytes(pw.total))
	sendLog(pw.client, pw.cmdIn, text)

	return n, nil
}

func sendLog(client api.Client, cmdIn *domain.ShellIn, text string) {
	b64 := base64.StdEncoding.EncodeToString([]byte(text + "\n"))
	client.SendShellLog(cmdIn.JobId, cmdIn.ID, b64)
}
