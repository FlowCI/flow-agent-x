package domain

import (
	"bytes"
)

const (
	logSeparator = '\003'
)

type LogItem struct {
	CmdId   string
	Content []byte
}

// format: {length of id}003{cmd id}003{content}
func (log *LogItem) Write(buffer *bytes.Buffer) []byte {
	i := len(log.CmdId)
	buffer.WriteByte(uint8(i))
	buffer.WriteByte(logSeparator)

	buffer.WriteString(log.CmdId)
	buffer.WriteByte(logSeparator)

	buffer.Write(log.Content)
	return buffer.Bytes()
}
