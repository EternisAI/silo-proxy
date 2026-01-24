package handler

import (
	"net/http"

	"github.com/EternisAI/silo-proxy/internal/api/http/dto"
	"github.com/gin-gonic/gin"
)

type HealthHandler struct{}

func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

func (h *HealthHandler) Check(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, dto.HealthResponse{Status: "ok"})
}
