# gRPC Link Overview

## Progress

- ✅ **Phase 1**: Basic gRPC Setup (protobuf definitions)
- ✅ **Phase 2**: Server Implementation (gRPC server, stream handler, connection manager)
- ✅ **Phase 3**: Agent Implementation (gRPC client, reconnection, graceful shutdown)
- ✅ **Phase 4**: Keep-Alive Mechanism (PING/PONG, stale connection detection)
- 🔲 **Phase 5**: Request Forwarding (HTTP → gRPC → HTTP)

## Architecture

```
User → Server (Cloud) ←[gRPC]→ Agent (Silo Box) → Local Services
        - HTTP:8080           - HTTP:8081
        - gRPC:9090           - gRPC Client
```

## Design Principles

1. **Simple**: No TLS/mTLS for now
2. **Agent-Initiated**: Agent connects to server (works through NAT)
3. **Bidirectional Stream**: Single persistent connection
4. **Separation of Concerns**: gRPC layer separate from HTTP layer

## Message Types

- **PING**: Agent → Server (keep-alive)
- **PONG**: Server → Agent (keep-alive response)
- **REQUEST**: Server → Agent (forward HTTP request)
- **RESPONSE**: Agent → Server (return HTTP response)

## Configuration

**Server** (`cmd/silo-proxy-server/application.yml`):
```yaml
grpc:
  port: 9090
```

**Agent** (`cmd/silo-proxy-agent/application.yml`):
```yaml
grpc:
  server_address: "localhost:9090"
  agent_id: "agent-1"
```

## Current Status (2026-01-25)

### What's Working

✅ **Bidirectional gRPC Connection**
- Agent initiates connection to server
- Persistent streaming connection maintained
- Thread-safe stream management

✅ **Automatic Reconnection**
- Exponential backoff: 1s → 2s → 4s → 8s → 16s → 30s max
- Automatic recovery from network interruptions
- Clean error handling and logging

✅ **Keep-Alive Mechanism**
- Agent sends PING every 30 seconds
- Server responds with PONG
- Server removes stale connections after 2 minutes

✅ **Graceful Shutdown**
- Signal handling (SIGINT/SIGTERM)
- Coordinated shutdown of HTTP and gRPC
- 10-second timeout for graceful shutdown

### Next Steps

🔲 **Phase 5: Request Forwarding**
- HTTP request → gRPC REQUEST message
- Forward to agent via stream
- Agent forwards to local service
- Return HTTP response to user
