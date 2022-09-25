#!/usr/bin/env bash

version=$1

if [[ -n $version ]]; then
  VersionTag="-t flowci/agent:$version"
fi

# docker run --privileged --rm tonistiigi/binfmt --install all
# docker buildx create --name flowci --use

docker buildx build -f ./Dockerfile --platform linux/arm64,linux/amd64 --push -t flowci/agent:latest $VersionTag ./bin

# docker rmi -f $(docker images -f 'dangling=true' -q)