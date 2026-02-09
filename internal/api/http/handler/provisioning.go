package handler

import (
	"log/slog"
	"net/http"

	"github.com/EternisAI/silo-proxy/internal/api/http/dto"
	"github.com/EternisAI/silo-proxy/internal/provisioning"
	"github.com/gin-gonic/gin"
)

type ProvisioningHandler struct {
	provisioningService *provisioning.Service
}

func NewProvisioningHandler(provisioningService *provisioning.Service) *ProvisioningHandler {
	return &ProvisioningHandler{
		provisioningService: provisioningService,
	}
}

// CreateKey generates a new provisioning key
// POST /provisioning-keys
func (h *ProvisioningHandler) CreateKey(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user_id not found in context"})
		return
	}

	var req dto.CreateProvisioningKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	key, plaintextKey, err := h.provisioningService.CreateKey(c.Request.Context(), userID, req.MaxUses, req.ExpiresInHours, req.Notes)
	if err != nil {
		slog.Error("Failed to create provisioning key", "error", err, "user_id", userID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create provisioning key"})
		return
	}

	response := dto.ProvisioningKeyResponse{
		ID:        key.ID,
		Key:       plaintextKey, // Only shown once!
		Status:    key.Status,
		MaxUses:   key.MaxUses,
		UsedCount: key.UsedCount,
		ExpiresAt: key.ExpiresAt,
		CreatedAt: key.CreatedAt,
		UpdatedAt: key.UpdatedAt,
		Notes:     key.Notes,
	}

	slog.Info("Provisioning key created", "key_id", key.ID, "user_id", userID, "max_uses", key.MaxUses)
	c.JSON(http.StatusCreated, response)
}

// ListKeys returns all provisioning keys for the authenticated user
// GET /provisioning-keys
func (h *ProvisioningHandler) ListKeys(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user_id not found in context"})
		return
	}

	keys, err := h.provisioningService.ListUserKeys(c.Request.Context(), userID)
	if err != nil {
		slog.Error("Failed to list provisioning keys", "error", err, "user_id", userID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list provisioning keys"})
		return
	}

	responses := make([]dto.ProvisioningKeyResponse, len(keys))
	for i, k := range keys {
		responses[i] = dto.ProvisioningKeyResponse{
			ID:        k.ID,
			Status:    k.Status,
			MaxUses:   k.MaxUses,
			UsedCount: k.UsedCount,
			ExpiresAt: k.ExpiresAt,
			CreatedAt: k.CreatedAt,
			UpdatedAt: k.UpdatedAt,
			RevokedAt: k.RevokedAt,
			Notes:     k.Notes,
		}
	}

	c.JSON(http.StatusOK, dto.ListProvisioningKeysResponse{Keys: responses})
}

// RevokeKey revokes a provisioning key
// DELETE /provisioning-keys/:id
func (h *ProvisioningHandler) RevokeKey(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user_id not found in context"})
		return
	}

	keyID := c.Param("id")
	if keyID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "key_id is required"})
		return
	}

	if err := h.provisioningService.RevokeKey(c.Request.Context(), keyID, userID); err != nil {
		slog.Error("Failed to revoke provisioning key", "error", err, "key_id", keyID, "user_id", userID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to revoke provisioning key"})
		return
	}

	slog.Info("Provisioning key revoked", "key_id", keyID, "user_id", userID)
	c.JSON(http.StatusOK, gin.H{"message": "provisioning key revoked"})
}
