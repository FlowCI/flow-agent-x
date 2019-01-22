package util

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	git "gopkg.in/src-d/go-git.v4"
)

var (
	ErrInvlidPluginUrl = errors.New("agent: invalid plugin repo url")
)

type Plugins struct {
	// file dir to store plugin
	dir string

	// server url
	server string
}

func NewPlugins(dir, server string) *Plugins {
	return &Plugins{
		dir:    dir,
		server: strings.TrimRight(server, "/"),
	}
}

func (p *Plugins) Load(name string) error {
	url := p.server + "/git/plugins/" + name
	dir := filepath.Join(p.dir, name)

	LogInfo("agent: clone plugin '%s' to '%s'", url, dir)

	_, err := git.PlainClone(dir, false, &git.CloneOptions{
		URL:      url,
		Progress: os.Stdout,
	})

	if err == git.ErrRepositoryAlreadyExists {
		return nil
	}

	return err
}
