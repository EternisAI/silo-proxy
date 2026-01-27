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
	grpcserver "github.com/EternisAI/silo-proxy/internal/grpc/server"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

var AppVersion string

func main() {
	InitConfig()

	slog.Info("Silo Proxy Server", "version", AppVersion)

	tlsConfig := &grpcserver.TLSConfig{
		Enabled:    config.Grpc.TLS.Enabled,
		CertFile:   config.Grpc.TLS.CertFile,
		KeyFile:    config.Grpc.TLS.KeyFile,
		CAFile:     config.Grpc.TLS.CAFile,
		ClientAuth: config.Grpc.TLS.ClientAuth,
	}

	grpcSrv := grpcserver.NewServer(config.Grpc.Port, tlsConfig)

	services := &internalhttp.Services{
		GrpcServer: grpcSrv,
	}

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
