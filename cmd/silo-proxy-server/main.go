package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	internalhttp "github.com/EternisAI/silo-proxy/internal/api/http"
	"github.com/EternisAI/silo-proxy/internal/auth"
	"github.com/EternisAI/silo-proxy/internal/cert"
	"github.com/EternisAI/silo-proxy/internal/db"
	"github.com/EternisAI/silo-proxy/internal/db/sqlc"
	grpcserver "github.com/EternisAI/silo-proxy/internal/grpc/server"
	"github.com/EternisAI/silo-proxy/internal/users"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

var AppVersion string

func main() {
	InitConfig()

	slog.Info("Silo Proxy Server", "version", AppVersion)

	// Initialize database
	if err := db.RunMigrations(config.DB.Url, config.DB.Schema); err != nil {
		slog.Error("Failed to run migrations", "error", err)
		os.Exit(1)
	}

	dbPool, err := db.InitDB(context.Background(), config.DB.Url, config.DB.Schema)
	if err != nil {
		slog.Error("Failed to initialize database", "error", err)
		os.Exit(1)
	}
	defer dbPool.Close()

	queries := sqlc.New(dbPool)
	authService := auth.NewService(queries, config.JWT)
	userService := users.NewService(queries)

	tlsConfig := &grpcserver.TLSConfig{
		Enabled:    config.Grpc.TLS.Enabled,
		CertFile:   config.Grpc.TLS.CertFile,
		KeyFile:    config.Grpc.TLS.KeyFile,
		CAFile:     config.Grpc.TLS.CAFile,
		ClientAuth: config.Grpc.TLS.ClientAuth,
	}

	var certService *cert.Service
	if config.Grpc.TLS.Enabled {

		var err error
		certService, err = cert.New(
			config.Grpc.TLS.CAFile,
			config.Grpc.TLS.CAKeyFile,
			config.Grpc.TLS.CertFile,
			config.Grpc.TLS.KeyFile,
			config.Grpc.TLS.AgentCertDir,
			config.Grpc.TLS.DomainNames,
			config.Grpc.TLS.IPAddresses,
			dbPool,
		)
		if err != nil {
			slog.Error("Failed to initialize certificates", "error", err)
			os.Exit(1)
		}
	}

	grpcSrv := grpcserver.NewServer(config.Grpc.Port, tlsConfig)

	portManager, err := internalhttp.NewPortManager(
		config.Http.AgentPortRange.Start,
		config.Http.AgentPortRange.End,
	)
	if err != nil {
		slog.Error("Failed to create port manager", "error", err)
		os.Exit(1)
	}

	agentServerManager := internalhttp.NewAgentServerManager(portManager, grpcSrv)
	grpcSrv.SetAgentServerManager(agentServerManager)

	slog.Info("Agent port pool initialized",
		"range_start", config.Http.AgentPortRange.Start,
		"range_end", config.Http.AgentPortRange.End,
		"pool_size", config.Http.AgentPortRange.End-config.Http.AgentPortRange.Start+1)

	services := &internalhttp.Services{
		GrpcServer:  grpcSrv,
		CertService: certService,
		AuthService: authService,
		UserService: userService,
	}

	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"PUT", "PATCH", "GET", "POST", "DELETE"},
		AllowHeaders:     []string{"Origin", "Content-Length", "Content-Type", "Authorization", "X-API-Key"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))
	engine.Use(gin.Recovery())
	internalhttp.SetupRoute(engine, services, config.Http.AdminAPIKey, config.JWT.Secret)

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", config.Http.Port),
		Handler: engine,
	}

	errChan := make(chan error, 2)
	go func() {
		slog.Info("Starting HTTP server", "address", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("HTTP server error: %w", err)
		}
	}()

	go func() {
		if err := grpcSrv.Start(); err != nil {
			errChan <- fmt.Errorf("gRPC server error: %w", err)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errChan:
		slog.Error("Server error", "error", err)
	case sig := <-sigChan:
		slog.Info("Received shutdown signal", "signal", sig)
	}

	slog.Info("Shutting down servers...")

	var wg sync.WaitGroup
	shutdownTimeout := 10 * time.Second

	wg.Add(1)
	go func() {
		defer wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := httpServer.Shutdown(ctx); err != nil {
			slog.Error("HTTP server shutdown error", "error", err)
		} else {
			slog.Info("HTTP server stopped")
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := grpcSrv.StopWithTimeout(shutdownTimeout); err != nil {
			slog.Error("gRPC server shutdown error", "error", err)
		}
	}()

	wg.Wait()
	slog.Info("Shutdown complete")
}
