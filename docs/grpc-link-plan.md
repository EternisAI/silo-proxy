# gRPC Link Implementation Plan

## Architecture Overview

```
User → Silo Proxy Server (Cloud) ←[gRPC]→ Proxy Agent (Silo Box) → Local Services
        - HTTP Server (8080)                  - HTTP Server (8081)
        - gRPC Server (9090)                  - gRPC Client
```

## Design Principles

1. **Simple**: No TLS/mTLS for now, focus on basic communication
2. **Agent-Initiated**: Proxy Agent connects to Proxy Server (works through NAT/firewall)
3. **Bidirectional Stream**: Single persistent connection for both directions
4. **Separation of Concerns**: gRPC layer separate from HTTP layer

## Components

### 1. Proto Definition (`proto/proxy.proto`)

```protobuf
service ProxyService {
  rpc Stream(stream ProxyMessage) returns (stream ProxyMessage);
}

message ProxyMessage {
  string id = 1;
  MessageType type = 2;
  bytes payload = 3;
  map<string, string> metadata = 4;
}

enum MessageType {
  PING = 1;      // Keep-alive from agent
  PONG = 2;      // Keep-alive response from server
  REQUEST = 3;   // Server sends request to agent
  RESPONSE = 4;  // Agent sends response to server
}
```

### 2. Silo Proxy Server (Cloud)

**gRPC Server Side:**
- Listen on port 9090 (configurable)
- Accept bidirectional stream from agent
- Handle incoming messages (PING, RESPONSE)
- Send outgoing messages (PONG, REQUEST)

**HTTP Server Side:**
- Listen on port 8080 (existing)
- Receive user requests
- Forward to agent via gRPC stream
- Wait for response and return to user

**Directory Structure:**
```
cmd/silo-proxy-server/
├── main.go                    # Start both HTTP and gRPC servers
├── config.go                  # Add gRPC config
├── logger.go
└── application.yml            # Add grpc.port: 9090

internal/grpc/
├── server/
│   ├── server.go              # gRPC server setup
│   └── stream_handler.go      # Handle bidirectional stream
└── client/
    └── manager.go             # Manage connected agents
```

### 3. Proxy Agent (Silo Box)

**gRPC Client Side:**
- Connect to server at startup
- Establish bidirectional stream
- Send PING messages periodically
- Listen for incoming REQUEST messages
- Forward requests to local services
- Send RESPONSE back to server

**HTTP Server Side:**
- Listen on port 8081 (existing, for local debugging)
- Health check endpoint

**Directory Structure:**
```
cmd/silo-proxy-agent/
├── main.go                    # Start HTTP server and gRPC client
├── config.go                  # Add server address config
├── logger.go
└── application.yml            # Add grpc.server_address

internal/grpc/
└── client/
    ├── client.go              # gRPC client setup
    └── stream_handler.go      # Handle bidirectional stream
```

## Implementation Steps

### Phase 1: Basic gRPC Setup
1. Create proto definition
2. Generate Go code from proto
3. Add gRPC dependencies to go.mod
4. Update Makefile for proto generation

### Phase 2: Server Implementation
1. Create gRPC server in `internal/grpc/server/`
2. Implement bidirectional stream handler
3. Add connection manager to track connected agents
4. Update server main.go to start gRPC server alongside HTTP
5. Update server config to include gRPC port

### Phase 3: Agent Implementation
1. Create gRPC client in `internal/grpc/client/`
2. Implement bidirectional stream handler
3. Add reconnection logic (retry with backoff)
4. Update agent main.go to start gRPC client on startup
5. Update agent config to include server address

### Phase 4: Keep-Alive Mechanism
1. Agent sends PING every 30 seconds
2. Server responds with PONG
3. Both sides close connection if no message received for 90 seconds

### Phase 5: Request Forwarding (Next Phase)
1. Server accepts HTTP request from user
2. Server generates REQUEST message with unique ID
3. Server sends REQUEST to agent via stream
4. Agent receives REQUEST, forwards to local service
5. Agent sends RESPONSE back to server
6. Server returns response to user

## Configuration

### Server (`application.yml`)
```yaml
log:
  level: debug
http:
  port: 8080
grpc:
  port: 9090
```

### Agent (`application.yml`)
```yaml
log:
  level: debug
http:
  port: 8081
grpc:
  server_address: "proxy.example.com:9090"  # Cloud server address
```

## Dependencies

Add to `go.mod`:
```
google.golang.org/grpc
google.golang.org/protobuf
```

## Testing Plan

### Unit Tests
- Test message serialization/deserialization
- Test connection manager add/remove

### Integration Tests
1. Start server and agent
2. Verify agent connects successfully
3. Verify PING/PONG messages
4. Test connection recovery after disconnect

### Manual Testing
1. Run server: `./bin/silo-proxy-server`
2. Run agent: `./bin/silo-proxy-agent`
3. Check logs for connection establishment
4. Check logs for PING/PONG messages

## Out of Scope (For Now)

- TLS/mTLS encryption
- Authentication/authorization
- Multiple concurrent requests (will add later)
- Request timeout handling (will add later)
- Metrics and monitoring (will add later)
- Load balancing multiple agents (will add later)

## Success Criteria

- [ ] Proto file defined and generates Go code
- [ ] Server starts gRPC server on port 9090
- [ ] Agent connects to server successfully
- [ ] Bidirectional stream established
- [ ] PING/PONG messages flow correctly
- [ ] Connection survives for extended period
- [ ] Agent reconnects after disconnect

## Next Steps After This

Once the gRPC link is stable:
1. Implement HTTP request forwarding
2. Add request/response correlation (using message ID)
3. Add timeout handling
4. Add proper error handling
5. Consider adding TLS for production
