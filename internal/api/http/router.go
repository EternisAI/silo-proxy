package http

import (
	"github.com/EternisAI/silo-proxy/internal/api/http/handler"
	"github.com/EternisAI/silo-proxy/internal/api/http/middleware"
	"github.com/EternisAI/silo-proxy/internal/auth"
	"github.com/EternisAI/silo-proxy/internal/cert"
	grpcserver "github.com/EternisAI/silo-proxy/internal/grpc/server"
	"github.com/gin-gonic/gin"
)

type Services struct {
	GrpcServer  *grpcserver.Server
	CertService *cert.Service
	AuthService *auth.Service
}

func SetupRoute(engine *gin.Engine, srvs *Services, adminAPIKey string) {
	engine.Use(middleware.RequestLogger())

	healthHandler := handler.NewHealthHandler()
	engine.GET("/health", healthHandler.Check)

	authHandler := handler.NewAuthHandler(srvs.AuthService)
	authRoutes := engine.Group("/auth")
	{
		authRoutes.POST("/register", authHandler.Register)
		authRoutes.POST("/login", authHandler.Login)
	}

	agents := engine.Group("/agents")
	{
		if srvs.GrpcServer != nil {
			adminHandler := handler.NewAdminHandler(srvs.GrpcServer)
			agents.GET("", adminHandler.ListAgents)
		}

		certHandler := handler.NewCertHandler(srvs.CertService)
		certRoutes := agents.Group("")
		certRoutes.Use(middleware.APIKeyAuth(adminAPIKey))
		{
			certRoutes.POST("/:id/certificate", certHandler.CreateAgentCertificate)
			certRoutes.GET("/:id/certificate", certHandler.GetAgentCertificate)
			certRoutes.DELETE("/:id/certificate", certHandler.DeleteAgentCertificate)
		}
	}
}
