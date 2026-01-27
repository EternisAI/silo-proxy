# Per-Agent Listening Ports Overview

## Progress

- ✅ **Phase 1**: Port Management Infrastructure (Complete - commit e13000e)
- ✅ **Phase 2**: Agent Server Management (Complete)
- ✅ **Phase 3**: ConnectionManager Integration (Complete)
- ⏸️ **Phase 4**: Admin API Implementation
- ⏸️ **Phase 5**: Configuration Updates
- ⏸️ **Phase 6**: Main Application Wiring
- ⏸️ **Phase 7**: Cleanup Obsolete Code

## Overview

Transform the silo-proxy-server architecture to allocate a unique HTTP listening port for each connected agent. Port 8080 will become an admin interface with endpoints to list all connected agents and their assigned ports.

## Detailed Phase Documentation

- [Phase 1: Port Management Infrastructure](./phase1.md) ✅
- [Phase 2: Agent Server Management](./phase2.md) ✅
- [Phase 3: ConnectionManager Integration](./phase3.md) ✅

## Motivation

Current architecture routes all proxy traffic through a single HTTP server on port 8080 using path-based routing (`/proxy/:agent_id/*path`). This proposal creates dedicated listening ports per agent for:

1. **Simplified routing**: Direct port-to-agent mapping eliminates path prefix requirements
2. **Better isolation**: Each agent gets its own HTTP server instance
3. **Easier monitoring**: Per-agent port enables independent monitoring and metrics
4. **Clear separation**: Admin functions (port 8080) separate from proxy traffic (agent ports)

## Architecture

### Current Architecture

```
User Request → HTTP Server :8080 → Route by /proxy/:agent_id → gRPC → Agent
                    ↓
              Admin: /health
```

### Proposed Architecture

```
User Request → HTTP Server :8100 → gRPC → Agent-1
User Request → HTTP Server :8101 → gRPC → Agent-2
User Request → HTTP Server :8102 → gRPC → Agent-3

Admin Request → HTTP Server :8080 → Admin API
                    ↓
        GET /agents (list all agents + ports)
        GET /health
```

## Design Principles

### 1. Port Allocation Strategy

**Decision**: Dynamic allocation from configurable port range

**Configuration** (`application.yml`):

```yaml
http:
  port: 8080
  agent_port_range:
    start: 8100
    end: 8200
```

**Rationale**:

- Predictable port usage (no OS ephemeral ports)
- Easy firewall configuration (fixed range)
- Configurable for different environments
- Automatic port reuse prevents exhaustion

**Implementation**:

- Buffered channel holds available ports
- Allocation: atomic channel receive
- Release: send back to channel
- Thread-safe by design

### 2. Server Management

**Decision**: One `http.Server` per agent + shared admin server

**Structure**:

- **Admin Server**: Single server on port 8080 with admin endpoints
- **Agent Servers**: One `http.Server` per connected agent
  - Minimal Gin engine per agent
  - No `agent_id` in routes (direct passthrough)
  - All paths route to that specific agent
  - CORS + logging middleware

**Server Lifecycle**:

```
Agent Connects:
1. ConnectionManager.Register() called
2. Allocate port from PortManager
3. Create new http.Server with Gin engine
4. Start server in goroutine
5. Store server reference

Agent Disconnects:
1. ConnectionManager.Deregister() called
2. Shutdown http.Server gracefully (5s timeout)
3. Release port back to PortManager
4. Remove server reference
```

### 3. Integration Approach

**Hook Points**:

- `ConnectionManager.Register()`: Start agent HTTP server
- `ConnectionManager.Deregister()`: Stop agent HTTP server
- `AgentConnection` struct: Add `Port` field

**Thread Safety**:

- PortManager: Channel-based (naturally thread-safe)
- AgentServerManager: RWMutex for server map
- ConnectionManager: Existing RWMutex maintained

## Architecture Components

### PortManager

**File**: `internal/api/http/port_manager.go`

**Responsibilities**:

- Manage pool of available ports
- Allocate ports atomically
- Track allocations (port → agent_id mapping)
- Release ports back to pool

**Interface**:

```go
type PortManager struct {
    availablePorts chan int
    allocatedPorts map[int]string  // port -> agent_id
    mu             sync.RWMutex
    rangeStart     int
    rangeEnd       int
}

func NewPortManager(start, end int) *PortManager
func (pm *PortManager) Allocate(agentID string) (int, error)
func (pm *PortManager) Release(port int)
func (pm *PortManager) GetAllocations() map[int]string
```

### AgentServerManager

**File**: `internal/api/http/agent_server_manager.go`

**Responsibilities**:

- Manage lifecycle of multiple HTTP servers
- Create/start agent servers on connection
- Stop/cleanup agent servers on disconnection
- Coordinate graceful shutdown

**Interface**:

```go
type AgentServerInfo struct {
    AgentID    string
    Port       int
    Server     *http.Server
    StartedAt  time.Time
}

type AgentServerManager struct {
    servers     map[string]*AgentServerInfo
    mu          sync.RWMutex
    portManager *PortManager
    grpcServer  *grpcserver.Server
    shutdownWg  sync.WaitGroup
}

func NewAgentServerManager(pm *PortManager, gs *grpcserver.Server) *AgentServerManager
func (asm *AgentServerManager) StartAgentServer(agentID string) (int, error)
func (asm *AgentServerManager) StopAgentServer(agentID string) error
func (asm *AgentServerManager) GetAllServers() []*AgentServerInfo
func (asm *AgentServerManager) Shutdown() error
```

## Error Handling

### Port Allocation Failures

**Scenario**: Port range exhausted (more agents than available ports)

**Handling**:

- `PortManager.Allocate()` returns error if channel empty
- `AgentServerManager.StartAgentServer()` propagates error
- `ConnectionManager.Register()` fails, agent connection rejected
- Log warning with details for monitoring

### Port Bind Failures

**Scenario**: Port already in use (unlikely with managed pool)

**Handling**:

- Retry allocation up to 3 times
- Each retry gets different port from pool
- After exhaustion, fail agent registration
- Log error with diagnostic information

### Graceful Shutdown

**Scenario**: Agent disconnects or server shuts down

**Handling**:

- Use `http.Server.Shutdown()` with 5-second context timeout
- If timeout exceeded, force `Close()`
- Always release port back to pool (even on forced close)
- Log cleanup status for monitoring

### Concurrent Operations

**Thread Safety**:

- **PortManager**: Channel-based allocation (naturally thread-safe)
- **AgentServerManager**: RWMutex for server map
- **ConnectionManager**: Existing RWMutex maintained

**Race Conditions Handled**:

1. Multiple agents connecting simultaneously → Channel serializes allocation
2. Agent disconnect during server startup → Context cancellation aborts startup
3. Shutdown during agent connect → Channels closed, operations fail gracefully

## Testing Strategy

### Unit Tests

**`port_manager_test.go`**:

- Test allocation from pool
- Test release back to pool
- Test exhaustion handling
- Test concurrent allocations (10 goroutines)
- Test allocation tracking

**`agent_server_manager_test.go`**:

- Test server lifecycle (start/stop)
- Test concurrent server operations
- Test port cleanup on stop
- Test graceful vs forced shutdown

### Integration Tests

**End-to-End Flow**:

1. Start server
2. Connect agent
3. Verify port allocated and logged
4. Verify HTTP server listening on allocated port
5. Make HTTP request to agent port
6. Verify request forwarded to agent
7. Disconnect agent
8. Verify port released and server stopped

**Admin API**:

1. Connect 3 agents
2. Call `GET /agents`
3. Verify response contains 3 agents with correct ports
4. Disconnect 1 agent
5. Call `GET /agents` again
6. Verify response contains 2 agents

**Port Exhaustion**:

1. Configure 3-port range (8100-8102)
2. Connect 3 agents successfully
3. Attempt to connect 4th agent
4. Verify connection rejected with appropriate error
5. Disconnect 1 agent
6. Retry 4th agent connection
7. Verify success with reused port

### Manual Testing

```bash
# 1. Start server with limited port range
# Edit application.yml: agent_port_range: {start: 8100, end: 8102}
make run

# 2. Start 3 agents
make run-agent  # terminal 1
make run-agent-2  # terminal 2
make run-agent-3  # terminal 3

# 3. Verify logs show port allocations
# Expected: "Agent registered agent_id=agent-1 port=8100"

# 4. Test admin API
curl http://localhost:8080/agents | jq

# 5. Test agent-specific ports
curl http://localhost:8100/api/status  # agent-1
curl http://localhost:8101/api/status  # agent-2
curl http://localhost:8102/api/status  # agent-3

# 6. Stop one agent (Ctrl+C in terminal 1)
# Verify log: "Agent deregistered agent_id=agent-1"

# 7. Verify admin API updated
curl http://localhost:8080/agents | jq
# Should show only 2 agents

# 8. Start new agent
make run-agent-4  # terminal 4
# Should reuse port 8100

# 9. Test graceful shutdown
kill -TERM <server-pid>
# Verify all servers stop cleanly
```

## Migration Guide

### Breaking Changes

⚠️ **This is an intentional breaking change to the architecture**

**Removed**:

- Route `/proxy/:agent_id/*path` no longer exists
- Port 8080 no longer accepts proxy traffic
- Direct agent routing via path is removed

**New Usage Pattern**:

1. Call `GET :8080/agents` to discover agent ports
2. Make requests directly to agent-specific ports
3. No path prefix required

**Example Migration**:

**Before**:

```bash
# All traffic through single port with path routing
curl http://localhost:8080/proxy/agent-1/api/status
curl http://localhost:8080/proxy/agent-2/api/status
```

**After**:

```bash
# Discover agent ports
curl http://localhost:8080/agents
# {
#   "agents": [
#     {"agent_id": "agent-1", "port": 8100, ...},
#     {"agent_id": "agent-2", "port": 8101, ...}
#   ]
# }

# Direct requests to agent ports
curl http://localhost:8100/api/status
curl http://localhost:8101/api/status
```

### Configuration Migration

**Old** (`application.yml`):

```yaml
http:
  port: 8080
```

**New** (`application.yml`):

```yaml
http:
  port: 8080 // admin port
  agent_port_range:
    start: 8100
    end: 8200
```

## Production Considerations

### Port Range Sizing

**Recommended**: 100 ports (default: 8100-8200)

**Factors**:

- Expected number of agents
- Growth headroom (2-3x current usage)
- Firewall rule complexity
- Port availability on host

**Example Configurations**:

- **Small deployment** (1-10 agents): 8100-8120 (20 ports)
- **Medium deployment** (10-50 agents): 8100-8200 (100 ports)
- **Large deployment** (50+ agents): 8100-8500 (400 ports)

### Firewall Configuration

Ensure firewall allows inbound connections to:

- Admin port: 8080 (TCP)
- Agent port range: 8100-8200 (TCP)
- gRPC port: 9090 (TCP)

**Example iptables**:

```bash
# Admin port
iptables -A INPUT -p tcp --dport 8080 -j ACCEPT

# Agent port range
iptables -A INPUT -p tcp --dport 8100:8200 -j ACCEPT

# gRPC port
iptables -A INPUT -p tcp --dport 9090 -j ACCEPT
```

### Monitoring

**Key Metrics to Monitor**:

- Active agent count
- Port pool utilization (allocated / total)
- Agent connection/disconnection rate
- HTTP request rate per agent port
- Agent server start/stop failures

**Logging**:

- Agent registration/deregistration (INFO level)
- Port allocation/release (INFO level)
- Server start/stop failures (ERROR level)
- Port exhaustion (WARN level)

### Security

**Admin API**:

- Consider adding authentication (API keys, JWT)
- Restrict admin port to internal network
- Rate limit admin endpoints

**Agent Ports**:

- Restrict access to known IP ranges
- Add authentication at application level

## Success Criteria

✅ **Phase 1 Complete**: Port allocation/release works correctly

✅ **Phase 2 Complete**: Agent servers start/stop on connect/disconnect

✅ **Phase 3 Complete**: Agents tracked with port numbers

✅ **Phase 4 Complete**: Admin API lists agents with ports

✅ **Phase 5 Complete**: Configuration migrated successfully

✅ **Phase 6 Complete**: Full integration in main application

✅ **Phase 7 Complete**: Obsolete code removed

✅ **Testing Complete**: All unit, integration, and manual tests pass

✅ **Documentation Updated**: README and usage examples reflect new architecture

## Future Enhancements

- No need for TLS support for agent HTTP servers
- Authentication/authorization for admin endpoints
- Metrics endpoint per agent (request counts, latency)
- Health check per agent port
- Static port mapping (configure specific ports per agent_id)
- WebSocket support on agent ports
- Request/response compression
- Agent port discovery via DNS SRV records
