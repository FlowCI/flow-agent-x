FROM docker:20.10-cli as docker

FROM python:3.10.6-alpine3.16

RUN apk update
RUN apk add bash git curl wget

## docker v20.10.18 ##
COPY --from=docker /usr/local/bin/docker /usr/local/bin/docker

## docker compose v2.11.1 ##
COPY --from=docker /usr/libexec/docker/cli-plugins/docker-compose /usr/local/bin/docker-compose

## ssh config
RUN mkdir -p $HOME/.ssh
RUN echo "StrictHostKeyChecking=no" >> $HOME/.ssh/config

## upgrade pip
RUN pip install --upgrade pip

## install required pip packages
RUN python3 -m pip install requests==2.22.0 python-lib-flow.ci==1.21.6

## default work dir
ENV FLOWCI_AGENT_WORKSPACE=/ws
RUN mkdir -p $FLOWCI_AGENT_WORKSPACE

WORKDIR $FLOWCI_AGENT_WORKSPACE
COPY ./flow-agent-x-linux /usr/bin

ENV FLOWCI_DOCKER_AGENT=true

## start docker ##
CMD flow-agent-x-linux