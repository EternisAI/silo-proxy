package provisioning

import (
	"time"
)

type ProvisioningKey struct {
	ID        string
	KeyHash   string
	UserID    string
	Status    string
	MaxUses   int
	UsedCount int
	ExpiresAt time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
	RevokedAt *time.Time
	Notes     string
}

type AgentProvisionResult struct {
	AgentID         string
	CertFingerprint string // Optional: TLS certificate fingerprint
}
