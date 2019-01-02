package domain

const (
	// LogTypeOut for stdout
	LogTypeOut = "STDOUT"

	// LogTypeErr for stderr
	LogTypeErr = "STDERR"
)

// LogType stdout or stderr
type LogType string

// LogItem line of log output
type LogItem struct {
	CmdID   string
	Type    LogType
	Content string
	Number  int64
}
