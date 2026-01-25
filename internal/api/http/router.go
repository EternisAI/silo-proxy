package http

import (
	"github.com/EternisAI/silo-proxy/internal/api/http/handler"
	"github.com/EternisAI/silo-proxy/internal/api/http/middleware"
	grpcserver "github.com/EternisAI/silo-proxy/internal/grpc/server"
	"github.com/gin-gonic/gin"
)

type Config struct {
	Port uint
}

type Services struct {
	GrpcServer *grpcserver.Server
}

func SetupRoute(engine *gin.Engine, srvs *Services) {
	engine.Use(middleware.RequestLogger())

	healthHandler := handler.NewHealthHandler()
	engine.GET("/health", healthHandler.Check)

	if srvs.GrpcServer != nil {
		proxyHandler := handler.NewProxyHandler(srvs.GrpcServer)
		engine.Any("/proxy/:agent_id/*path", proxyHandler.ProxyRequest)
	}
}
