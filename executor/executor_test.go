package executor

import (
	"github/flowci/flow-agent-x/domain"
	"github/flowci/flow-agent-x/util"
	"path"
	"runtime"
)

func printLog(channel <-chan *domain.LogItem) {
	for {
		item, ok := <-channel
		if !ok {
			break
		}
		util.LogDebug("[LOG]: %s", item.Content)
	}
}
func getTestDataDir() string {
	_, filename, _, _ := runtime.Caller(0)
	base := path.Dir(filename)
	return path.Join(base, "_testdata")
}