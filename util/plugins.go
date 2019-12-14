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
		return p.pull(dir, url)
	}

	return err
}

func (p *Plugins) clone(dir, url string) error {
	options := &git.CloneOptions{
		URL:      url,
		Progress: os.Stdout,
	}

	_, err := git.PlainClone(dir, false, options)
	return err
}

func (p *Plugins) pull(dir, url string) (out error) {
	defer func() {
		if err := recover(); err != nil {
			out = err.(error)
		}
	}()

	repo, err := git.PlainOpen(dir)
	PanicIfErr(err)

	workTree, err := repo.Worktree()
	PanicIfErr(err)

	// update remote url if url been changed
	remote, err := repo.Remote("origin")
	PanicIfErr(err)

	config := remote.Config()
	if config.URLs[0] != url {
		err = repo.DeleteRemote("origin")
		PanicIfErr(err)

		config.URLs[0] = url
		_, err = repo.CreateRemote(config)
		PanicIfErr(err)
	}

	err = workTree.Pull(&git.PullOptions{RemoteName: "origin"})
	if err == git.NoErrAlreadyUpToDate {
		return nil
	}

	return err
}
