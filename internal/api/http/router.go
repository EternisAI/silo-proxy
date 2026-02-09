package http

import (
	"github.com/EternisAI/silo-proxy/internal/agents"
	"github.com/EternisAI/silo-proxy/internal/api/http/handler"
	"github.com/EternisAI/silo-proxy/internal/api/http/middleware"
	"github.com/EternisAI/silo-proxy/internal/auth"
	"github.com/EternisAI/silo-proxy/internal/cert"
	grpcserver "github.com/EternisAI/silo-proxy/internal/grpc/server"
	"github.com/EternisAI/silo-proxy/internal/provisioning"
	"github.com/EternisAI/silo-proxy/internal/users"
	"github.com/gin-gonic/gin"
)

type Services struct {
	GrpcServer          *grpcserver.Server
	CertService         *cert.Service
	AuthService         *auth.Service
	UserService         *users.Service
	ProvisioningService *provisioning.Service
	AgentService        *agents.Service
}

func SetupRoute(engine *gin.Engine, srvs *Services, adminAPIKey string, jwtSecret string) {
	engine.Use(middleware.RequestLogger())

	healthHandler := handler.NewHealthHandler()
	engine.GET("/health", healthHandler.Check)

	authHandler := handler.NewAuthHandler(srvs.AuthService)
	authRoutes := engine.Group("/auth")
	{
		authRoutes.POST("/register", authHandler.Register)
		authRoutes.POST("/login", authHandler.Login)
	}

	userHandler := handler.NewUserHandler(srvs.UserService)
	usersGroup := engine.Group("/users")
	usersGroup.Use(middleware.JWTAuth(jwtSecret))
	{
		usersGroup.DELETE("/me", userHandler.DeleteUser)
		usersGroup.GET("", middleware.RequireRole("Admin"), userHandler.ListUsers)
	}

	// Provisioning key management (authenticated)
	if srvs.ProvisioningService != nil {
		provisioningHandler := handler.NewProvisioningHandler(srvs.ProvisioningService)
		provisioningRoutes := engine.Group("/provisioning-keys")
		provisioningRoutes.Use(middleware.JWTAuth(jwtSecret))
		{
			provisioningRoutes.POST("", provisioningHandler.CreateKey)
			provisioningRoutes.GET("", provisioningHandler.ListKeys)
			provisioningRoutes.DELETE("/:id", provisioningHandler.RevokeKey)
		}
	}

	// Agent management
	agents := engine.Group("/agents")
	{
		// Authenticated agent management (user-scoped)
		if srvs.AgentService != nil && srvs.GrpcServer != nil {
			agentsHandler := handler.NewAgentsHandler(srvs.AgentService, srvs.GrpcServer)
			agentRoutes := agents.Group("")
			agentRoutes.Use(middleware.JWTAuth(jwtSecret))
			{
				agentRoutes.GET("", agentsHandler.ListAgents)
				agentRoutes.GET("/:id", agentsHandler.GetAgent)
				agentRoutes.DELETE("/:id", agentsHandler.DeregisterAgent)
			}
		} else if srvs.GrpcServer != nil {
			// Legacy admin handler (backward compatibility)
			adminHandler := handler.NewAdminHandler(srvs.GrpcServer)
			agents.GET("", adminHandler.ListAgents)
		}

		// Certificate management (API key auth)
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
