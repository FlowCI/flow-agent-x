package domain

import "fmt"

// LogItem line of log output
type LogItem struct {
	CmdID   string
	Content string
}

func (item LogItem) String() string {
	return fmt.Sprintf("%s#%s", item.CmdID, item.Content)
}
