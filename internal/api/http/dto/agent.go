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
