#!/usr/bin/env bash

docker pull flowci/pyenv:1.0
docker create volumn pyenv
docker run --rm -v pyenv:/ws pyenv1.0 ~/init-pyenv-volume.sh

export FLOWCI_AGENT_VOLUMES="name=pyenv,dest=/ci/.pyenv,script=init.sh;$FLOWCI_AGENT_VOLUMES"