flow-agent-x
============

[![LICENSE](https://img.shields.io/github/license/pingcap/tidb.svg)](https://github.com/pingcap/tidb/blob/master/LICENSE)  

The new version agent for flow.ci

## How to start

- [Start from docker](https://github.com/FlowCI/docker)

- For more detail, please refer [doc](https://github.com/flowci/docs)

## Build binary

```bash
make build

# binary will be created at ./bin/flow-agent-x
```

## Run Unit Test

```bash
make run
```

## Build docker image
```bash
make docker

# docker image with name 'flowci/agent'
```