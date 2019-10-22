PROJECT=flow-agent-x

GO		:= GO111MODULE=on go
GOBUILD := $(GO) build -o bin/$(PROJECT) -v
GOTEST := $(GO) test ./... -v

.PHONY: build test

build:
	$(GOBUILD)

test:
	$(GOTEST)