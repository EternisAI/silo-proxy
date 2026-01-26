package handler

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

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
			agentID = id
		}
	}

	h.forwardRequestWithRewrite(c, agentID, c.Request.URL.Path, "")
}

func (h *ProxyHandler) ProxyRequest(c *gin.Context) {
	agentID := c.Param("agent_id")
	if agentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "agent_id is required"})
		return
	}

	targetPath := c.Param("path")
	basePathPrefix := fmt.Sprintf("/proxy/%s", agentID)
	h.forwardRequestWithRewrite(c, agentID, targetPath, basePathPrefix)
}

func (h *ProxyHandler) forwardRequestWithRewrite(c *gin.Context, agentID, targetPath, basePathPrefix string) {
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

	payload := response.Payload

	if basePathPrefix != "" && isHTMLResponse(response.Metadata) {
		payload = rewriteHTMLPaths(response.Payload, basePathPrefix)
		slog.Debug("HTML response rewritten", "agent_id", agentID, "base_path", basePathPrefix)
	}

	c.Status(statusCode)
	c.Writer.Write(payload)
}

func isHTMLResponse(metadata map[string]string) bool {
	contentType := ""
	for key, value := range metadata {
		if strings.ToLower(key) == "header_content-type" || strings.ToLower(key) == "content_type" {
			contentType = strings.ToLower(value)
			break
		}
	}
	return strings.Contains(contentType, "text/html")
}

func rewriteHTMLPaths(htmlBytes []byte, basePathPrefix string) []byte {
	html := string(htmlBytes)

	baseTag := fmt.Sprintf("<base href=\"%s/\">", basePathPrefix)
	if strings.Contains(html, "<head>") {
		html = strings.Replace(html, "<head>", "<head>"+baseTag, 1)
	} else if strings.Contains(html, "<HEAD>") {
		html = strings.Replace(html, "<HEAD>", "<HEAD>"+baseTag, 1)
	} else {
		return htmlBytes
	}

	return []byte(html)
}
