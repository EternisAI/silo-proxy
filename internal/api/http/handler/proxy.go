package handler

import (
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/EternisAI/silo-proxy/internal/api/http/middleware"
	"github.com/EternisAI/silo-proxy/internal/grpc/server"
	"github.com/EternisAI/silo-proxy/proto"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ProxyHandler struct {
	grpcServer *server.Server
}

func NewProxyHandler(grpcServer *server.Server) *ProxyHandler {
	return &ProxyHandler{
		grpcServer: grpcServer,
	}
}

func (h *ProxyHandler) ProxyRootRequest(c *gin.Context) {
	agentID := "agent-1"

	if subdomainAgentID, exists := c.Get(middleware.SubdomainAgentIDKey); exists {
		if id, ok := subdomainAgentID.(string); ok && id != "" {
			slog.Debug("Subdomain Routing", "agentID", id)
			agentID = id
		}
	}

	h.forwardRequest(c, agentID, c.Request.URL.Path)
}

func (h *ProxyHandler) ProxyRequest(c *gin.Context) {
	agentID := c.Param("agent_id")
	if agentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "agent_id is required"})
		return
	}

	targetPath := c.Param("path")
	h.forwardRequest(c, agentID, targetPath)
}

func (h *ProxyHandler) forwardRequest(c *gin.Context, agentID, targetPath string) {
	conn, ok := h.grpcServer.GetConnectionManager().GetConnection(agentID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
		return
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		slog.Error("Failed to read request body", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read request body"})
		return
	}

	headers := make(map[string]string)
	for key, values := range c.Request.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	requestMsg := &proto.ProxyMessage{
		Id:      uuid.New().String(),
		Type:    proto.MessageType_REQUEST,
		Payload: body,
		Metadata: map[string]string{
			"method":       c.Request.Method,
			"path":         targetPath,
			"query":        c.Request.URL.RawQuery,
			"content_type": c.ContentType(),
		},
	}

	for key, value := range headers {
		requestMsg.Metadata["header_"+key] = value
	}

	slog.Info("Forwarding request to agent",
		"agent_id", agentID,
		"message_id", requestMsg.Id,
		"method", c.Request.Method,
		"path", targetPath)

	response, err := h.grpcServer.SendRequestToAgent(c.Request.Context(), conn.ID, requestMsg)
	if err != nil {
		slog.Error("Failed to forward request", "error", err, "agent_id", agentID)
		c.JSON(http.StatusGatewayTimeout, gin.H{"error": err.Error()})
		return
	}

	statusCode := http.StatusOK
	if statusStr, ok := response.Metadata["status_code"]; ok {
		if code, err := strconv.Atoi(statusStr); err == nil {
			statusCode = code
		}
	}

	for key, value := range response.Metadata {
		if key != "status_code" && len(value) > 0 {
			headerName := key
			if len(key) > 7 && key[:7] == "header_" {
				headerName = key[7:]
			}
			c.Header(headerName, value)
		}
	}

	slog.Info("Received response from agent",
		"agent_id", agentID,
		"message_id", response.Id,
		"status_code", statusCode)

	c.Status(statusCode)
	c.Writer.Write(response.Payload)
}
