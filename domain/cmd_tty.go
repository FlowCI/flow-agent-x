package domain

import (
	"encoding/json"
)

const (
	TtyActionOpen  = "OPEN"
	TtyActionClose = "CLOSE"
	TtyActionShell = "SHELL"
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
)

// ===================================
//		TtyOut Methods
// ===================================

func (obj *TtyOut) ToBytes() []byte {
	data, _ := json.Marshal(obj)
	return append(ttyOutInd, data...)
}
