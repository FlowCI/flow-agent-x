FROM ubuntu:18.04

RUN apt update
RUN apt install git curl -y

## docker ##
RUN curl -L https://github.com/FlowCI/docker/releases/download/v0.20.9/docker-19_03_5 -o /usr/local/bin/docker \
    && chmod +x /usr/local/bin/docker \
    && ln -s /usr/local/bin/docker /usr/bin/docker

## docker compose ##
RUN curl -L "https://github.com/docker/compose/releases/download/1.24.1/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose \
    && chmod +x /usr/local/bin/docker-compose \
    && ln -s /usr/local/bin/docker-compose /usr/bin/docker-compose

## ssh config
RUN mkdir -p $HOME/.ssh
RUN echo "StrictHostKeyChecking=no" >> $HOME/.ssh/config

## default work dir
ENV FLOWCI_AGENT_WORKSPACE=/ws
RUN mkdir -p $FLOWCI_AGENT_WORKSPACE

WORKDIR $FLOWCI_AGENT_WORKSPACE
COPY ./bin/flow-agent-x-linux /usr/bin

## start docker ##
CMD service docker start && flow-agent-x-linux