#!/usr/bin/env bash

# build within golang docker
docker run -it --rm \
-v "$PWD":/go/src/flow-agent-x \
-w /go/src/flow-agent-x golang:1.12 \
/bin/bash -c "curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh && dep ensure && go build"

docker build -f ./Dockerfile -t flowci/agent:latest .

# docker rmi -f $(docker images -f 'dangling=true' -q)