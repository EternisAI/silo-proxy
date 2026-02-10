package provision

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreate(t *testing.T) {
	ks := NewKeyStore(1 * time.Hour)

	pk, err := ks.Create("agent-1")
	require.NoError(t, err)
	assert.Equal(t, "agent-1", pk.AgentID)
	assert.True(t, strings.HasPrefix(pk.Key, "sk_"))
	assert.Len(t, pk.Key, 3+64) // "sk_" + 32 bytes hex
	assert.False(t, pk.Used)
	assert.WithinDuration(t, time.Now().Add(1*time.Hour), pk.ExpiresAt, 5*time.Second)
}

func TestValidate(t *testing.T) {
	ks := NewKeyStore(1 * time.Hour)

	pk, err := ks.Create("agent-1")
	require.NoError(t, err)

	result, err := ks.Validate(pk.Key)
	require.NoError(t, err)
	assert.Equal(t, "agent-1", result.AgentID)
}

func TestValidateNotFound(t *testing.T) {
	ks := NewKeyStore(1 * time.Hour)

	_, err := ks.Validate("sk_nonexistent")
	assert.ErrorIs(t, err, ErrKeyNotFound)
}

func TestValidateExpired(t *testing.T) {
	ks := NewKeyStore(1 * time.Millisecond)

	pk, err := ks.Create("agent-1")
	require.NoError(t, err)

	time.Sleep(5 * time.Millisecond)

	_, err = ks.Validate(pk.Key)
	assert.ErrorIs(t, err, ErrKeyExpired)
}

func TestValidateAlreadyUsed(t *testing.T) {
	ks := NewKeyStore(1 * time.Hour)

	pk, err := ks.Create("agent-1")
	require.NoError(t, err)

	ks.MarkUsed(pk.Key)

	_, err = ks.Validate(pk.Key)
	assert.ErrorIs(t, err, ErrKeyAlreadyUsed)
}

func TestRevoke(t *testing.T) {
	ks := NewKeyStore(1 * time.Hour)

	pk1, err := ks.Create("agent-1")
	require.NoError(t, err)
	_, err = ks.Create("agent-1")
	require.NoError(t, err)
	_, err = ks.Create("agent-2")
	require.NoError(t, err)

	removed := ks.Revoke("agent-1")
	assert.True(t, removed)

	_, err = ks.Validate(pk1.Key)
	assert.ErrorIs(t, err, ErrKeyNotFound)

	// agent-2 key should still exist
	keys := ks.List()
	assert.Len(t, keys, 1)
	assert.Equal(t, "agent-2", keys[0].AgentID)
}

func TestRevokeNotFound(t *testing.T) {
	ks := NewKeyStore(1 * time.Hour)

	removed := ks.Revoke("nonexistent")
	assert.False(t, removed)
}

func TestList(t *testing.T) {
	ks := NewKeyStore(1 * time.Hour)

	_, err := ks.Create("agent-1")
	require.NoError(t, err)
	_, err = ks.Create("agent-2")
	require.NoError(t, err)

	keys := ks.List()
	assert.Len(t, keys, 2)

	// Keys should be redacted (empty)
	for _, k := range keys {
		assert.Empty(t, k.Key)
	}
}

func TestListExcludesUsedAndExpired(t *testing.T) {
	ks := NewKeyStore(1 * time.Millisecond)

	pk1, err := ks.Create("agent-expired")
	require.NoError(t, err)
	_ = pk1

	time.Sleep(5 * time.Millisecond)

	// Create a fresh key with longer TTL
	ks.ttl = 1 * time.Hour
	pk2, err := ks.Create("agent-used")
	require.NoError(t, err)
	ks.MarkUsed(pk2.Key)

	_, err = ks.Create("agent-active")
	require.NoError(t, err)

	keys := ks.List()
	assert.Len(t, keys, 1)
	assert.Equal(t, "agent-active", keys[0].AgentID)
}

func TestCleanup(t *testing.T) {
	ks := NewKeyStore(1 * time.Millisecond)

	_, err := ks.Create("agent-1")
	require.NoError(t, err)

	time.Sleep(5 * time.Millisecond)

	ks.cleanup()

	ks.mu.RLock()
	count := len(ks.keys)
	ks.mu.RUnlock()
	assert.Equal(t, 0, count)
}

func TestConcurrentAccess(t *testing.T) {
	ks := NewKeyStore(1 * time.Hour)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			agentID := "agent-concurrent"
			pk, err := ks.Create(agentID)
			if err != nil {
				return
			}
			_, _ = ks.Validate(pk.Key)
			_ = ks.List()
			if id%5 == 0 {
				ks.MarkUsed(pk.Key)
			}
		}(i)
	}
	wg.Wait()
}
