package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/EternisAI/silo-proxy/internal/api/http/dto"
	"github.com/EternisAI/silo-proxy/internal/users"
	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	userService *users.Service
}

func NewUserHandler(userService *users.Service) *UserHandler {
	return &UserHandler{userService: userService}
}

func (h *UserHandler) DeleteUser(c *gin.Context) {
	userID, _ := c.Get("user_id")

	if err := h.userService.DeleteUser(c.Request.Context(), userID.(string)); err != nil {
		if errors.Is(err, users.ErrUserNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		slog.Error("Failed to delete user", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *UserHandler) ListUsers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	offset := (page - 1) * pageSize
	userList, total, err := h.userService.ListUsers(c.Request.Context(), pageSize, offset)
	if err != nil {
		slog.Error("Failed to list users", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	userResponses := make([]dto.UserResponse, len(userList))
	for i, u := range userList {
		userResponses[i] = dto.UserResponse{
			ID:        u.ID,
			Username:  u.Username,
			Role:      u.Role,
			CreatedAt: u.CreatedAt.Format(time.RFC3339),
		}
	}

	c.JSON(http.StatusOK, dto.ListUsersResponse{
		Users:    userResponses,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}
