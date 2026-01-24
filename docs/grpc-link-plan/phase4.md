# Phase 4: Keep-Alive Mechanism

**Status**: 🔲 Not Started

## Tasks

1. Agent sends PING every 30 seconds
2. Server responds with PONG
3. Both sides detect timeout after 90 seconds of silence

## Implementation

### Agent: Send PING
```go
func (c *Client) startPingLoop() {
    ticker := time.NewTicker(30 * time.Second)
    for range ticker.C {
        c.SendMessage(&proto.ProxyMessage{
            Type: proto.MessageType_PING,
        })
    }
}
```

### Server: Respond with PONG
```go
func (s *Server) handleMessage(msg *proto.ProxyMessage, conn *AgentConnection) {
    switch msg.Type {
    case proto.MessageType_PING:
        conn.lastSeen = time.Now()
        conn.Send(&proto.ProxyMessage{Type: proto.MessageType_PONG})
    }
}
```

### Timeout Detection (both sides)
```go
func startTimeoutChecker(timeout time.Duration) {
    ticker := time.NewTicker(30 * time.Second)
    for range ticker.C {
        if time.Since(lastSeen) > timeout {
            // Close connection and trigger reconnect
        }
    }
}
```

## Config Updates

**Server** (`application.yml`):
```yaml
grpc:
  port: 9090
  timeout_seconds: 90
```

**Agent** (`application.yml`):
```yaml
grpc:
  server_address: "localhost:9090"
  ping_interval_seconds: 30
  timeout_seconds: 90
```

## Verification

Watch logs for PING/PONG messages every 30 seconds:
```
Agent: [DEBUG] Sent PING
Server: [DEBUG] Received PING from agent-xxx
Agent: [DEBUG] Received PONG
```
