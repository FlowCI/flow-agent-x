package service

import (
	"github/flowci/flow-agent-x/util"
	"os"
	"path/filepath"
	"strings"

	git "gopkg.in/src-d/go-git.v4"
)

type PluginManager struct {
	// file dir to store plugin
	dir string

	// server url
	server string
}

func NewPluginManager(dir, server string) *PluginManager {
	return &PluginManager{
		dir:    dir,
		server: strings.TrimRight(server, "/"),
	}
}

func (p *PluginManager) Load(name string) error {
	url := p.server + "/git/plugins/" + name
	dir := filepath.Join(p.dir, name)

	util.LogInfo("agent: clone plugin '%s' to '%s'", url, dir)

	err := p.clone(dir, url)

	if err == git.ErrRepositoryAlreadyExists {
		return p.pull(dir, url)
	}

	return err
}

func (p *PluginManager) clone(dir, url string) error {
	options := &git.CloneOptions{
		URL:      url,
		Progress: os.Stdout,
	}

	_, err := git.PlainClone(dir, false, options)
	return err
}

func (p *PluginManager) pull(dir, url string) (out error) {
	defer util.RecoverPanic(func(e error) {
		out = e
	})

	repo, err := git.PlainOpen(dir)
	util.PanicIfErr(err)

	workTree, err := repo.Worktree()
	util.PanicIfErr(err)

	// update remote url if url been changed
	remote, err := repo.Remote("origin")
	util.PanicIfErr(err)

	config := remote.Config()
	if config.URLs[0] != url {
		err = repo.DeleteRemote("origin")
		util.PanicIfErr(err)

		config.URLs[0] = url
		_, err = repo.CreateRemote(config)
		util.PanicIfErr(err)
	}

	err = workTree.Pull(&git.PullOptions{RemoteName: "origin"})
	if err == git.NoErrAlreadyUpToDate {
		return nil
	}

	return err
}
