package agents

import (
	"time"
)

type Agent struct {
	ID                   string
	UserID               string
	ProvisionedWithKeyID string
	Status               string
	CertFingerprint      string
	RegisteredAt         time.Time
	LastSeenAt           time.Time
	LastIPAddress        string
	Metadata             map[string]interface{}
	Notes                string
}

type ConnectionLog struct {
	ID               string
	AgentID          string
	ConnectedAt      time.Time
	DisconnectedAt   *time.Time
	DurationSeconds  int
	IPAddress        string
	DisconnectReason string
}
