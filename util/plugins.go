package util

import (
	"os"
	"path/filepath"
	"strings"

	git "gopkg.in/src-d/go-git.v4"
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

	err := p.clone(dir, url)

	if err == git.ErrRepositoryAlreadyExists {
		return p.pull(dir)
	}

	return err
}

func (p *Plugins) clone(dir ,url string) error {
	options := &git.CloneOptions{
		URL:      url,
		Progress: os.Stdout,
	}

	_, err := git.PlainClone(dir, false, options)
	return err
}

func (p *Plugins) pull(dir string) error {
	repo, err := git.PlainOpen(dir)
	if err != nil {
		return err
	}

	workTree, err := repo.Worktree()
	if err != nil {
		return err
	}

	err = workTree.Pull(&git.PullOptions{RemoteName: "origin"})
	if err == git.NoErrAlreadyUpToDate {
		return nil
	}

	return err
}
