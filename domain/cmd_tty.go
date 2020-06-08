package domain

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
		IsSuccess bool   `json:"isSuccess"`
		Error     string `json:"error"`
	}

	// Shell log output
	TtyLog struct {
		ID      string `json:"id"`
		Content string `json:"content"`
	}
)
