# gRPC Link Overview

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
  server_address: "proxy.example.com:9090"
```
