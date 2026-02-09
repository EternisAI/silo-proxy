package provisioning

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/EternisAI/silo-proxy/internal/cert"
	"github.com/EternisAI/silo-proxy/internal/db/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	keyPrefix = "pk_"
	keyLength = 32 // 32 bytes = 256 bits
)

var (
	ErrKeyNotFound = errors.New("provisioning key not found")
	ErrKeyExpired  = errors.New("provisioning key expired")
	ErrKeyExhausted = errors.New("provisioning key exhausted")
	ErrKeyInvalid  = errors.New("provisioning key invalid")
)

type Service struct {
	queries     *sqlc.Queries
	certService *cert.Service
}

func NewService(queries *sqlc.Queries, certService *cert.Service) *Service {
	return &Service{
		queries:     queries,
		certService: certService,
	}
}

// GenerateKey creates a new provisioning key with crypto/rand
func GenerateKey() (string, error) {
	bytes := make([]byte, keyLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Use base64 URL-safe encoding
	encoded := base64.RawURLEncoding.EncodeToString(bytes)
	return keyPrefix + encoded, nil
}

// HashKey computes SHA-256 hash of the key
func HashKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return fmt.Sprintf("%x", hash)
}

// CreateKey generates and stores a new provisioning key
func (s *Service) CreateKey(ctx context.Context, userID string, maxUses int, expiresInHours int, notes string) (*ProvisioningKey, string, error) {
	// Generate key
	key, err := GenerateKey()
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate key: %w", err)
	}

	// Hash the key for storage
	keyHash := HashKey(key)

	// Parse user ID
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		return nil, "", fmt.Errorf("invalid user ID: %w", err)
	}

	// Calculate expiration time
	expiresAt := time.Now().Add(time.Duration(expiresInHours) * time.Hour)

	// Store in database
	dbKey, err := s.queries.CreateProvisioningKey(ctx, sqlc.CreateProvisioningKeyParams{
		KeyHash:   keyHash,
		UserID:    pgtype.UUID{Bytes: parsedUserID, Valid: true},
		MaxUses:   int32(maxUses),
		ExpiresAt: pgtype.Timestamp{Time: expiresAt, Valid: true},
		Notes:     pgtype.Text{String: notes, Valid: notes != ""},
	})
	if err != nil {
		return nil, "", fmt.Errorf("failed to store key: %w", err)
	}

	result := &ProvisioningKey{
		ID:        uuidToString(dbKey.ID.Bytes),
		KeyHash:   dbKey.KeyHash,
		UserID:    uuidToString(dbKey.UserID.Bytes),
		Status:    string(dbKey.Status),
		MaxUses:   int(dbKey.MaxUses),
		UsedCount: int(dbKey.UsedCount),
		ExpiresAt: dbKey.ExpiresAt.Time,
		CreatedAt: dbKey.CreatedAt.Time,
		UpdatedAt: dbKey.UpdatedAt.Time,
		Notes:     dbKey.Notes.String,
	}

	// Return both the model and the plaintext key (only shown once)
	return result, key, nil
}

// ProvisionAgent validates a provisioning key and creates a new agent
func (s *Service) ProvisionAgent(ctx context.Context, key string, remoteIP string) (*AgentProvisionResult, error) {
	// Hash the provided key
	keyHash := HashKey(key)

	// Lookup key in database (only returns active keys)
	dbKey, err := s.queries.GetProvisioningKeyByHash(ctx, keyHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			slog.Warn("Provisioning attempt with invalid key", "remote_ip", remoteIP)
			return nil, ErrKeyNotFound
		}
		return nil, fmt.Errorf("failed to lookup key: %w", err)
	}

	// Validate expiration
	if time.Now().After(dbKey.ExpiresAt.Time) {
		slog.Warn("Provisioning attempt with expired key",
			"key_id", uuidToString(dbKey.ID.Bytes),
			"expires_at", dbKey.ExpiresAt.Time,
			"remote_ip", remoteIP)
		return nil, ErrKeyExpired
	}

	// Atomically increment key usage count.
	// The WHERE clause (used_count < max_uses AND status = 'active') ensures
	// concurrent requests cannot exceed max_uses — only one will succeed.
	_, err = s.queries.IncrementKeyUsage(ctx, dbKey.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			slog.Warn("Provisioning attempt with exhausted key",
				"key_id", uuidToString(dbKey.ID.Bytes),
				"remote_ip", remoteIP)
			return nil, ErrKeyExhausted
		}
		return nil, fmt.Errorf("failed to increment key usage: %w", err)
	}

	// Create agent in database (ID is auto-generated)
	dbAgent, err := s.queries.CreateAgent(ctx, sqlc.CreateAgentParams{
		UserID:               dbKey.UserID,
		ProvisionedWithKeyID: pgtype.UUID{Bytes: dbKey.ID.Bytes, Valid: true},
		Metadata:             []byte("{}"),
		Notes:                pgtype.Text{String: "", Valid: false},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create agent: %w", err)
	}

	result := &AgentProvisionResult{
		AgentID: uuidToString(dbAgent.ID.Bytes),
	}

	slog.Info("Agent provisioned successfully",
		"agent_id", result.AgentID,
		"user_id", uuidToString(dbKey.UserID.Bytes),
		"key_id", uuidToString(dbKey.ID.Bytes),
		"remote_ip", remoteIP)

	return result, nil
}

// ListUserKeys returns all provisioning keys for a user
func (s *Service) ListUserKeys(ctx context.Context, userID string) ([]ProvisioningKey, error) {
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	dbKeys, err := s.queries.ListProvisioningKeysByUser(ctx, pgtype.UUID{Bytes: parsedUserID, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("failed to list keys: %w", err)
	}

	result := make([]ProvisioningKey, len(dbKeys))
	for i, k := range dbKeys {
		result[i] = ProvisioningKey{
			ID:        uuidToString(k.ID.Bytes),
			KeyHash:   k.KeyHash,
			UserID:    uuidToString(k.UserID.Bytes),
			Status:    string(k.Status),
			MaxUses:   int(k.MaxUses),
			UsedCount: int(k.UsedCount),
			ExpiresAt: k.ExpiresAt.Time,
			CreatedAt: k.CreatedAt.Time,
			UpdatedAt: k.UpdatedAt.Time,
			Notes:     k.Notes.String,
		}
		if k.RevokedAt.Valid {
			result[i].RevokedAt = &k.RevokedAt.Time
		}
	}

	return result, nil
}

// RevokeKey revokes a provisioning key
func (s *Service) RevokeKey(ctx context.Context, keyID string, userID string) error {
	parsedKeyID, err := uuid.Parse(keyID)
	if err != nil {
		return fmt.Errorf("invalid key ID: %w", err)
	}

	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("invalid user ID: %w", err)
	}

	if err := s.queries.RevokeProvisioningKey(ctx, sqlc.RevokeProvisioningKeyParams{
		ID:     pgtype.UUID{Bytes: parsedKeyID, Valid: true},
		UserID: pgtype.UUID{Bytes: parsedUserID, Valid: true},
	}); err != nil {
		return fmt.Errorf("failed to revoke key: %w", err)
	}

	slog.Info("Provisioning key revoked", "key_id", keyID, "user_id", userID)
	return nil
}

// ExpireOldKeys marks expired keys as expired (cleanup task)
func (s *Service) ExpireOldKeys(ctx context.Context) error {
	if err := s.queries.ExpireOldKeys(ctx); err != nil {
		return fmt.Errorf("failed to expire old keys: %w", err)
	}
	return nil
}

func uuidToString(id [16]byte) string {
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		id[0:4], id[4:6], id[6:8], id[8:10], id[10:16])
}
