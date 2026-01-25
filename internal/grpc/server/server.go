package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/EternisAI/silo-proxy/proto"
	"google.golang.org/grpc"
)

type Server struct {
	proto.UnimplementedProxyServiceServer
	grpcServer     *grpc.Server
	connManager    *ConnectionManager
	streamHandler  *StreamHandler
	port           int
	listener       net.Listener
}

func NewServer(port int) *Server {
	connManager := NewConnectionManager()
	streamHandler := NewStreamHandler(connManager)

	return &Server{
		connManager:   connManager,
		streamHandler: streamHandler,
		port:          port,
	}
}

func (s *Server) Start() error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		return fmt.Errorf("failed to listen on port %d: %w", s.port, err)
	}
	s.listener = lis

	s.grpcServer = grpc.NewServer()
	proto.RegisterProxyServiceServer(s.grpcServer, s)

	slog.Info("Starting gRPC server", "port", s.port)

	if err := s.grpcServer.Serve(lis); err != nil {
		return fmt.Errorf("failed to serve gRPC: %w", err)
	}

	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	slog.Info("Stopping gRPC server")

	stopped := make(chan struct{})
	go func() {
		s.grpcServer.GracefulStop()
		close(stopped)
	}()

	select {
	case <-stopped:
		slog.Info("gRPC server stopped gracefully")
	case <-ctx.Done():
		slog.Warn("gRPC server stop timeout, forcing shutdown")
		s.grpcServer.Stop()
	}

	s.connManager.Stop()

	return nil
}

func (s *Server) Stream(stream proto.ProxyService_StreamServer) error {
	return s.streamHandler.HandleStream(stream)
}

func (s *Server) StopWithTimeout(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return s.Stop(ctx)
}
