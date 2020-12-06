flow-agent-x
============
  
![GitHub](https://img.shields.io/github/license/flowci/flow-agent-x)
![GitHub release (latest by date)](https://img.shields.io/github/v/release/flowci/flow-agent-x)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/flowci/flow-agent-x)

The new version agent for flow.ci

## How to start

- [Start from docker](https://github.com/FlowCI/docker)

- For more detail, please refer [doc](https://github.com/flowci/docs)

## Build binary

```bash
make build

# binary will be created at ./bin/flow-agent-x-mac
# binary will be created at ./bin/flow-agent-x-linux
```

## Run Unit Test

```bash
make test
```

## Build docker image
```bash
make docker

# docker image with name 'flowci/agent'
```