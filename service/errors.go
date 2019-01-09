package service

import "errors"

var (
	ErrorCmdMissingScripts  = errors.New("The cmd missing shell script")
	ErrorCmdUnsupportedType = errors.New("Unsupported cmd type")
)
