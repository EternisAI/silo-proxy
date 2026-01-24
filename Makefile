GO = $(shell which go 2>/dev/null)
DOCKER = $(shell which docker 2>/dev/null)

APP				:= silo-proxy
VERSION 		?= v0.1.0
LDFLAGS 		:= -ldflags "-X main.AppVersion=$(VERSION)"

.PHONY: all build clean test generate swagger docker

all: clean build

install:
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.29.0
	go install github.com/swaggo/swag/cmd/swag@v1.16.4
clean:
	$(GO) clean -testcache
	$(RM) -rf bin/*
build:
	$(GO) build -o bin/$(APP)	$(LDFLAGS) cmd/$(APP)/*.go
run:
	$(GO) run $(LDFLAGS) cmd/$(APP)/*.go
test:
	$(GO) test -v ./...
generate: install
	sqlc generate
	swag init -g cmd/$(APP)/main.go -o docs
docker:
	$(DOCKER) build --build-arg APP=$(APP) --build-arg VERSION=$(VERSION) -t $(APP):$(VERSION) -t $(APP):latest .
