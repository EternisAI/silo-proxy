package http

import (
	"github.com/EternisAI/silo-proxy/internal/api/http/handler"
	"github.com/EternisAI/silo-proxy/internal/api/http/middleware"
	"github.com/EternisAI/silo-proxy/internal/cert"
	grpcserver "github.com/EternisAI/silo-proxy/internal/grpc/server"
	"github.com/gin-gonic/gin"
)

type Services struct {
	GrpcServer  *grpcserver.Server
	CertService *cert.Service
}

func SetupRoute(engine *gin.Engine, srvs *Services) {
	engine.Use(middleware.RequestLogger())

	healthHandler := handler.NewHealthHandler()
	engine.GET("/health", healthHandler.Check)

	certHandler := handler.NewCertHandler(srvs.CertService)

	agents := engine.Group("/agents")
	{
		if srvs.GrpcServer != nil {
			adminHandler := handler.NewAdminHandler(srvs.GrpcServer)
			agents.GET("", adminHandler.ListAgents)
		}

		agents.POST("/:id/certificate", certHandler.CreateAgentCertificate)
		agents.GET("/:id/certificate", certHandler.GetAgentCertificate)
		agents.DELETE("/:id/certificate", certHandler.DeleteAgentCertificate)
	}

	// Temp endpoint for cleaning
	engine.DELETE("/server-certs", certHandler.DeleteServerCerts)
}
