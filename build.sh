#!/usr/bin/env bash

version=$1

if [[ -n $version ]]; then
  VersionTag="-t flowci/agent:$version"
fi

docker build -f ./Dockerfile -t flowci/agent:latest $VersionTag ./bin

# docker rmi -f $(docker images -f 'dangling=true' -q)