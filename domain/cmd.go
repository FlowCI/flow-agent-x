package domain

type CmdType string

type CmdStatus string

const (
	CmdTypeShell CmdType = "SHELL"
	CmdTypeTty   CmdType = "TTY"
	CmdTypeKill  CmdType = "KILL"
	CmdTypeClose CmdType = "CLOSE"
)

const (
	CmdStatusPending   CmdStatus = "PENDING"
	CmdStatusRunning   CmdStatus = "RUNNING"
	CmdStatusSuccess   CmdStatus = "SUCCESS"
	CmdStatusSkipped   CmdStatus = "SKIPPED"
	CmdStatusException CmdStatus = "EXCEPTION"
	CmdStatusKilled    CmdStatus = "KILLED"
	CmdStatusTimeout   CmdStatus = "TIMEOUT"
)

var (
	shellOutInd = []byte{1}
	ttyOutInd   = []byte{2}
)

const (
	// CmdExitCodeUnknown default exit code
	CmdExitCodeUnknown = -1

	// CmdExitCodeTimeOut exit code for timeout
	CmdExitCodeTimeOut = -100

	// CmdExitCodeKilled exit code for killed
	CmdExitCodeKilled = -1

	// CmdExitCodeSuccess exit code for command executed successfully
	CmdExitCodeSuccess = 0
)

type (
	CmdIn struct {
		Type CmdType `json:"type"`
	}

	CmdOut interface {
		ToBytes() []byte
	}

	CmdStdLog struct {
		ID      string `json:"id"`
		Content string `json:"content"` // b64 content
	}
)
