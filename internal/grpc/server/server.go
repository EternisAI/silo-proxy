package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/EternisAI/silo-proxy/proto"
	"google.golang.org/grpc"

	grpctls "github.com/EternisAI/silo-proxy/internal/grpc/tls"
)

const (
	requestTimeout = 30 * time.Second
)

type Server struct {
	proto.UnimplementedProxyServiceServer
	grpcServer      *grpc.Server
	connManager     *ConnectionManager
	streamHandler   *StreamHandler
	port            int
	tlsConfig       *TLSConfig
	listener        net.Listener
	pendingRequests map[string]chan *proto.ProxyMessage
	pendingMu       sync.RWMutex
}

type TLSConfig struct {
	Enabled    bool
	CertFile   string
	KeyFile    string
	CAFile     string
	ClientAuth string
}

func NewServer(port int, tlsConfig *TLSConfig) *Server {
	connManager := NewConnectionManager(nil)

	s := &Server{
		connManager:     connManager,
		port:            port,
		tlsConfig:       tlsConfig,
		pendingRequests: make(map[string]chan *proto.ProxyMessage),
	}

	streamHandler := NewStreamHandler(connManager, s)
	s.streamHandler = streamHandler

	return s
}

func (s *Server) Start() error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		return fmt.Errorf("failed to listen on port %d: %w", s.port, err)
	}
	s.listener = lis

	var opts []grpc.ServerOption

	if s.tlsConfig != nil && s.tlsConfig.Enabled {
		clientAuth, err := grpctls.ParseClientAuthType(s.tlsConfig.ClientAuth)
		if err != nil {
			return fmt.Errorf("invalid client auth type: %w", err)
		}

		creds, err := grpctls.LoadServerCredentials(
			s.tlsConfig.CertFile,
			s.tlsConfig.KeyFile,
			s.tlsConfig.CAFile,
			clientAuth,
		)
		if err != nil {
			return fmt.Errorf("failed to load TLS credentials: %w", err)
		}

		opts = append(opts, grpc.Creds(creds))
		slog.Info("Starting gRPC server with TLS", "port", s.port, "client_auth", s.tlsConfig.ClientAuth)
	} else {
		slog.Warn("Starting gRPC server without TLS (insecure)", "port", s.port)
	}

	s.grpcServer = grpc.NewServer(opts...)
	proto.RegisterProxyServiceServer(s.grpcServer, s)

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

func (s *Server) SendRequestToAgent(ctx context.Context, agentID string, msg *proto.ProxyMessage) (*proto.ProxyMessage, error) {
	respCh := make(chan *proto.ProxyMessage, 1)

	s.pendingMu.Lock()
	s.pendingRequests[msg.Id] = respCh
	s.pendingMu.Unlock()

	defer func() {
		s.pendingMu.Lock()
		delete(s.pendingRequests, msg.Id)
		s.pendingMu.Unlock()
	}()

	if err := s.connManager.SendToAgent(agentID, msg); err != nil {
		return nil, fmt.Errorf("failed to send request to agent: %w", err)
	}

	select {
	case response := <-respCh:
		return response, nil
	case <-time.After(requestTimeout):
		return nil, fmt.Errorf("request timeout")
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *Server) HandleResponse(msg *proto.ProxyMessage) {
	s.pendingMu.RLock()
	respCh, ok := s.pendingRequests[msg.Id]
	s.pendingMu.RUnlock()

	if !ok {
		slog.Warn("Received response for unknown request", "message_id", msg.Id)
		return
	}

	select {
	case respCh <- msg:
	default:
		slog.Warn("Response channel full or closed", "message_id", msg.Id)
	}
}

func (s *Server) GetConnectionManager() *ConnectionManager {
	return s.connManager
}

func (s *Server) SetAgentServerManager(asm AgentServerManager) {
	s.connManager.SetAgentServerManager(asm)
}
