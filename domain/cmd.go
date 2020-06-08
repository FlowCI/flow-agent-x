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
	DockerOption struct {
		Image             string   `json:"image"`
		Entrypoint        []string `json:"entrypoint"` // host:container
		Ports             []string `json:"ports"`
		NetworkMode       string   `json:"networkMode"`
		User              string   `json:"user"`
		IsStopContainer   bool     `json:"isStopContainer"`
		IsDeleteContainer bool     `json:"isDeleteContainer"`
	}

	CmdIn struct {
		Type CmdType `json:"type"`
	}
)
