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
		ID        string
		IsSuccess bool
		Error     string
	}

	// Shell log output
	TtyLog struct {
		ID  string
		Log string
	}
)
