package dto

import "time"

type AgentInfo struct {
	AgentID  string    `json:"agent_id"`
	Port     int       `json:"port"`
	LastSeen time.Time `json:"last_seen"`
}

type AgentsResponse struct {
	Agents []AgentInfo `json:"agents"`
	Count  int         `json:"count"`
}

type CertificateCreatedResponse struct {
	AgentID   string `json:"agent_id"`
	SyncKey   string `json:"sync_key"`
	ExpiresAt string `json:"expires_at"`
	Message   string `json:"message"`
}
