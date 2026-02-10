package provision

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

var (
	ErrKeyNotFound   = errors.New("provision key not found")
	ErrKeyExpired    = errors.New("provision key has expired")
	ErrKeyAlreadyUsed = errors.New("provision key has already been used")
)

type ProvisionKey struct {
	Key       string
	AgentID   string
	CreatedAt time.Time
	ExpiresAt time.Time
	Used      bool
}

type KeyStore struct {
	mu   sync.RWMutex
	keys map[string]*ProvisionKey
	ttl  time.Duration
}

func NewKeyStore(ttl time.Duration) *KeyStore {
	return &KeyStore{
		keys: make(map[string]*ProvisionKey),
		ttl:  ttl,
	}
}

func (ks *KeyStore) Create(agentID string) (*ProvisionKey, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("failed to generate random key: %w", err)
	}

	key := "sk_" + hex.EncodeToString(b)
	now := time.Now()

	pk := &ProvisionKey{
		Key:       key,
		AgentID:   agentID,
		CreatedAt: now,
		ExpiresAt: now.Add(ks.ttl),
		Used:      false,
	}

	ks.mu.Lock()
	ks.keys[key] = pk
	ks.mu.Unlock()

	slog.Info("Provision key created", "agent_id", agentID, "expires_at", pk.ExpiresAt)
	return pk, nil
}

func (ks *KeyStore) Validate(key string) (*ProvisionKey, error) {
	ks.mu.RLock()
	pk, exists := ks.keys[key]
	ks.mu.RUnlock()

	if !exists {
		return nil, ErrKeyNotFound
	}
	if pk.Used {
		return nil, ErrKeyAlreadyUsed
	}
	if time.Now().After(pk.ExpiresAt) {
		return nil, ErrKeyExpired
	}
	return pk, nil
}

func (ks *KeyStore) MarkUsed(key string) {
	ks.mu.Lock()
	if pk, exists := ks.keys[key]; exists {
		pk.Used = true
	}
	ks.mu.Unlock()
}

func (ks *KeyStore) Revoke(agentID string) bool {
	ks.mu.Lock()
	defer ks.mu.Unlock()

	removed := false
	for key, pk := range ks.keys {
		if pk.AgentID == agentID {
			delete(ks.keys, key)
			removed = true
		}
	}
	return removed
}

func (ks *KeyStore) List() []ProvisionKey {
	ks.mu.RLock()
	defer ks.mu.RUnlock()

	var result []ProvisionKey
	for _, pk := range ks.keys {
		if pk.Used || time.Now().After(pk.ExpiresAt) {
			continue
		}
		result = append(result, ProvisionKey{
			AgentID:   pk.AgentID,
			CreatedAt: pk.CreatedAt,
			ExpiresAt: pk.ExpiresAt,
		})
	}
	return result
}

func (ks *KeyStore) StartCleanup(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ks.cleanup()
		}
	}
}

func (ks *KeyStore) cleanup() {
	ks.mu.Lock()
	defer ks.mu.Unlock()

	now := time.Now()
	removed := 0
	for key, pk := range ks.keys {
		if pk.Used || now.After(pk.ExpiresAt) {
			delete(ks.keys, key)
			removed++
		}
	}
	if removed > 0 {
		slog.Debug("Cleaned up provision keys", "removed", removed)
	}
}
