package service

import "errors"

var (
	ErrorCmdIsRunning       = errors.New("There has cmd running, service not available")
	ErrorCmdMissingScripts  = errors.New("The cmd missing shell script")
	ErrorCmdUnsupportedType = errors.New("Unsupported cmd type")
)
