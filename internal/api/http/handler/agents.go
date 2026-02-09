package handler

import (
	"log/slog"
	"net/http"

	"github.com/EternisAI/silo-proxy/internal/agents"
	"github.com/EternisAI/silo-proxy/internal/api/http/dto"
	grpcserver "github.com/EternisAI/silo-proxy/internal/grpc/server"
	"github.com/gin-gonic/gin"
)

type AgentsHandler struct {
	agentService *agents.Service
	grpcServer   *grpcserver.Server
}

func NewAgentsHandler(agentService *agents.Service, grpcServer *grpcserver.Server) *AgentsHandler {
	return &AgentsHandler{
		agentService: agentService,
		grpcServer:   grpcServer,
	}
}

// ListAgents returns all agents for the authenticated user
// GET /agents
func (h *AgentsHandler) ListAgents(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user_id not found in context"})
		return
	}

	agentList, err := h.agentService.ListAgentsByUser(c.Request.Context(), userID)
	if err != nil {
		slog.Error("Failed to list agents", "error", err, "user_id", userID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list agents"})
		return
	}

	// Get connection manager to check which agents are currently connected
	connManager := h.grpcServer.GetConnectionManager()

	responses := make([]dto.AgentResponse, len(agentList))
	for i, a := range agentList {
		response := dto.AgentResponse{
			ID:                   a.ID,
			Status:               a.Status,
			RegisteredAt:         a.RegisteredAt,
			LastSeenAt:           a.LastSeenAt,
			LastIPAddress:        a.LastIPAddress,
			CertFingerprint:      a.CertFingerprint,
			ProvisionedWithKeyID: a.ProvisionedWithKeyID,
			Metadata:             a.Metadata,
			Notes:                a.Notes,
			Connected:            false,
		}

		// Check if agent is currently connected
		if conn, ok := connManager.GetConnection(a.ID); ok {
			response.Connected = true
			response.Port = conn.Port
		}

		responses[i] = response
	}

	c.JSON(http.StatusOK, dto.ListAgentsResponse{Agents: responses})
}

// GetAgent returns details for a specific agent
// GET /agents/:id
func (h *AgentsHandler) GetAgent(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user_id not found in context"})
		return
	}

	agentID := c.Param("id")
	if agentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "agent_id is required"})
		return
	}

	agent, err := h.agentService.GetAgentByID(c.Request.Context(), agentID)
	if err != nil {
		if err == agents.ErrAgentNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
			return
		}
		slog.Error("Failed to get agent", "error", err, "agent_id", agentID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get agent"})
		return
	}

	// Verify ownership
	if agent.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	// Get connection manager to check if agent is currently connected
	connManager := h.grpcServer.GetConnectionManager()
	response := dto.AgentResponse{
		ID:                   agent.ID,
		Status:               agent.Status,
		RegisteredAt:         agent.RegisteredAt,
		LastSeenAt:           agent.LastSeenAt,
		LastIPAddress:        agent.LastIPAddress,
		CertFingerprint:      agent.CertFingerprint,
		ProvisionedWithKeyID: agent.ProvisionedWithKeyID,
		Metadata:             agent.Metadata,
		Notes:                agent.Notes,
		Connected:            false,
	}

	if conn, ok := connManager.GetConnection(agent.ID); ok {
		response.Connected = true
		response.Port = conn.Port
	}

	c.JSON(http.StatusOK, response)
}

// DeregisterAgent soft-deletes an agent (sets status to inactive) and disconnects if online
// DELETE /agents/:id
func (h *AgentsHandler) DeregisterAgent(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user_id not found in context"})
		return
	}

	agentID := c.Param("id")
	if agentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "agent_id is required"})
		return
	}

	// Get agent to verify ownership
	agent, err := h.agentService.GetAgentByID(c.Request.Context(), agentID)
	if err != nil {
		if err == agents.ErrAgentNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
			return
		}
		slog.Error("Failed to get agent", "error", err, "agent_id", agentID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get agent"})
		return
	}

	// Verify ownership
	if agent.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	// Update status to inactive
	if err := h.agentService.UpdateStatus(c.Request.Context(), agentID, "inactive"); err != nil {
		slog.Error("Failed to update agent status", "error", err, "agent_id", agentID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to deregister agent"})
		return
	}

	// Disconnect if currently online
	connManager := h.grpcServer.GetConnectionManager()
	if _, ok := connManager.GetConnection(agentID); ok {
		connManager.Deregister(agentID)
		slog.Info("Agent forcefully disconnected", "agent_id", agentID, "user_id", userID)
	}

	slog.Info("Agent deregistered", "agent_id", agentID, "user_id", userID)
	c.JSON(http.StatusOK, gin.H{"message": "agent deregistered"})
}
