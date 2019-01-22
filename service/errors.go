package service

import "errors"

var (
	ErrorCmdIsRunning       = errors.New("agent: cmd is running, service not available")
	ErrorCmdMissingScripts  = errors.New("agent: the cmd missing shell script")
	ErrorCmdUnsupportedType = errors.New("agent: unsupported cmd type")
)
