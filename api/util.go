package api

import (
	"encoding/base64"
	"github.com/flowci/flow-agent-x/util"
	"strings"
)

func buildMessage(event string, body []byte) (out []byte) {
	out = append([]byte(event), '\n')
	out = append(out, body...)
	return
}

func encodeCacheName(workspace, fullPath string) string {
	cacheName := util.TrimLeftString(fullPath, workspace)
	if strings.HasPrefix(cacheName, util.UnixPathSeparator) {
		cacheName = cacheName[1:]
	}
	return base64.StdEncoding.EncodeToString([]byte(cacheName))
}

func decodeCacheName(encodedFileName string) string {
	cacheName, err := base64.StdEncoding.DecodeString(encodedFileName)
	if err != nil {
		return ""
	}
	return string(cacheName)
}
