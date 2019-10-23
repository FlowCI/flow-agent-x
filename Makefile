PROJECT=flow-agent-x

GO		    := GO111MODULE=on go
GOBUILD     := $(GO) build -o bin/$(PROJECT) -v
GOTEST      := $(GO) test ./... -v
DOCKERBUILD := ./build.sh

.PHONY: build test docker

build:
	$(GOBUILD)

test:
	$(GOTEST)

docker:
	$(DOCKERBUILD) $(tag)