# Phase 2: Agent Server Management

**Status**: ✅ Complete

## Overview

Implemented `AgentServerManager` component that manages the lifecycle of per-agent HTTP servers. Each connected agent gets its own dedicated HTTP server listening on a unique port allocated from the `PortManager`.

## Implementation

### AgentServerManager

**File**: `internal/api/http/agent_server_manager.go`

```go
type AgentServerInfo struct {
    AgentID   string
    Port      int
    Server    *http.Server
    StartedAt time.Time
}

type AgentServerManager struct {
    servers     map[string]*AgentServerInfo
    mu          sync.RWMutex
    portManager *PortManager
    grpcServer  *grpcserver.Server
}
```

**Key Methods**:
- `NewAgentServerManager(pm, gs)` - Creates new manager
- `StartAgentServer(agentID)` - Allocates port and starts HTTP server
- `StopAgentServer(agentID)` - Gracefully stops server (5s timeout) and releases port
- `GetAllServers()` - Returns snapshot of running servers
- `Shutdown()` - Stops all servers

### ProxyHandler Extension

Added `ProxyRequestDirect(c *gin.Context, agentID string)` method to forward requests directly to a specific agent without requiring agent_id in the URL path.

### Server Lifecycle

**Agent Connects:**
1. Allocate port from PortManager
2. Create minimal Gin engine (logging + recovery + NoRoute handler)
3. Start http.Server in goroutine
4. Store server info

**Agent Disconnects:**
1. Graceful shutdown with 5s timeout
2. Force close if timeout exceeded
3. Release port back to PortManager

### Error Handling

- **Port exhaustion**: Return error, reject agent connection
- **Port bind failure**: Up to 3 retries with different ports
- **Graceful shutdown**: 5s timeout, then force close
- **Port always released**: Even on errors

## Testing

Comprehensive test suite with 89.5% code coverage:
- Server lifecycle (start/stop)
- Concurrent operations (10 servers)
- Port exhaustion handling
- Graceful vs forced shutdown
- Port reuse after release
- Bulk shutdown

**Result**: All tests passing ✅

## Changes

**New Files**:
- `internal/api/http/agent_server_manager.go` (234 lines)
- `internal/api/http/agent_server_manager_test.go` (397 lines)

**Modified Files**:
- `internal/api/http/handler/proxy.go` - Added `ProxyRequestDirect` method

## Next Steps

**Phase 3**: ConnectionManager Integration - Hook into Register/Deregister lifecycle
