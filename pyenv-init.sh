#!/usr/bin/env bash

docker volume create pyenv
docker run --rm -v pyenv:/ws flowci/pyenv:1.0 bash -c "~/init-pyenv-volume.sh"

export FLOWCI_AGENT_VOLUMES="name=pyenv,dest=/ci/python,script=init.sh;$FLOWCI_AGENT_VOLUMES"