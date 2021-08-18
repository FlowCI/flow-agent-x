PROJECT=flow-agent-x
CURRENT_DIR 	:= $(shell pwd)

LINUX_AMD64     := GOOS=linux GOARCH=amd64
MAC_AMD64       := GOOS=darwin GOARCH=amd64
WIN_AMD64       := GOOS=windows GOARCH=amd64

GO		    	:= go
GOGEN			:= $(GO) generate ./...
GOBUILD_LINUX   := $(LINUX_AMD64) $(GO) build -o bin/$(PROJECT)-linux -v
GOBUILD_MAC     := $(MAC_AMD64) $(GO) build -o bin/$(PROJECT)-mac -v
GOBUILD_WIN     := $(WIN_AMD64) $(GO) build -o bin/$(PROJECT)-win -v

GOTEST_MOCK_GEN := docker run --rm -v "$(CURRENT_DIR)":/src -w /src vektra/mockery --all
GOTEST      	:= $(GO) test ./... -v -timeout 10s
GOENV			:= -e GOCACHE=/ws/.cache -e GOPATH=/ws/.vender -e GO111MODULE=on

DOCKER_IMG		:= golang:1.13
DOCKER_RUN 		:= docker run -it --rm -v $(CURRENT_DIR):/ws $(GOENV) -w /ws --network host $(DOCKER_IMG) /bin/bash -c

DOCKER_BUILD 	:= ./build.sh

.PHONY: build protogen test image clean cleanall

build:
	$(DOCKER_RUN) "$(GOGEN) && $(GOBUILD_LINUX) && $(GOBUILD_MAC) && $(GOBUILD_WIN)"

test:
	$(GOTEST_MOCK_GEN)
	$(DOCKER_RUN) "$(GOTEST)"

image: build
	$(DOCKER_BUILD) $(tag)

clean:
	$(GO) clean -i ./...
	rm -rf bin

cleanall: clean
	rm -rf .cache
	rm -rf .vender
