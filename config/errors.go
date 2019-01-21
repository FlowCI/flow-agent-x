package config

import "errors"

var (
	ErrSettingsNotBeenLoaded = errors.New("The agent settings not been initialized")
)
