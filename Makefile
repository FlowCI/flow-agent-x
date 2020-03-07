PROJECT=flow-agent-x

LINUX_AMD64     := GOOS=linux GOARCH=amd64
MAC_AMD64       := GOOS=darwin GOARCH=amd64

GO		    	:= GO111MODULE=on go
GOBUILD_LINUX   := $(LINUX_AMD64) $(GO) build -o bin/$(PROJECT)-linux -v
GOBUILD_MAC     := $(MAC_AMD64) $(GO) build -o bin/$(PROJECT)-mac -v
GOTEST      	:= $(GO) test ./... -v

CURRENT_DIR 	:= $(shell pwd)
DOCKER_VERSION	:= golang:1.12
DOCKER_DIR 		:= /go/src/flow-agent-x
DOCKER_RUN 		:= docker run -it --rm -v $(CURRENT_DIR):$(DOCKER_DIR) -w $(DOCKER_DIR) $(DOCKER_VERSION) /bin/bash -c

DOCKER_BUILD 	:= ./build.sh

.PHONY: build test docker

build:
	$(DOCKER_RUN) "$(GOBUILD_LINUX) && $(GOBUILD_MAC)"

test:
	$(DOCKER_RUN) "$(GOTEST)"

docker: build
	$(DOCKER_BUILD) $(tag)