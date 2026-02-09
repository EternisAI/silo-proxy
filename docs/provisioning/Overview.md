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

### Phase 3: API & Client Integration ⏸️ PLANNED

**Deliverables:**
- HTTP API endpoints (POST/GET/DELETE keys, GET/DELETE agents)
- Agent config + client changes
- E2E test: full provisioning flow via dashboard
- Documentation updates

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
