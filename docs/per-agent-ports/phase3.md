# Phase 3: ConnectionManager Integration

**Status**: ✅ Complete

## Overview

Integrated `AgentServerManager` into `ConnectionManager` so per-agent HTTP servers are automatically started when agents connect and stopped when they disconnect.

## Implementation

### AgentServerManager Interface

Created interface to avoid circular dependencies:

```go
type AgentServerManager interface {
    StartAgentServer(agentID string) (int, error)
    StopAgentServer(agentID string) error
    Shutdown() error
}
```

### AgentConnection Enhancement

Added `Port` field:

```go
type AgentConnection struct {
    ID       string
    Port     int // HTTP server port (0 if no dedicated server)
    Stream   proto.ProxyService_StreamServer
    SendCh   chan *proto.ProxyMessage
    LastSeen time.Time
    ctx      context.Context
    cancel   context.CancelFunc
}
```

### ConnectionManager Updates

#### Constructor
```go
func NewConnectionManager(agentServerManager AgentServerManager) *ConnectionManager
```
- Parameter is optional (can be nil)
- Backward compatible

#### Register()
- Start HTTP server if manager available
- Assign port to AgentConnection
- Fail registration if server start fails

#### Deregister()
- Stop HTTP server if present
- Release port back to pool
- Log errors but continue cleanup

#### Stop()
- Stop all agent servers
- Call manager Shutdown()

#### removeStaleConnections()
- Stop servers for stale agents (>2 min timeout)

## Testing

22 test cases covering:
- Registration with/without server manager
- Error handling
- Duplicate agents
- Stale cleanup
- Concurrent operations (10 agents)
- Port field persistence

**Result**: All tests passing ✅

## Changes

**Modified**:
- `internal/grpc/server/connection_manager.go`
- `internal/grpc/server/server.go` (passes nil for now)

**New**:
- `internal/grpc/server/connection_manager_test.go` (471 lines)

## Next Steps

**Phase 4**: Admin API - Create `/agents` endpoint to list connected agents with ports
