package dto

import "time"

type CreateProvisioningKeyRequest struct {
	MaxUses         int    `json:"max_uses" binding:"required,min=1"`
	ExpiresInHours  int    `json:"expires_in_hours" binding:"required,min=1"`
	Notes           string `json:"notes"`
}

type ProvisioningKeyResponse struct {
	ID        string     `json:"id"`
	Key       string     `json:"key,omitempty"` // Only returned on creation
	Status    string     `json:"status"`
	MaxUses   int        `json:"max_uses"`
	UsedCount int        `json:"used_count"`
	ExpiresAt time.Time  `json:"expires_at"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
	Notes     string     `json:"notes,omitempty"`
}

type ListProvisioningKeysResponse struct {
	Keys []ProvisioningKeyResponse `json:"keys"`
}

type AgentResponse struct {
	ID                   string                 `json:"id"`
	Status               string                 `json:"status"`
	RegisteredAt         time.Time              `json:"registered_at"`
	LastSeenAt           time.Time              `json:"last_seen_at"`
	LastIPAddress        string                 `json:"last_ip_address,omitempty"`
	Connected            bool                   `json:"connected"`
	Port                 int                    `json:"port,omitempty"`
	CertFingerprint      string                 `json:"cert_fingerprint,omitempty"`
	ProvisionedWithKeyID string                 `json:"provisioned_with_key_id,omitempty"`
	Metadata             map[string]interface{} `json:"metadata,omitempty"`
	Notes                string                 `json:"notes,omitempty"`
}

type ListAgentsResponse struct {
	Agents []AgentResponse `json:"agents"`
}
