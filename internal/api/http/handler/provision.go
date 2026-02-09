package handler

import (
	"log/slog"
	"net/http"

	"github.com/EternisAI/silo-proxy/internal/api/http/dto"
	"github.com/EternisAI/silo-proxy/internal/cert"
	"github.com/EternisAI/silo-proxy/internal/provision"
	"github.com/gin-gonic/gin"
)

type ProvisionHandler struct {
	keyStore    *provision.KeyStore
	certService *cert.Service
}

func NewProvisionHandler(keyStore *provision.KeyStore, certService *cert.Service) *ProvisionHandler {
	return &ProvisionHandler{
		keyStore:    keyStore,
		certService: certService,
	}
}

func (h *ProvisionHandler) CreateProvisionKey(ctx *gin.Context) {
	var req dto.CreateProvisionKeyRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := cert.ValidateAgentID(req.AgentID); err != nil {
		slog.Warn("Invalid agent ID for provision key", "agent_id", req.AgentID, "error", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	pk, err := h.keyStore.Create(req.AgentID)
	if err != nil {
		slog.Error("Failed to create provision key", "error", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create provision key"})
		return
	}

	ctx.JSON(http.StatusCreated, dto.CreateProvisionKeyResponse{
		Key:       pk.Key,
		AgentID:   pk.AgentID,
		ExpiresAt: pk.ExpiresAt,
	})
}

func (h *ProvisionHandler) ListProvisionKeys(ctx *gin.Context) {
	keys := h.keyStore.List()

	keyInfos := make([]dto.ProvisionKeyInfo, len(keys))
	for i, k := range keys {
		keyInfos[i] = dto.ProvisionKeyInfo{
			AgentID:   k.AgentID,
			CreatedAt: k.CreatedAt,
			ExpiresAt: k.ExpiresAt,
		}
	}

	ctx.JSON(http.StatusOK, dto.ListProvisionKeysResponse{
		Keys:  keyInfos,
		Count: len(keyInfos),
	})
}

func (h *ProvisionHandler) RevokeProvisionKey(ctx *gin.Context) {
	agentID := ctx.Param("id")

	if removed := h.keyStore.Revoke(agentID); !removed {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "No provision keys found for this agent"})
		return
	}

	slog.Info("Provision keys revoked", "agent_id", agentID)
	ctx.JSON(http.StatusOK, gin.H{"message": "Provision keys revoked"})
}

func (h *ProvisionHandler) Provision(ctx *gin.Context) {
	if h.certService == nil {
		slog.Warn("Provision requested but TLS is disabled")
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "TLS is not enabled on this server"})
		return
	}

	var req dto.ProvisionRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	pk, err := h.keyStore.Validate(req.Key)
	if err != nil {
		slog.Warn("Provision key validation failed", "error", err)
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	agentID := pk.AgentID

	if h.certService.AgentCertExists(agentID) {
		slog.Warn("Certificate already exists for agent", "agent_id", agentID)
		ctx.JSON(http.StatusConflict, gin.H{"error": "Certificate already exists for this agent"})
		return
	}

	agentCert, agentKey, err := h.certService.GenerateAgentCert(agentID)
	if err != nil {
		slog.Error("Failed to generate agent certificate", "error", err, "agent_id", agentID)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate agent certificate"})
		return
	}

	certPEM, err := cert.CertToPEM(agentCert)
	if err != nil {
		slog.Error("Failed to encode certificate", "error", err, "agent_id", agentID)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to encode certificate"})
		return
	}

	keyPEM, err := cert.KeyToPEM(agentKey)
	if err != nil {
		slog.Error("Failed to encode key", "error", err, "agent_id", agentID)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to encode key"})
		return
	}

	caCertBytes, err := h.certService.GetCACert()
	if err != nil {
		slog.Error("Failed to read CA certificate", "error", err, "agent_id", agentID)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read CA certificate"})
		return
	}

	h.keyStore.MarkUsed(req.Key)

	slog.Info("Agent provisioned successfully", "agent_id", agentID)
	ctx.JSON(http.StatusOK, dto.ProvisionResponse{
		AgentID:   agentID,
		CertPEM:   string(certPEM),
		KeyPEM:    string(keyPEM),
		CACertPEM: string(caCertBytes),
	})
}
