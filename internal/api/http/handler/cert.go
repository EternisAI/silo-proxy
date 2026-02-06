package handler

import (
	"archive/zip"
	"bytes"
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/EternisAI/silo-proxy/internal/cert"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
)

func (h *CertHandler) validateAgentID(ctx *gin.Context) (string, bool) {
	agentID := ctx.Param("id")
	if err := cert.ValidateAgentID(agentID); err != nil {
		slog.Warn("Invalid agent ID", "agent_id", agentID, "error", err)
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return "", false
	}
	return agentID, true
}

type CertHandler struct {
	certService *cert.Service
}

func NewCertHandler(certService *cert.Service) *CertHandler {
	return &CertHandler{
		certService: certService,
	}
}

func (h *CertHandler) CreateAgentCertificate(ctx *gin.Context) {
	if h.certService == nil {
		slog.Warn("Agent cert creation requested but TLS is disabled")
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "TLS is not enabled on this server",
		})
		return
	}

	agentID, ok := h.validateAgentID(ctx)
	if !ok {
		return
	}

	var req struct {
		UserID string `json:"user_id" binding:"required"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "user_id is required in request body",
		})
		return
	}

	var userID pgtype.UUID
	if err := userID.Scan(req.UserID); err != nil {
		slog.Error("Failed to parse user ID", "error", err, "user_id", req.UserID)
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid user_id format",
		})
		return
	}

	slog.Info("Creating agent certificate", "agent_id", agentID, "user_id", userID)

	agentCert, agentKey, err := h.certService.GenerateAgentCertWithDB(ctx, agentID, userID)
	if err != nil {
		slog.Error("Failed to generate agent certificate", "error", err, "agent_id", agentID)
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	caCertBytes, err := h.certService.GetCACert()
	if err != nil {
		slog.Error("Failed to read CA certificate", "error", err, "agent_id", agentID)
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to read CA certificate",
		})
		return
	}

	zipBuffer, err := h.createCertZip(agentID, agentCert, agentKey, caCertBytes)
	if err != nil {
		slog.Error("Failed to create zip file", "error", err, "agent_id", agentID)
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create zip file",
		})
		return
	}

	ctx.Header("Content-Type", "application/zip")
	ctx.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s-certs.zip\"", agentID))
	ctx.Data(http.StatusCreated, "application/zip", zipBuffer.Bytes())

	slog.Info("Agent certificate created successfully", "agent_id", agentID, "zip_size", zipBuffer.Len())
}

func (h *CertHandler) GetAgentCertificate(ctx *gin.Context) {
	if h.certService == nil {
		slog.Warn("Agent cert retrieval requested but TLS is disabled")
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "TLS is not enabled on this server",
		})
		return
	}

	agentID, ok := h.validateAgentID(ctx)
	if !ok {
		return
	}

	slog.Info("Retrieving agent certificate", "agent_id", agentID)

	agentCertBytes, agentKeyBytes, err := h.certService.GetAgentCertFromDB(ctx, agentID)
	if err != nil {
		slog.Error("Failed to read agent certificate", "error", err, "agent_id", agentID)
		ctx.JSON(http.StatusNotFound, gin.H{
			"error": "Certificate not found for this agent",
		})
		return
	}

	caCertBytes, err := h.certService.GetCACert()
	if err != nil {
		slog.Error("Failed to read CA certificate", "error", err, "agent_id", agentID)
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to read CA certificate",
		})
		return
	}

	zipBuffer, err := h.createCertZipFromBytes(agentID, agentCertBytes, agentKeyBytes, caCertBytes)
	if err != nil {
		slog.Error("Failed to create zip file", "error", err, "agent_id", agentID)
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create zip file",
		})
		return
	}

	ctx.Header("Content-Type", "application/zip")
	ctx.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s-certs.zip\"", agentID))
	ctx.Data(http.StatusOK, "application/zip", zipBuffer.Bytes())

	slog.Info("Agent certificate retrieved successfully", "agent_id", agentID, "zip_size", zipBuffer.Len())
}

func (h *CertHandler) DeleteAgentCertificate(ctx *gin.Context) {
	if h.certService == nil {
		slog.Warn("Agent cert deletion requested but TLS is disabled")
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "TLS is not enabled on this server",
		})
		return
	}

	agentID, ok := h.validateAgentID(ctx)
	if !ok {
		return
	}

	userIDStr := ctx.Query("user_id")
	if userIDStr == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "user_id query parameter is required",
		})
		return
	}

	var userID pgtype.UUID
	if err := userID.Scan(userIDStr); err != nil {
		slog.Error("Failed to parse user ID", "error", err, "user_id", userIDStr)
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid user_id format",
		})
		return
	}

	slog.Info("Deleting agent certificate", "agent_id", agentID, "user_id", userID)

	if err := h.certService.DeleteAgentCertFromDB(ctx, agentID, userID); err != nil {
		slog.Error("Failed to delete agent certificate", "error", err, "agent_id", agentID)
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message": "Successfully deleted agent certificate",
	})
	slog.Info("Agent certificate deleted successfully", "agent_id", agentID)
}

func (h *CertHandler) RevokeAgentCertificate(ctx *gin.Context) {
	if h.certService == nil {
		slog.Warn("Agent cert revocation requested but TLS is disabled")
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "TLS is not enabled on this server",
		})
		return
	}

	agentID, ok := h.validateAgentID(ctx)
	if !ok {
		return
	}

	var req struct {
		UserID string `json:"user_id" binding:"required"`
		Reason string `json:"reason"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "user_id is required in request body",
		})
		return
	}

	var userID pgtype.UUID
	if err := userID.Scan(req.UserID); err != nil {
		slog.Error("Failed to parse user ID", "error", err, "user_id", req.UserID)
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid user_id format",
		})
		return
	}

	slog.Info("Revoking agent certificate", "agent_id", agentID, "user_id", userID, "reason", req.Reason)

	if err := h.certService.RevokeAgentCert(ctx, agentID, userID, req.Reason); err != nil {
		slog.Error("Failed to revoke agent certificate", "error", err, "agent_id", agentID)
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message": "Successfully revoked agent certificate",
	})
	slog.Info("Agent certificate revoked successfully", "agent_id", agentID)
}

func (h *CertHandler) ListUserAgentCertificates(ctx *gin.Context) {
	if h.certService == nil {
		slog.Warn("Agent cert list requested but TLS is disabled")
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "TLS is not enabled on this server",
		})
		return
	}

	userIDStr := ctx.Query("user_id")
	if userIDStr == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "user_id query parameter is required",
		})
		return
	}

	var userID pgtype.UUID
	if err := userID.Scan(userIDStr); err != nil {
		slog.Error("Failed to parse user ID", "error", err, "user_id", userIDStr)
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid user_id format",
		})
		return
	}

	certs, err := h.certService.ListUserAgentCerts(ctx, userID)
	if err != nil {
		slog.Error("Failed to list certificates", "error", err, "user_id", userID)
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to list certificates",
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"certificates": certs,
	})
}

func (h *CertHandler) createCertZip(agentID string, agentCert *x509.Certificate, agentKey *rsa.PrivateKey, caCertBytes []byte) (*bytes.Buffer, error) {
	agentCertPEM, err := cert.CertToPEM(agentCert)
	if err != nil {
		return nil, fmt.Errorf("failed to encode agent certificate: %w", err)
	}

	agentKeyPEM, err := cert.KeyToPEM(agentKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encode agent key: %w", err)
	}

	return h.createCertZipFromBytes(agentID, agentCertPEM, agentKeyPEM, caCertBytes)
}

func (h *CertHandler) createCertZipFromBytes(agentID string, certPEM, keyPEM, caCertBytes []byte) (*bytes.Buffer, error) {
	zipBuffer := new(bytes.Buffer)
	zipWriter := zip.NewWriter(zipBuffer)
	defer zipWriter.Close()

	files := map[string][]byte{
		fmt.Sprintf("%s-cert.pem", agentID): certPEM,
		fmt.Sprintf("%s-key.pem", agentID):  keyPEM,
		"ca-cert.pem":                       caCertBytes,
	}

	for filename, content := range files {
		f, err := zipWriter.Create(filename)
		if err != nil {
			return nil, fmt.Errorf("failed to create zip entry for %s: %w", filename, err)
		}
		if _, err := f.Write(content); err != nil {
			return nil, fmt.Errorf("failed to write zip entry for %s: %w", filename, err)
		}
	}

	if err := zipWriter.Close(); err != nil {
		return nil, fmt.Errorf("failed to finalize zip archive: %w", err)
	}

	return zipBuffer, nil
}
