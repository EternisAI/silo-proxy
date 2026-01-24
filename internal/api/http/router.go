package http

import (
	"github.com/EternisAI/silo-proxy/internal/api/http/handler"
	"github.com/EternisAI/silo-proxy/internal/api/http/middleware"
	"github.com/gin-gonic/gin"
)

type Config struct {
	Port uint
}

type Services struct {
}

func SetupRoute(engine *gin.Engine, srvs *Services) {
	engine.Use(middleware.RequestLogger())

	healthHandler := handler.NewHealthHandler()

	engine.GET("/health", healthHandler.Check)
}
