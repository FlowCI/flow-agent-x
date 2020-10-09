package service

import "errors"

var (
	ErrorCmdIsRunning       = errors.New("agent: cmd is running, service not available")
	ErrorCmdUnsupportedType = errors.New("agent: unsupported cmd type")

	ErrorCmdScriptIsPersented     = errors.New("agent: the scripts should be empty for session open")
	ErrorCmdMissingSessionID      = errors.New("agent: the session id is required for cmd")
	ErrorCmdSessionNotFound       = errors.New("agent: session not found")
	ErrorCmdSessionMissingScripts = errors.New("agent: script is missing")
)
