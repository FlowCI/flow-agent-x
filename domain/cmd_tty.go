package domain

import (
	"bytes"
	"encoding/json"
)

const (
	TtyActionOpen  = "OPEN"
	TtyActionClose = "CLOSE"
	TtyActionShell = "SheLL"
)

type (
	TtyIn struct {
		CmdIn
		ID     string `json:"id"`
		Action string `json:"action"`
		Input  string `json:"input"`
	}

	// Open, Close control action response
	TtyOut struct {
		ID        string `json:"id"`
		Action    string `json:"action"`
		IsSuccess bool   `json:"success"`
		Error     string `json:"error"`
	}

	// Shell log output
	TtyLog struct {
		ID      string `json:"id"`
		Content []byte `json:"content"`
	}
)

// ===================================
//		TtyOut Methods
// ===================================

func (obj *TtyOut) ToBytes() []byte {
	data, _ := json.Marshal(obj)
	return append(ttyOutInd, data...)
}

// ===================================
//		TtyLog Methods
// ===================================

// format: {ind}{length of id}003{cmd id}003{content}
func (log *TtyLog) ToBytes(buffer *bytes.Buffer) []byte {
	buffer.Write(ttyOutInd)

	i := len(log.ID)
	buffer.WriteByte(uint8(i))
	buffer.WriteByte(logSeparator)

	buffer.WriteString(log.ID)
	buffer.WriteByte(logSeparator)

	buffer.Write(log.Content)
	return buffer.Bytes()
}
