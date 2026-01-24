GO = $(shell which go 2>/dev/null)
DOCKER = $(shell which docker 2>/dev/null)
PROTOC = $(shell which protoc 2>/dev/null || echo $(HOME)/.local/bin/protoc)

SERVER			:= silo-proxy-server
AGENT			:= silo-proxy-agent
VERSION 		?= v0.1.0
LDFLAGS 		:= -ldflags "-X main.AppVersion=$(VERSION)"

.PHONY: all build build-server build-agent clean test generate docker protoc protoc-gen

all: clean build

install:
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.29.0
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
docker:
	$(DOCKER) build --build-arg APP=$(SERVER) --build-arg VERSION=$(VERSION) -t $(SERVER):$(VERSION) -t $(SERVER):latest .

protoc:
ifeq ($(PROTOC_GEN_GO),)
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
endif
ifeq ($(PROTOC_GEN_GO_GRPC),)
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
endif

protoc-gen: protoc
	$(PROTOC) \
	  --proto_path=proto \
	  --go_out=proto \
	  --go_opt=paths=source_relative \
	  --go-grpc_out=proto \
	  --go-grpc_opt=paths=source_relative \
	  proto/proxy.proto
