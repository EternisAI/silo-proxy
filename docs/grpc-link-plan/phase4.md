# Phase 4: Keep-Alive Mechanism

**Status**: ✅ **COMPLETED** (Implemented in Phase 2 & 3)

## Tasks

1. ✅ Agent sends PING every 30 seconds
2. ✅ Server responds with PONG
3. ✅ Server detects stale connections (2-minute timeout)

## Implementation

### Agent: Send PING (`internal/grpc/client/client.go`)
```go
func (c *Client) pingLoop(done chan struct{}, errChan chan error) {
    ticker := time.NewTicker(pingInterval) // 30 seconds
    defer ticker.Stop()

    for {
        select {
        case <-done:
            return
        case <-ticker.C:
            ping := &proto.ProxyMessage{
                Id:       uuid.New().String(),
                Type:     proto.MessageType_PING,
                Metadata: map[string]string{},
            }
            c.Send(ping)
            slog.Debug("PING sent", "message_id", ping.Id)
        }
    }
}
```

### Server: Respond with PONG (`internal/grpc/server/stream_handler.go`)
```go
func (sh *StreamHandler) processMessage(agentID string, msg *proto.ProxyMessage) error {
    switch msg.Type {
    case proto.MessageType_PING:
        slog.Debug("PING received", "agent_id", agentID, "message_id", msg.Id)

        pong := &proto.ProxyMessage{
            Id:       uuid.New().String(),
            Type:     proto.MessageType_PONG,
            Metadata: map[string]string{},
        }

        sh.connManager.SendToAgent(agentID, pong)
        slog.Debug("PONG sent", "agent_id", agentID, "message_id", pong.Id)
    }
}
```

### Timeout Detection (`internal/grpc/server/connection_manager.go`)
```go
// Server-side stale connection cleanup
func (cm *ConnectionManager) cleanupStaleConnections() {
    ticker := time.NewTicker(cleanupInterval) // 30 seconds
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            now := time.Now()
            for agentID, conn := range cm.agents {
                if now.Sub(conn.LastSeen) > staleConnectionTimeout { // 2 minutes
                    slog.Warn("Removing stale connection", "agent_id", agentID)
                    conn.cancel()
                    close(conn.SendCh)
                    delete(cm.agents, agentID)
                }
            }
        }
    }
}
```

## Configuration

**Server** (`cmd/silo-proxy-server/application.yml`):
```yaml
grpc:
  port: 9090
# Hardcoded: staleConnectionTimeout = 2 minutes, cleanupInterval = 30 seconds
```

**Agent** (`cmd/silo-proxy-agent/application.yml`):
```yaml
grpc:
  server_address: localhost:9090
  agent_id: agent-1
# Hardcoded: pingInterval = 30 seconds
```

## Verification

**Completed**: 2026-01-25

Watch logs for PING/PONG messages every 30 seconds:
```
# Agent logs
time=2026-01-25T12:14:07.723+08:00 level=DEBUG msg="PING sent" message_id=d4be43f5-bf1f-415e-a22e-29c145cae6d2
time=2026-01-25T12:14:07.724+08:00 level=DEBUG msg="PONG received" message_id=867ac8dc-c309-46a8-b4bf-e84b9a028c24

# Server logs
time=2026-01-25T12:14:07.723+08:00 level=DEBUG msg="PING received" agent_id=agent-1 message_id=d4be43f5-bf1f-415e-a22e-29c145cae6d2
time=2026-01-25T12:14:07.724+08:00 level=DEBUG msg="PONG sent" agent_id=agent-1 message_id=867ac8dc-c309-46a8-b4bf-e84b9a028c24
```
