package config

import "errors"

var (
	ErrSettingsNotBeenLoaded = errors.New("agent: settings has not been initialized")
)
