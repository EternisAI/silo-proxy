package http

import (
	"github.com/EternisAI/silo-proxy/internal/api/http/handler"
	"github.com/EternisAI/silo-proxy/internal/api/http/middleware"
	grpcserver "github.com/EternisAI/silo-proxy/internal/grpc/server"
	"github.com/gin-gonic/gin"
)

type Config struct {
	Port           uint
	AgentPortRange PortRange
}

type PortRange struct {
	Start int
	End   int
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
		// Specific agent routing with prefix
		engine.Any("/proxy/:agent_id/*path", proxyHandler.ProxyRequest)
		// Catch-all: route everything else to default agent (agent-1)
		engine.NoRoute(proxyHandler.ProxyRootRequest)
	}
}
