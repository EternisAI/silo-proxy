package handler

import (
	"net/http"

	"github.com/EternisAI/silo-proxy/internal/api/http/dto"
	grpcserver "github.com/EternisAI/silo-proxy/internal/grpc/server"
	"github.com/gin-gonic/gin"
)

type AdminHandler struct {
	grpcServer *grpcserver.Server
}

func NewAdminHandler(grpcServer *grpcserver.Server) *AdminHandler {
	return &AdminHandler{
		grpcServer: grpcServer,
	}
}

func (h *AdminHandler) ListAgents(ctx *gin.Context) {
	connManager := h.grpcServer.GetConnectionManager()
	agentIDs := connManager.ListConnections()

	agents := make([]dto.AgentInfo, 0, len(agentIDs))
	for _, agentID := range agentIDs {
		conn, ok := connManager.GetConnection(agentID)
		if ok {
			agents = append(agents, dto.AgentInfo{
				AgentID:  conn.ID,
				Port:     conn.Port,
				LastSeen: conn.LastSeen,
			})
		}
	}

	ctx.JSON(http.StatusOK, dto.AgentsResponse{
		Agents: agents,
		Count:  len(agents),
	})
}
