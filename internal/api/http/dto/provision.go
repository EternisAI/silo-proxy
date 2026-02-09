package dto

import "time"

type CreateProvisionKeyRequest struct {
	AgentID string `json:"agent_id" binding:"required"`
}

type CreateProvisionKeyResponse struct {
	Key       string    `json:"key"`
	AgentID   string    `json:"agent_id"`
	ExpiresAt time.Time `json:"expires_at"`
}

type ListProvisionKeysResponse struct {
	Keys  []ProvisionKeyInfo `json:"keys"`
	Count int                `json:"count"`
}

type ProvisionKeyInfo struct {
	AgentID   string    `json:"agent_id"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

type ProvisionRequest struct {
	Key string `json:"key" binding:"required"`
}

type ProvisionResponse struct {
	AgentID   string `json:"agent_id"`
	CertPEM   string `json:"cert_pem"`
	KeyPEM    string `json:"key_pem"`
	CACertPEM string `json:"ca_cert_pem"`
}
