package dto

import "time"

type AgentCertInfo struct {
	AgentID   string    `json:"agent_id"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	CertPath  string    `json:"cert_path"`
}

type ListAgentsResponse struct {
	Agents []AgentCertInfo `json:"agents"`
	Count  int             `json:"count"`
}

type DeleteCertificateResponse struct {
	Message      string   `json:"message"`
	DeletedPaths []string `json:"deleted_paths,omitempty"`
}
