package handler

import (
	"archive/zip"
	"bytes"
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/EternisAI/silo-proxy/internal/cert"
	"github.com/gin-gonic/gin"
)

func (h *CertHandler) validateAgentID(ctx *gin.Context) (string, bool) {
	agentID := ctx.Param("id")
	if err := cert.ValidateAgentID(agentID); err != nil {
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

	if h.certService.AgentCertExists(agentID) {
		ctx.JSON(http.StatusConflict, gin.H{
			"error": "Certificate already exists for this agent",
		})
		return
	}

	slog.Info("Creating agent certificate", "agent_id", agentID)

	agentCert, agentKey, err := h.certService.GenerateAgentCert(agentID)
	if err != nil {
		slog.Error("Failed to generate agent certificate", "error", err, "agent_id", agentID)
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to generate agent certificate",
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

	if !h.certService.AgentCertExists(agentID) {
		ctx.JSON(http.StatusNotFound, gin.H{
			"error": "Certificate not found for this agent",
		})
		return
	}

	slog.Info("Retrieving agent certificate", "agent_id", agentID)

	agentCertBytes, agentKeyBytes, err := h.certService.GetAgentCert(agentID)
	if err != nil {
		slog.Error("Failed to read agent certificate", "error", err, "agent_id", agentID)
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to read agent certificate",
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

	if !h.certService.AgentCertExists(agentID) {
		ctx.JSON(http.StatusNotFound, gin.H{
			"error": "Certificate not found for this agent",
		})
		return
	}

	slog.Info("Deleting agent certificate", "agent_id", agentID)

	certDir := h.certService.GetAgentCertDir(agentID)
	if err := h.certService.DeleteAgentCert(agentID); err != nil {
		slog.Error("Failed to delete agent certificate", "error", err, "agent_id", agentID)
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to delete agent certificate",
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message":       "Successfully deleted agent certificate",
		"deleted_paths": []string{certDir},
	})
	slog.Info("Agent certificate deleted successfully", "agent_id", agentID)
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
		return nil, fmt.Errorf("failed to close zip writer: %w", err)
	}

	return zipBuffer, nil
}

func (h *CertHandler) DeleteServerCerts(ctx *gin.Context) {
	if h.certService == nil {
		slog.Warn("Server cert deletion requested but TLS is disabled")
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "TLS is not enabled on this server",
		})
		return
	}

	slog.Info("Deleting all server certificates")

	certPaths := []string{
		h.certService.CaCertPath,
		h.certService.CaKeyPath,
		h.certService.ServerCertPath,
		h.certService.ServerKeyPath,
	}

	deletedFiles := []string{}
	var errors []string

	for _, path := range certPaths {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			slog.Debug("Certificate file does not exist, skipping", "path", path)
			continue
		}

		if err := os.Remove(path); err != nil {
			slog.Error("Failed to delete certificate file", "error", err, "path", path)
			errors = append(errors, fmt.Sprintf("%s: %v", path, err))
		} else {
			slog.Info("Deleted certificate file", "path", path)
			deletedFiles = append(deletedFiles, path)
		}
	}

	if len(errors) > 0 {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error":         "Failed to delete some certificate files",
			"deleted_files": deletedFiles,
			"errors":        errors,
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message":       "Successfully deleted server certificates",
		"deleted_files": deletedFiles,
	})

	slog.Info("Server certificates deleted successfully", "count", len(deletedFiles))
}
