package handler

import (
	"archive/zip"
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/EternisAI/silo-proxy/internal/api/http/dto"
	"github.com/EternisAI/silo-proxy/internal/cert"
	"github.com/gin-gonic/gin"
)

type CertHandler struct {
	certService *cert.Service
}

func NewCertHandler(certService *cert.Service) *CertHandler {
	return &CertHandler{
		certService: certService,
	}
}

func (h *CertHandler) ProvisionAgent(ctx *gin.Context) {
	if h.certService == nil {
		slog.Warn("Agent cert provisioning requested but TLS is disabled")
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "TLS is not enabled on this server",
		})
		return
	}

	var req dto.ProvisionAgentRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Invalid request: %v", err),
		})
		return
	}

	if req.AgentID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "agent_id is required",
		})
		return
	}

	slog.Info("Provisioning agent certificates", "agent_id", req.AgentID)

	agentCert, agentKey, err := h.certService.GenerateAgentCert(req.AgentID)
	if err != nil {
		slog.Error("Failed to generate agent certificate", "error", err, "agent_id", req.AgentID)
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to generate agent certificate",
		})
		return
	}

	caCertBytes, err := h.certService.GetCACert()
	if err != nil {
		slog.Error("Failed to read CA certificate", "error", err, "agent_id", req.AgentID)
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to read CA certificate",
		})
		return
	}

	zipBuffer := new(bytes.Buffer)
	zipWriter := zip.NewWriter(zipBuffer)
	defer zipWriter.Close()

	agentCertPEM, err := cert.CertToPEM(agentCert)
	if err != nil {
		slog.Error("Failed to encode agent certificate", "error", err, "agent_id", req.AgentID)
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to encode agent certificate",
		})
		return
	}

	agentKeyPEM, err := cert.KeyToPEM(agentKey)
	if err != nil {
		slog.Error("Failed to encode agent key", "error", err, "agent_id", req.AgentID)
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to encode agent key",
		})
		return
	}

	files := map[string][]byte{
		fmt.Sprintf("%s-cert.pem", req.AgentID): agentCertPEM,
		fmt.Sprintf("%s-key.pem", req.AgentID):  agentKeyPEM,
		"ca-cert.pem":                           caCertBytes,
	}

	for filename, content := range files {
		f, err := zipWriter.Create(filename)
		if err != nil {
			slog.Error("Failed to create zip file entry", "error", err, "filename", filename)
			ctx.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to create zip file",
			})
			return
		}
		if _, err := f.Write(content); err != nil {
			slog.Error("Failed to write to zip file", "error", err, "filename", filename)
			ctx.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to write zip file",
			})
			return
		}
	}

	ctx.Header("Content-Type", "application/zip")
	ctx.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s-certs.zip\"", req.AgentID))
	ctx.Data(http.StatusOK, "application/zip", zipBuffer.Bytes())

	slog.Info("Agent certificates provisioned successfully", "agent_id", req.AgentID, "zip_size", zipBuffer.Len())
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
