package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/EternisAI/silo-proxy/internal/api/http/dto"
	"github.com/EternisAI/silo-proxy/internal/auth"
	"github.com/EternisAI/silo-proxy/internal/db/sqlc"
	"github.com/EternisAI/silo-proxy/internal/users"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type AuthHandler struct {
	queries   *sqlc.Queries
	jwtConfig auth.JWTConfig
}

func NewAuthHandler(queries *sqlc.Queries, jwtConfig auth.JWTConfig) *AuthHandler {
	return &AuthHandler{
		queries:   queries,
		jwtConfig: jwtConfig,
	}
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req dto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	hash, err := users.HashPassword(req.Password)
	if err != nil {
		slog.Error("Failed to hash password", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user"})
		return
	}

	user, err := h.queries.CreateUser(c.Request.Context(), sqlc.CreateUserParams{
		Username:     req.Username,
		PasswordHash: hash,
		Role:         sqlc.UserRoleStudent,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			c.JSON(http.StatusConflict, gin.H{"error": "username already exists"})
			return
		}
		slog.Error("Failed to create user", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user"})
		return
	}

	c.JSON(http.StatusCreated, dto.RegisterResponse{
		ID:       uuidToString(user.ID.Bytes),
		Username: user.Username,
		Role:     string(user.Role),
	})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.queries.GetUserByUsername(c.Request.Context(), req.Username)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}
		slog.Error("Failed to query user", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	if !users.CheckPassword(req.Password, user.PasswordHash) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	token, err := auth.GenerateToken(h.jwtConfig, uuidToString(user.ID.Bytes), user.Username, string(user.Role))
	if err != nil {
		slog.Error("Failed to generate token", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, dto.LoginResponse{Token: token})
}

func uuidToString(id [16]byte) string {
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		id[0:4], id[4:6], id[6:8], id[8:10], id[10:16])
}
