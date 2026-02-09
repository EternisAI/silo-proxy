# Phase 1: Provision Key Store

**Status**: Pending

## Summary

Implement an in-memory store for provision keys with CRUD operations, one-time use enforcement, TTL-based expiry, and background cleanup.

## Files to Add

- `internal/provision/key_store.go` - Core implementation
- `internal/provision/key_store_test.go` - Unit tests

## Implementation

### Data Structures

```go
type ProvisionKey struct {
    Key       string
    AgentID   string
    ExpiresAt time.Time
    Used      bool
    CreatedAt time.Time
}

type KeyStore struct {
    keys map[string]*ProvisionKey // provision key string -> ProvisionKey
    mu   sync.RWMutex
    ttl  time.Duration
}
```

### API

```go
func NewKeyStore(defaultTTL time.Duration) *KeyStore
func (s *KeyStore) Create(agentID string, ttl time.Duration) *ProvisionKey
func (s *KeyStore) Validate(key string) (*ProvisionKey, error)
func (s *KeyStore) MarkUsed(key string)
func (s *KeyStore) Revoke(agentID string) bool
func (s *KeyStore) List() []*ProvisionKey
func (s *KeyStore) StartCleanup(ctx context.Context, interval time.Duration)
```

### Key Generation

- 32 random bytes, hex-encoded, prefixed with `sk_`
- Example: `sk_a1b2c3d4e5f6...` (68 characters total)
- Use `crypto/rand` for generation

### Validate Logic

`Validate` checks three conditions and returns a specific error for each:
1. Key exists in the map → `"invalid provision key"`
2. Key not expired → `"provision key expired"`
3. Key not already used → `"provision key already used"`

Returns the `ProvisionKey` on success.

### Background Cleanup

`StartCleanup` runs in a goroutine, periodically removing keys that are both expired and used (or expired beyond a grace period). Controlled by a context for clean shutdown.

```go
func (s *KeyStore) StartCleanup(ctx context.Context, interval time.Duration) {
    ticker := time.NewTicker(interval)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            s.removeExpired()
        }
    }
}
```

### Thread Safety

- All map access protected by `sync.RWMutex`
- `Create`, `MarkUsed`, `Revoke`, `removeExpired`: write lock
- `Validate`, `List`: read lock

## Test Cases

1. Create key and validate successfully
2. Validate with invalid key returns error
3. Validate expired key returns error
4. Validate used key returns error
5. MarkUsed prevents reuse
6. Revoke removes key for agent
7. List returns only active (unused, unexpired) keys
8. Cleanup removes expired keys
9. Concurrent create + validate (10 goroutines)
10. Custom TTL override per key

## Next Steps

**Phase 2**: Build admin API endpoints that use KeyStore for CRUD operations.
