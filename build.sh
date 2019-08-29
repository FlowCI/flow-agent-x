#!/usr/bin/env bash

dep ensure
go build -o flow-agent-x

docker build -f ./Dockerfile -t flowci/agent:latest .