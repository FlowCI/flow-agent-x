PROJECT=flow-agent-x

LINUX_AMD64     := GOOS=linux GOARCH=amd64
MAC_AMD64       := GOOS=darwin GOARCH=amd64

CURRENT_DIR 	:= $(shell pwd)
DOCKER_IMG		:= flowci/gosdk:1.0
DOCKER_DIR 		:= /ws
DOCKER_RUN 		:= docker run -it --rm -v $(CURRENT_DIR):$(DOCKER_DIR) -w $(DOCKER_DIR) --network host $(DOCKER_IMG) /bin/bash -c

GO		    	:= go
GOGEN			:= $(GO) generate ./...
GOBUILD_LINUX   := $(LINUX_AMD64) $(GO) build -o bin/$(PROJECT)-linux -v
GOBUILD_MAC     := $(MAC_AMD64) $(GO) build -o bin/$(PROJECT)-mac -v
GOTEST      	:= $(GO) test ./... -v
GOENV			:= GOCACHE=$(DOCKER_DIR)/.cache GOPATH=$(DOCKER_DIR)/.vender GO111MODULE=on

DOCKER_BUILD 	:= ./build.sh

.PHONY: build protogen test docker

build:
	$(DOCKER_RUN) "$(GOENV) && $(GOGEN) && $(GOBUILD_LINUX) && $(GOBUILD_MAC)"

test:
	$(DOCKER_RUN) "$(GOTEST)"

docker: build
	$(DOCKER_BUILD) $(tag)