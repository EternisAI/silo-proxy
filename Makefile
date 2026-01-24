GO = $(shell which go 2>/dev/null)
DOCKER = $(shell which docker 2>/dev/null)

SERVER			:= silo-proxy-server
AGENT			:= silo-proxy-agent
VERSION 		?= v0.1.0
LDFLAGS 		:= -ldflags "-X main.AppVersion=$(VERSION)"

.PHONY: all build build-server build-agent clean test generate swagger docker

all: clean build

install:
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.29.0
	go install github.com/swaggo/swag/cmd/swag@v1.16.4
clean:
	$(GO) clean -testcache
	$(RM) -rf bin/*
build: build-server build-agent
build-server:
	$(GO) build -o bin/$(SERVER) $(LDFLAGS) cmd/$(SERVER)/*.go
build-agent:
	$(GO) build -o bin/$(AGENT) $(LDFLAGS) cmd/$(AGENT)/*.go
run:
	$(GO) run $(LDFLAGS) cmd/$(SERVER)/*.go
test:
	$(GO) test -v ./...
generate: install
	sqlc generate
	swag init -g cmd/$(SERVER)/main.go -o docs
docker:
	$(DOCKER) build --build-arg APP=$(SERVER) --build-arg VERSION=$(VERSION) -t $(SERVER):$(VERSION) -t $(SERVER):latest .
