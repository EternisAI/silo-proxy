package http

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/EternisAI/silo-proxy/internal/api/http/handler"
	"github.com/EternisAI/silo-proxy/internal/api/http/middleware"
	grpcserver "github.com/EternisAI/silo-proxy/internal/grpc/server"
	"github.com/gin-gonic/gin"
)

const (
	serverShutdownTimeout = 5 * time.Second
	maxPortBindRetries    = 3
)

// AgentServerInfo holds information about a running agent HTTP server
type AgentServerInfo struct {
	AgentID   string
	Port      int
	Server    *http.Server
	Listener  net.Listener
	StartedAt time.Time
}

// AgentServerManager manages the lifecycle of per-agent HTTP servers.
// Each connected agent gets its own dedicated HTTP server listening on
// a unique port allocated from the PortManager.
type AgentServerManager struct {
	servers     map[string]*AgentServerInfo // agentID -> server info
	mu          sync.RWMutex
	portManager *PortManager
	grpcServer  *grpcserver.Server
	shutdownWg  sync.WaitGroup
}

// NewAgentServerManager creates a new AgentServerManager.
// It requires a PortManager for port allocation and a gRPC server
// for forwarding requests to agents.
func NewAgentServerManager(pm *PortManager, gs *grpcserver.Server) *AgentServerManager {
	return &AgentServerManager{
		servers:     make(map[string]*AgentServerInfo),
		portManager: pm,
		grpcServer:  gs,
	}
}

// StartAgentServer allocates a port and starts a new HTTP server for the specified agent.
// The server will proxy all incoming requests directly to the agent via gRPC.
// Returns the allocated port number on success, or an error if port allocation
// or server startup fails.
func (asm *AgentServerManager) StartAgentServer(agentID string) (int, error) {
	asm.mu.Lock()
	defer asm.mu.Unlock()

	// Check if server already exists for this agent
	if _, exists := asm.servers[agentID]; exists {
		return 0, fmt.Errorf("server already exists for agent: %s", agentID)
	}

	// Try to allocate port and bind server with retries
	var port int
	var srv *http.Server
	var lastErr error

	for attempt := 1; attempt <= maxPortBindRetries; attempt++ {
		// Allocate port from pool
		allocatedPort, err := asm.portManager.Allocate(agentID)
		if err != nil {
			return 0, fmt.Errorf("failed to allocate port: %w", err)
		}

		// Try to bind the port synchronously to verify it's available
		listener, err := net.Listen("tcp", fmt.Sprintf(":%d", allocatedPort))
		if err != nil {
			// Binding failed, release port and retry
			asm.portManager.Release(allocatedPort)
			lastErr = fmt.Errorf("failed to bind port %d: %w", allocatedPort, err)
			slog.Warn("Port binding failed, retrying",
				"agent_id", agentID,
				"port", allocatedPort,
				"attempt", attempt,
				"error", err)
			continue
		}

		// Port binding succeeded, create server
		engine := asm.createAgentEngine(agentID)
		srv = &http.Server{
			Handler: engine,
		}

		info := &AgentServerInfo{
			AgentID:   agentID,
			Port:      allocatedPort,
			Server:    srv,
			Listener:  listener,
			StartedAt: time.Now(),
		}

		asm.servers[agentID] = info

		// Start server in goroutine with the listener we already bound
		go func(serverInfo *AgentServerInfo) {
			slog.Info("Starting agent HTTP server",
				"agent_id", serverInfo.AgentID,
				"port", serverInfo.Port)

			if err := serverInfo.Server.Serve(serverInfo.Listener); err != nil && err != http.ErrServerClosed {
				slog.Error("Agent HTTP server failed",
					"agent_id", serverInfo.AgentID,
					"port", serverInfo.Port,
					"error", err)

				// Cleanup resources on unexpected failure
				_ = serverInfo.Listener.Close()
				asm.mu.Lock()
				if info, ok := asm.servers[serverInfo.AgentID]; ok && info == serverInfo {
					delete(asm.servers, serverInfo.AgentID)
					asm.portManager.Release(serverInfo.Port)
					slog.Info("Cleaned up failed agent server",
						"agent_id", serverInfo.AgentID,
						"port", serverInfo.Port)
				}
				asm.mu.Unlock()
			}
		}(info)

		port = allocatedPort
		lastErr = nil
		break
	}

	if lastErr != nil {
		return 0, fmt.Errorf("failed to start server after %d attempts: %w", maxPortBindRetries, lastErr)
	}

	slog.Info("Agent HTTP server started successfully",
		"agent_id", agentID,
		"port", port)

	return port, nil
}

// StopAgentServer gracefully shuts down the HTTP server for the specified agent
// and releases the port back to the pool. If graceful shutdown fails within
// the timeout, it forces the server to close.
func (asm *AgentServerManager) StopAgentServer(agentID string) error {
	asm.mu.Lock()
	info, exists := asm.servers[agentID]
	if !exists {
		asm.mu.Unlock()
		slog.Warn("Attempted to stop non-existent agent server", "agent_id", agentID)
		return fmt.Errorf("no server found for agent: %s", agentID)
	}
	delete(asm.servers, agentID)
	asm.mu.Unlock()

	slog.Info("Stopping agent HTTP server",
		"agent_id", agentID,
		"port", info.Port)

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), serverShutdownTimeout)
	defer cancel()

	err := info.Server.Shutdown(ctx)
	if err != nil {
		slog.Warn("Graceful shutdown timeout, forcing close",
			"agent_id", agentID,
			"port", info.Port,
			"error", err)
		// Force close if graceful shutdown fails
		info.Server.Close()
	}

	if info.Listener != nil {
		if err := info.Listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			slog.Warn("Failed to close agent listener",
				"agent_id", agentID,
				"port", info.Port,
				"error", err)
		}
	}

	// Always release port back to pool
	asm.portManager.Release(info.Port)

	slog.Info("Agent HTTP server stopped",
		"agent_id", agentID,
		"port", info.Port)

	return nil
}

// GetAllServers returns a snapshot of all currently running agent servers.
// The returned slice is safe to use and will not be affected by subsequent
// server lifecycle operations.
func (asm *AgentServerManager) GetAllServers() []*AgentServerInfo {
	asm.mu.RLock()
	defer asm.mu.RUnlock()

	servers := make([]*AgentServerInfo, 0, len(asm.servers))
	for _, info := range asm.servers {
		// Return copy to prevent external modification
		serverCopy := &AgentServerInfo{
			AgentID:   info.AgentID,
			Port:      info.Port,
			Server:    info.Server,
			Listener:  info.Listener,
			StartedAt: info.StartedAt,
		}
		servers = append(servers, serverCopy)
	}

	return servers
}

// Shutdown gracefully stops all running agent servers.
// It waits for all servers to shut down completely before returning.
func (asm *AgentServerManager) Shutdown() error {
	asm.mu.Lock()
	agentIDs := make([]string, 0, len(asm.servers))
	for agentID := range asm.servers {
		agentIDs = append(agentIDs, agentID)
	}
	asm.mu.Unlock()

	slog.Info("Shutting down all agent servers", "count", len(agentIDs))

	// Stop all servers
	for _, agentID := range agentIDs {
		if err := asm.StopAgentServer(agentID); err != nil {
			slog.Error("Failed to stop agent server during shutdown",
				"agent_id", agentID,
				"error", err)
		}
	}

	slog.Info("All agent servers shut down")
	return nil
}

// createAgentEngine creates a minimal Gin engine for a specific agent.
// All routes proxy directly to the agent without requiring agent_id in the path.
func (asm *AgentServerManager) createAgentEngine(agentID string) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()

	// Add middleware
	engine.Use(middleware.RequestLogger())
	engine.Use(gin.Recovery())

	// Create proxy handler for this specific agent
	proxyHandler := handler.NewProxyHandler(asm.grpcServer)

	// All requests route directly to this agent (no agent_id prefix needed)
	engine.NoRoute(func(c *gin.Context) {
		// Forward all requests to this specific agent
		proxyHandler.ProxyRequestDirect(c, agentID)
	})

	return engine
}
