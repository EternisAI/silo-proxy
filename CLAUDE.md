# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Silo Proxy is a gRPC-based reverse proxy that enables access to services behind NAT/firewalls without port forwarding or VPN configuration. The system uses agent-initiated outbound connections to create bidirectional gRPC streams, allowing the server to forward HTTP requests through these connections to services behind NAT.

**Core Architecture**: The agent behind NAT initiates a persistent gRPC connection to the cloud server. HTTP requests arrive at the server, are converted to ProxyMessages, sent over the gRPC stream to the agent, which forwards them to the local service, then returns responses through the same stream.

## Build & Run Commands

### Building
- `make build` - Build both server and agent binaries
- `make build-server` - Build server binary only (`bin/silo-proxy-server`)
- `make build-agent` - Build agent binary only (`bin/silo-proxy-agent`)
- `make clean` - Clean build artifacts and test cache

### Running
- `make run` - Start the proxy server (HTTP :8080, gRPC :9090)
- `make run-agent` - Start an agent that connects to the server

### Testing
- `make test` - Run all tests with verbose output
- `go test ./internal/grpc/server -v` - Run specific package tests

### Code Generation
- `make protoc-gen` - Regenerate gRPC/protobuf code from `proto/proxy.proto`
- `make generate-certs` - Generate TLS certificates for testing

### Docker
- `make docker-server` - Build server Docker image
- `make docker-agent` - Build agent Docker image
- `make docker-all` - Build both images

## Architecture

### Component Structure

```
cmd/
├── silo-proxy-server/  # Server entry point (HTTP + gRPC server)
├── silo-proxy-agent/   # Agent entry point (gRPC client + HTTP forwarder)

internal/
├── api/http/           # HTTP layer (routing, handlers, middleware)
│   ├── handler/        # HTTP handlers (proxy, health endpoints)
│   ├── middleware/     # HTTP middleware (logger, CORS)
│   └── dto/           # Data transfer objects
├── grpc/
│   ├── server/        # gRPC server (connection mgmt, stream handling)
│   ├── client/        # gRPC client (agent logic, request forwarding)
│   └── tls/          # TLS configuration utilities

proto/                 # Protocol buffer definitions
```

### Key Components

**Server Side (`cmd/silo-proxy-server/main.go`)**:
- HTTP server (Gin): Receives user HTTP requests, routes to proxy handler
- gRPC server: Manages bidirectional streams with agents
- ConnectionManager: Tracks connected agents and their streams
- Runs both servers concurrently with graceful shutdown coordination

**Agent Side (`cmd/silo-proxy-agent/main.go`)**:
- gRPC client: Initiates persistent connection to server, maintains stream
- RequestHandler: Forwards ProxyMessages to local service (e.g., localhost:3000)
- Auto-reconnect: Exponential backoff (1s → 30s) if connection drops

**ConnectionManager (`internal/grpc/server/connection_manager.go`)**:
- Maintains `map[agentID]*AgentConnection` with RWMutex protection
- Each AgentConnection has a buffered channel (SendCh) for queuing messages
- Automatic cleanup of stale connections (2min timeout, 30s cleanup interval)
- Thread-safe message sending with timeout (5s)

**Message Flow**:
1. User HTTP request → Gin handler (`internal/api/http/handler/proxy.go`)
2. Handler converts HTTP request to ProxyMessage (REQUEST type)
3. ProxyMessage sent to agent via ConnectionManager.SendToAgent()
4. Agent receives REQUEST, forwards to local service via HTTP
5. Agent converts HTTP response to ProxyMessage (RESPONSE type)
6. Response flows back through gRPC stream to server
7. Server returns HTTP response to original caller

### Protocol Buffer Message Types

The system uses 4 message types defined in `proto/proxy.proto`:
- **PING**: Agent → Server keep-alive heartbeat (every 30s)
- **PONG**: Server → Agent keep-alive response
- **REQUEST**: Server → Agent HTTP request forwarding
- **RESPONSE**: Agent → Server HTTP response return

Each ProxyMessage contains:
- `id`: Unique message ID (UUID)
- `type`: MessageType enum
- `payload`: Raw HTTP body bytes
- `metadata`: Map containing method, path, query, headers, status_code

### Routing

**Root Path Routing** (default):
- All requests to `http://localhost:8080/` route to `agent-1`
- Transparent passthrough without path manipulation

**Multi-Agent Routing**:
- `http://localhost:8080/proxy/:agent_id/*path`
- Example: `/proxy/agent-2/api/status` → forwards `/api/status` to agent-2
- Path prefix `/proxy/:agent_id` is stripped before forwarding

**Implementation**: `ProxyRootRequest()` hardcodes agent-1, `ProxyRequest()` extracts agent_id from URL parameter.

## Configuration

### Server Configuration (`cmd/silo-proxy-server/application.yml`)

```yaml
http:
  port: 8080          # HTTP server port
grpc:
  port: 9090          # gRPC server port
  tls:
    enabled: false    # Enable TLS for gRPC
    cert_file: ""
    key_file: ""
    ca_file: ""
    client_auth: ""   # Options: NoClientCert, RequestClientCert, RequireAnyClientCert
```

### Agent Configuration (`cmd/silo-proxy-agent/application.yml`)

```yaml
grpc:
  server_address: "localhost:9090"  # Server address to connect to
  agent_id: "agent-1"                # Unique agent identifier
  tls:
    enabled: false
    cert_file: ""
    key_file: ""
    ca_file: ""
    server_name_override: ""
local:
  service_url: "http://localhost:3000"  # Local service to proxy to
```

**Configuration Loading**: Uses Viper with environment variable override support. Config paths searched: `.` and `./cmd/silo-proxy-{server|agent}`. Environment variables use underscore separators (e.g., `GRPC_PORT`).

## Current Development Phase

The project is in the middle of implementing **per-agent listening ports** (see `docs/per-agent-ports/overview.md`). This is a breaking architectural change that allocates a unique HTTP port for each connected agent instead of path-based routing.

**Target Architecture**:
- Port 8080: Admin API (list agents, health checks)
- Ports 8100-8200: One dedicated port per agent (dynamic allocation)
- No path prefixes required when accessing agent ports

**Implementation Status**: All phases are on hold (⏸️). Key files to be added:
- `internal/api/http/port_manager.go` - Port pool management
- `internal/api/http/agent_server_manager.go` - HTTP server lifecycle per agent

**Integration Points**:
- `ConnectionManager.Register()`: Start HTTP server on new port
- `ConnectionManager.Deregister()`: Stop HTTP server and release port
- `AgentConnection` struct: Add `Port int` field

## TLS Support

TLS is supported for gRPC connections (server-agent communication). HTTP endpoints do not use TLS.

**Server TLS**:
- Mutual TLS supported via `client_auth` configuration
- Options: NoClientCert (server-only), RequestClientCert, RequireAnyClientCert, RequireAndVerifyClientCert

**Agent TLS**:
- Can verify server certificate via `ca_file`
- Supports `server_name_override` for testing with self-signed certs

**Certificate Generation**: Run `make generate-certs` to create test certificates using the script in `scripts/generate-certs.sh`.

## Testing Patterns

**Connection Manager Tests**:
- Mock gRPC streams using gomock or manual mocks
- Test concurrent operations (multiple agents connecting/disconnecting)
- Verify cleanup of stale connections

**Handler Tests**:
- Use `httptest.NewRecorder()` for Gin handlers
- Mock `grpcServer.GetConnectionManager()` and `ConnectionManager.GetConnection()`
- Test timeout scenarios and error paths

**Integration Tests**:
1. Start server and agent
2. Make HTTP request to server
3. Verify request reaches local service
4. Verify response returns correctly
5. Test disconnection and reconnection

## Common Patterns

### Error Handling
- All errors are logged with `slog` (structured logging)
- HTTP errors return JSON: `{"error": "message"}`
- gRPC stream errors trigger agent reconnection
- Connection failures use exponential backoff

### Graceful Shutdown
Both server and agent coordinate shutdown:
1. Catch SIGINT/SIGTERM signals
2. Stop accepting new requests
3. Use context with 10s timeout for HTTP server shutdown
4. Coordinate with WaitGroup for concurrent shutdowns
5. Log shutdown status at each stage

### Thread Safety
- ConnectionManager: RWMutex for agent map, buffered channels for message passing
- Message sending: Non-blocking with timeout and context cancellation
- Agent registration: Lock held during map modification, old connections cancelled

### Configuration Management
- Use Viper for YAML + environment variable support
- Config structs use mapstructure tags
- Initialize logger immediately after config load
- Print config as JSON only at DEBUG level

## Development Guidelines

### When Modifying gRPC Protocol
1. Edit `proto/proxy.proto`
2. Run `make protoc-gen` to regenerate Go code
3. Update both server stream handler and client stream handler
4. Maintain backward compatibility for deployed agents

### When Adding New Proxy Features
- HTTP changes: Update handlers in `internal/api/http/handler/`
- Agent logic: Modify `internal/grpc/client/request_handler.go`
- Connection management: Modify `internal/grpc/server/connection_manager.go`
- Always preserve metadata (headers) in ProxyMessage

### When Adding Configuration
- Add field to Config struct in `cmd/silo-proxy-{server|agent}/config.go`
- Add to `application.yml`
- Document in CLAUDE.md
- Consider environment variable override support
