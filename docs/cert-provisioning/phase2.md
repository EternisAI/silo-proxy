# Phase 2: Admin API

**Status**: Pending

## Summary

Add REST endpoints for admins to create, list, and revoke provision keys. These endpoints are served on the existing admin HTTP server (port 8080).

## Files to Add

- `internal/api/http/handler/provision.go` - Handler functions
- `internal/api/http/handler/provision_test.go` - Handler tests

## Files to Modify

- `internal/api/http/router.go` - Register new routes

## Routes

```
POST   /api/v1/provision-keys      → CreateProvisionKey
GET    /api/v1/provision-keys      → ListProvisionKeys
DELETE /api/v1/provision-keys/:id  → RevokeProvisionKey
```

## Implementation

### Handler Struct

```go
type ProvisionHandler struct {
    keyStore *provision.KeyStore
}

func NewProvisionHandler(keyStore *provision.KeyStore) *ProvisionHandler
```

### CreateProvisionKey

```go
func (h *ProvisionHandler) CreateProvisionKey(c *gin.Context)
```

Request:
```json
{
  "agent_id": "agent-5",
  "ttl_hours": 24
}
```

Validation:
- `agent_id` is required, non-empty
- `ttl_hours` is optional, defaults to server config value
- `ttl_hours` must be positive if provided

Response (201):
```json
{
  "provision_key": "sk_a1b2c3d4e5f6...",
  "agent_id": "agent-5",
  "expires_at": "2026-02-10T12:00:00Z"
}
```

This is the only endpoint that returns the raw key value.

### ListProvisionKeys

```go
func (h *ProvisionHandler) ListProvisionKeys(c *gin.Context)
```

Response (200):
```json
{
  "keys": [
    {
      "agent_id": "agent-5",
      "expires_at": "2026-02-10T12:00:00Z",
      "used": false,
      "created_at": "2026-02-09T12:00:00Z"
    }
  ]
}
```

The `provision_key` value is never returned in list responses.

### RevokeProvisionKey

```go
func (h *ProvisionHandler) RevokeProvisionKey(c *gin.Context)
```

Deletes the provision key for the given agent ID. The `:id` path parameter is the agent ID.

Response (200):
```json
{
  "message": "provision key revoked",
  "agent_id": "agent-5"
}
```

Response (404) if no key exists for the agent:
```json
{
  "error": "no provision key found for agent"
}
```

### Router Registration

```go
v1 := router.Group("/api/v1")
{
    v1.POST("/provision-keys", provisionHandler.CreateProvisionKey)
    v1.GET("/provision-keys", provisionHandler.ListProvisionKeys)
    v1.DELETE("/provision-keys/:id", provisionHandler.RevokeProvisionKey)
}
```

## Test Cases

1. Create key with valid agent_id returns 201 + key
2. Create key with missing agent_id returns 400
3. Create key with custom TTL
4. List keys returns active keys without raw key values
5. List keys with no keys returns empty array
6. Revoke existing key returns 200
7. Revoke nonexistent key returns 404

## Next Steps

**Phase 3**: Implement CSR parsing and signing logic that the provision endpoint will use.
