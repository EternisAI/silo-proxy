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
	grpcclient "github.com/EternisAI/silo-proxy/internal/grpc/client"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

var AppVersion string

func main() {
	InitConfig()

	slog.Info("Silo Proxy Agent", "version", AppVersion)

	grpcClient := grpcclient.NewClient(config.Grpc.ServerAddress, config.Grpc.AgentID, config.Local.ServiceURL)
	if err := grpcClient.Start(); err != nil {
		slog.Error("Failed to start gRPC client", "error", err)
		os.Exit(1)
	}

	services := &internalhttp.Services{}

	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"PUT", "PATCH", "GET", "POST", "DELETE"},
		AllowHeaders:     []string{"Origin", "Content-Length", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))
	engine.Use(gin.Recovery())
	internalhttp.SetupRoute(engine, services)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", config.Http.Port),
		Handler: engine,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		slog.Info("Starting HTTP server", "address", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP server error", "error", err)
			quit <- syscall.SIGTERM
		}
	}()

	sig := <-quit
	slog.Info("Received shutdown signal", "signal", sig)

	slog.Info("Shutting down servers...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := server.Shutdown(ctx); err != nil {
			slog.Error("HTTP server shutdown error", "error", err)
		} else {
			slog.Info("HTTP server stopped")
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := grpcClient.Stop(); err != nil {
			slog.Error("gRPC client stop error", "error", err)
		}
	}()

	wg.Wait()
	slog.Info("Shutdown complete")
}
