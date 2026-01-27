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

	if srvs.GrpcServer != nil {
		adminHandler := handler.NewAdminHandler(srvs.GrpcServer)
		engine.GET("/agents", adminHandler.ListAgents)
	}

	certHandler := handler.NewCertHandler(srvs.CertService)
	engine.POST("/cert/agent", certHandler.ProvisionAgent)
	engine.DELETE("/cert/server", certHandler.DeleteServerCerts)
}
