package dto

type ProvisionAgentRequest struct {
	AgentID string `json:"agent_id" binding:"required"`
}
