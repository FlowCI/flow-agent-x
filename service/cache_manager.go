package service

import (
	"encoding/base64"
	"fmt"
	"github.com/dustin/go-humanize"
	"github/flowci/flow-agent-x/api"
	"github/flowci/flow-agent-x/domain"
	"github/flowci/flow-agent-x/util"
	"strings"
)

type CacheManager struct {
	client api.Client
}

type progressWriter struct {
	total  uint64
	client api.Client
	cmdIn  *domain.ShellIn
}

func (cm *CacheManager) Download(cmdIn *domain.ShellIn, jobDir string) {
	defer util.RecoverPanic(func(e error) {
		util.LogWarn(e.Error())
	})

	cache := cm.client.CacheGet(cmdIn.JobId, cmdIn.Cache.Key)
	sendLog(cm.client, cmdIn, fmt.Sprintf("Start to download cache.. %s", cache.Key))

	writer := &progressWriter{
		client: cm.client,
		cmdIn:  cmdIn,
	}

	for _, file := range cache.Files {
		sendLog(cm.client, cmdIn, fmt.Sprintf("---> cache %s", file))
		cm.client.CacheDownload(cache.Id, jobDir, file, writer)
	}

	sendLog(cm.client, cmdIn, "All cache file downloaded")
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n := len(p)
	pw.total += uint64(n)

	text := fmt.Sprintf("\r%s", strings.Repeat(" ", 50))
	sendLog(pw.client, pw.cmdIn, text)

	text = fmt.Sprintf("\rDownloading... %s complete", humanize.Bytes(pw.total))
	sendLog(pw.client, pw.cmdIn, text)

	return n, nil
}

func sendLog(client api.Client, cmdIn *domain.ShellIn, text string) {
	b64 := base64.StdEncoding.EncodeToString([]byte(text))
	client.SendShellLog(cmdIn.JobId, cmdIn.ID, b64)
}
