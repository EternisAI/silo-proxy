# Key-Based Agent Provisioning System

## Overview

This document describes the key-based agent provisioning system for Silo Proxy. The system enables secure, user-scoped agent registration through temporary, single-use provisioning keys.

## Problem Statement

**Current Issues:**
- Agents use static `agent_id` with no authentication/authorization
- Any agent can claim any ID (security risk)
- No user ownership tracking (prevents multi-tenant dashboards)

## Solution

The provisioning system works as follows:

1. Dashboard users generate temporary, single-use provisioning keys
2. Users copy keys to client devices
3. Agents connect with key, receive permanent agent_id (UUID) + optional TLS cert
4. Future connections validated against database
5. All agents associated with user accounts

## Architecture

### Database Schema

#### provisioning_keys Table
- Stores SHA-256 hashes of provisioning keys (never plaintext)
- Single-use by default (`max_uses=1`)
- Time-based expiration (24-48 hours recommended)
- Status lifecycle: active → exhausted/expired/revoked

#### agents Table
- `id` (UUID) IS the agent identifier (no separate agent_id column)
- Tracks user ownership and provisioning key used
- Stores connection metadata and status

#### agent_connection_logs Table
- Audit trail for all agent connections
- Tracks connection duration, IP address, disconnect reason

### Provisioning Flow

#### Case 1: New Agent (First Connection)

```
1. Agent → Server: ProxyMessage {
     Type: PING
     Metadata: {
       "provisioning_key": "pk_abc123..."
     }
   }

2. Server:
   - Hash key (SHA-256)
   - Lookup in provisioning_keys table
   - Validate: status='active', used_count < max_uses, expires_at > NOW()
   - Insert into agents table (id auto-generated)
   - Increment provisioning_keys.used_count
   - Register in ConnectionManager using agents.id

3. Server → Agent: ProxyMessage {
     Type: PONG
     Metadata: {
       "provisioning_status": "success",
       "agent_id": "550e8400-..."  // agents.id
     }
   }
```

#### Case 2: Established Agent

```
1. Agent → Server: ProxyMessage {
     Type: PING
     Metadata: {
       "agent_id": "550e8400-..."
     }
   }

2. Server:
   - Lookup agents.id in database
   - Validate: status = 'active'
   - Update last_seen_at
   - Register in ConnectionManager

3. Server → Agent: PONG (normal flow)
```

#### Case 3: Legacy Agent (Auto-Migration)

```
1. Agent sends agent_id (old static ID)
2. Server checks if exists in agents table
3. If NOT found: Auto-register with default user
4. Connection proceeds normally
```

## Implementation Status

### Phase 1: Database Schema ✅ COMPLETED

**Deliverables:**
- ✅ 3 migration files created and tested
- ✅ SQLC queries defined for all tables
- ✅ Type-safe Go code generated via SQLC

**Files Created:**
- `internal/db/migrations/0002_create_provisioning_keys.sql`
- `internal/db/migrations/0003_create_agents.sql`
- `internal/db/migrations/0004_create_agent_connection_logs.sql`
- `internal/db/queries/provisioning_keys.sql`
- `internal/db/queries/agents.sql`
- `internal/db/queries/agent_connection_logs.sql`
- `internal/db/sqlc/*.go` (generated)

### Phase 2: Core Provisioning Logic ✅ COMPLETED

**Deliverables:**
- ✅ Service layer (provisioning, agents)
- ✅ gRPC stream handler integration
- ✅ ConnectionManager database persistence
- ⏸️ E2E test: agent provisions via gRPC stream (will be done in Phase 3)

**Files Created/Modified:**
- `internal/provisioning/service.go` (NEW) - Key generation, validation, agent provisioning
- `internal/provisioning/models.go` (NEW) - Domain models
- `internal/agents/service.go` (NEW) - Agent management, connection logging
- `internal/agents/models.go` (NEW) - Domain models
- `internal/grpc/server/stream_handler.go` (MODIFIED) - Provisioning handshake
- `internal/grpc/server/connection_manager.go` (MODIFIED) - DB persistence
- `internal/grpc/server/server.go` (MODIFIED) - Service initialization
- `internal/grpc/server/connection_manager_test.go` (MODIFIED) - Test updates
- `cmd/silo-proxy-server/main.go` (MODIFIED) - Service wiring

### Phase 3: API & Client Integration ✅ COMPLETED

**Deliverables:**
- ✅ HTTP API endpoints (POST/GET/DELETE keys, GET/DELETE agents)
- ✅ Agent config + client changes
- ✅ Config persistence after provisioning
- ⏸️ E2E test: full provisioning flow via dashboard (manual testing guide provided)

**Files Created/Modified:**
- `internal/api/http/dto/provisioning.go` (NEW) - API DTOs
- `internal/api/http/handler/provisioning.go` (NEW) - Key management endpoints
- `internal/api/http/handler/agents.go` (NEW) - Agent management endpoints
- `internal/api/http/router.go` (MODIFIED) - Route registration
- `cmd/silo-proxy-server/main.go` (MODIFIED) - Service wiring
- `cmd/silo-proxy-agent/config.go` (MODIFIED) - Add provisioning_key field
- `cmd/silo-proxy-agent/main.go` (MODIFIED) - Config persistence logic
- `internal/grpc/client/client.go` (MODIFIED) - Provisioning handshake
- `docs/provisioning/Overview.md` (UPDATED) - Phase 3 completion

## Security

### Key Generation
- 32 bytes generated using `crypto/rand`
- Format: `pk_` prefix + base64url encoding
- Keys never logged in plaintext

### Storage
- Only SHA-256 hash stored in database
- Original key shown once to user during creation
- Keys are single-use by default

### Validation
- Key hash lookup in database
- Expiration time check
- Usage count enforcement
- User ownership validation for all operations

### Rate Limiting
- 5 requests/second on provisioning endpoint
- Prevents brute-force attacks

### Audit Logging
- All provisioning attempts logged
- Connection history tracked
- Legacy agent connections flagged

## Configuration

### Server Configuration

**Database Connection:**
```yaml
database:
  url: "postgres://user:password@localhost:5432/silo-proxy"
  schema: "public"  # Optional, defaults to "public"
```

### Agent Configuration

**Provisioning (First Connection):**
```yaml
grpc:
  server_address: "server.example.com:9090"
  provisioning_key: "pk_abc123..."  # Provided by dashboard
  tls:
    enabled: true
    ca_file: "ca.pem"
```

**Established Agent (Subsequent Connections):**
```yaml
grpc:
  server_address: "server.example.com:9090"
  agent_id: "550e8400-e29b-41d4-a716-446655440000"  # Auto-saved after provisioning
  tls:
    enabled: true
    ca_file: "ca.pem"
```

## Testing

### Phase 1 Verification
```bash
# 1. Run migrations (embedded, automatic on server start)
make build

# 2. Verify SQLC generation
make generate

# 3. Verify tables exist (requires running database)
psql -d silo-proxy -c "\dt"

# Expected output:
#   provisioning_keys
#   agents
#   agent_connection_logs
#   users
```

### Phase 3 Manual Testing Guide

#### Prerequisites
- Running PostgreSQL database
- Server built and configured
- Agent built

#### Step 1: Start the Server
```bash
# Make sure database URL is configured
export DB_URL="postgres://user:password@localhost:5432/silo-proxy?sslmode=disable"

# Start server
./bin/silo-proxy-server
```

#### Step 2: Create a User Account
```bash
# Register a new user
curl -X POST http://localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "username": "testuser",
    "password": "testpass123",
    "role": "User"
  }'

# Login and get JWT token
TOKEN=$(curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "testuser",
    "password": "testpass123"
  }' | jq -r .token)

echo "JWT Token: $TOKEN"
```

#### Step 3: Generate a Provisioning Key
```bash
# Create a single-use provisioning key (expires in 24 hours)
RESPONSE=$(curl -X POST http://localhost:8080/provisioning-keys \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "max_uses": 1,
    "expires_in_hours": 24,
    "notes": "Test key for agent-1"
  }')

echo "Provisioning Response: $RESPONSE"

# Extract the provisioning key
KEY=$(echo $RESPONSE | jq -r .key)
echo "Provisioning Key: $KEY"
```

#### Step 4: Configure and Start Agent
```bash
# Create agent config with provisioning key
cat > cmd/silo-proxy-agent/application.yaml <<EOF
log:
  level: info

http:
  port: 8081

grpc:
  server_address: localhost:9090
  provisioning_key: "$KEY"
  tls:
    enabled: false

local:
  service_url: http://localhost:3000
EOF

# Start agent
./bin/silo-proxy-agent
```

**Expected Output:**
```
INFO Agent started in provisioning mode
INFO Connecting to server address=localhost:9090
INFO Attempting to provision agent with key
INFO Agent provisioned successfully agent_id=550e8400-...
INFO Agent ID persisted to config path=/path/to/application.yaml
INFO Connected to server address=localhost:9090
```

#### Step 5: Verify Agent Registration
```bash
# Check agent config file - provisioning_key should be replaced with agent_id
cat cmd/silo-proxy-agent/application.yaml

# Expected to see:
# grpc:
#   agent_id: "550e8400-..."
#   # Note: provisioning_key is removed

# List agents via API
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/agents | jq

# Expected response:
# {
#   "agents": [
#     {
#       "id": "550e8400-...",
#       "status": "active",
#       "connected": true,
#       "port": 8100,
#       "registered_at": "2026-02-09T12:00:00Z",
#       "last_seen_at": "2026-02-09T12:00:00Z"
#     }
#   ]
# }
```

#### Step 6: Test Agent Reconnection
```bash
# Stop the agent (Ctrl+C)
# Restart the agent
./bin/silo-proxy-agent

# Expected output - should use agent_id, not provisioning_key:
# INFO Agent started with agent_id agent_id=550e8400-...
# INFO Connecting with agent_id agent_id=550e8400-...
# INFO Connected to server
```

#### Step 7: Test Key Management
```bash
# List all provisioning keys
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/provisioning-keys | jq

# Expected: Key should show status="exhausted" and used_count=1

# Create multi-use key (for testing)
curl -X POST http://localhost:8080/provisioning-keys \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "max_uses": 3,
    "expires_in_hours": 48,
    "notes": "Multi-use key for testing"
  }' | jq

# Revoke a key
KEY_ID=$(curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/provisioning-keys | jq -r '.keys[0].id')

curl -X DELETE "http://localhost:8080/provisioning-keys/$KEY_ID" \
  -H "Authorization: Bearer $TOKEN"
```

#### Step 8: Test Agent Deregistration
```bash
# Get agent ID
AGENT_ID=$(curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/agents | jq -r '.agents[0].id')

# Deregister agent (soft delete)
curl -X DELETE "http://localhost:8080/agents/$AGENT_ID" \
  -H "Authorization: Bearer $TOKEN"

# Verify agent is disconnected and status is "inactive"
# The agent should be forcefully disconnected
```

#### Troubleshooting

**Agent fails to provision:**
- Check provisioning key is valid (not expired/exhausted/revoked)
- Verify server logs for detailed error messages
- Ensure database is accessible

**Config file not updated:**
- Check agent has write permissions to config directory
- Verify config path detection logic in agent logs
- Manually update config with agent_id if needed

**Agent disconnects immediately:**
- Check agent status in database (should be "active")
- Verify no other agent is using the same agent_id
- Check server logs for rejection reason

## Future Enhancements

- Multi-use keys with configurable limits
- Key rotation policies
- Certificate-based authentication
- Agent grouping/tagging
- Webhook notifications on agent events
- Advanced audit logging with retention policies

## Design Principles

1. **agents.id is the agent identifier** - No separate agent_id column, simpler schema
2. **Single-use keys by default** - max_uses=1, configurable per key
3. **Connection audit logs** - Implemented from day 1 for compliance
4. **Legacy auto-migration** - Existing agents auto-registered with default user
5. **SHA-256 key hashing** - Keys never stored in plaintext
6. **JWT-based API auth** - Reuse existing middleware
