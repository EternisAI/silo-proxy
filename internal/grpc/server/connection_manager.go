package server

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/EternisAI/silo-proxy/internal/agents"
	"github.com/EternisAI/silo-proxy/proto"
)

// AgentServerManager interface defines the per-agent HTTP server management methods.
// This interface allows ConnectionManager to manage agent HTTP servers without
// direct dependency on the http package implementation.
type AgentServerManager interface {
	StartAgentServer(agentID string) (int, error)
	StopAgentServer(agentID string) error
	Shutdown() error
}

const (
	sendChannelBuffer      = 100
	sendTimeout            = 5 * time.Second
	staleConnectionTimeout = 2 * time.Minute
	cleanupInterval        = 30 * time.Second
)

type AgentConnection struct {
	ID       string
	Port     int // HTTP server port for this agent (0 if no dedicated server)
	Stream   proto.ProxyService_StreamServer
	SendCh   chan *proto.ProxyMessage
	LastSeen time.Time
	ctx      context.Context
	cancel   context.CancelFunc
}

type ConnectionManager struct {
	agents             map[string]*AgentConnection
	mu                 sync.RWMutex
	stopCh             chan struct{}
	agentServerManager AgentServerManager // Optional: manages per-agent HTTP servers
	agentService       *agents.Service    // Optional: for database persistence
}

// NewConnectionManager creates a new ConnectionManager.
// The agentServerManager parameter is optional (can be nil) and enables
// per-agent HTTP server management when provided.
func NewConnectionManager(agentServerManager AgentServerManager, agentService *agents.Service) *ConnectionManager {
	cm := &ConnectionManager{
		agents:             make(map[string]*AgentConnection),
		stopCh:             make(chan struct{}),
		agentServerManager: agentServerManager,
		agentService:       agentService,
	}
	go cm.cleanupStaleConnections()
	return cm
}

// SetAgentServerManager sets the AgentServerManager after ConnectionManager creation.
// This allows breaking circular dependencies during initialization.
func (cm *ConnectionManager) SetAgentServerManager(asm AgentServerManager) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.agentServerManager = asm
}

func (cm *ConnectionManager) Register(agentID string, stream proto.ProxyService_StreamServer) (*AgentConnection, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Clean up existing connection if present
	if existing, ok := cm.agents[agentID]; ok {
		slog.Warn("Agent already connected, replacing connection", "agent_id", agentID)
		existing.cancel()
		close(existing.SendCh)

		// Stop existing agent server if manager available
		if cm.agentServerManager != nil && existing.Port != 0 {
			if err := cm.agentServerManager.StopAgentServer(agentID); err != nil {
				slog.Error("Failed to stop existing agent server during re-registration",
					"agent_id", agentID,
					"port", existing.Port,
					"error", err)
			}
		}

		delete(cm.agents, agentID)
	}

	// Start per-agent HTTP server if manager available
	var port int
	if cm.agentServerManager != nil {
		allocatedPort, err := cm.agentServerManager.StartAgentServer(agentID)
		if err != nil {
			slog.Error("Failed to start agent HTTP server",
				"agent_id", agentID,
				"error", err)
			return nil, fmt.Errorf("failed to start agent HTTP server: %w", err)
		}
		port = allocatedPort
	}

	ctx, cancel := context.WithCancel(context.Background())
	conn := &AgentConnection{
		ID:       agentID,
		Port:     port,
		Stream:   stream,
		SendCh:   make(chan *proto.ProxyMessage, sendChannelBuffer),
		LastSeen: time.Now(),
		ctx:      ctx,
		cancel:   cancel,
	}

	cm.agents[agentID] = conn

	if port != 0 {
		slog.Info("Agent registered with dedicated HTTP server",
			"agent_id", agentID,
			"port", port,
			"total_connections", len(cm.agents))
	} else {
		slog.Info("Agent registered",
			"agent_id", agentID,
			"total_connections", len(cm.agents))
	}

	return conn, nil
}

func (cm *ConnectionManager) Deregister(agentID string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if conn, ok := cm.agents[agentID]; ok {
		conn.cancel()
		close(conn.SendCh)

		// Stop agent HTTP server if manager available
		if cm.agentServerManager != nil && conn.Port != 0 {
			if err := cm.agentServerManager.StopAgentServer(agentID); err != nil {
				slog.Error("Failed to stop agent HTTP server during deregistration",
					"agent_id", agentID,
					"port", conn.Port,
					"error", err)
			}
		}

		delete(cm.agents, agentID)

		if conn.Port != 0 {
			slog.Info("Agent deregistered, HTTP server stopped",
				"agent_id", agentID,
				"port", conn.Port,
				"total_connections", len(cm.agents))
		} else {
			slog.Info("Agent deregistered",
				"agent_id", agentID,
				"total_connections", len(cm.agents))
		}
	}
}

func (cm *ConnectionManager) SendToAgent(agentID string, msg *proto.ProxyMessage) error {
	cm.mu.RLock()
	conn, ok := cm.agents[agentID]
	cm.mu.RUnlock()

	if !ok {
		return fmt.Errorf("agent not found: %s", agentID)
	}

	select {
	case conn.SendCh <- msg:
		slog.Debug("Message queued for agent", "agent_id", agentID, "message_id", msg.Id, "type", msg.Type)
		return nil
	case <-time.After(sendTimeout):
		return fmt.Errorf("timeout sending message to agent: %s", agentID)
	case <-conn.ctx.Done():
		return fmt.Errorf("agent connection closed: %s", agentID)
	}
}

func (cm *ConnectionManager) UpdateLastSeen(agentID string) {
	cm.mu.Lock()
	conn, ok := cm.agents[agentID]
	if ok {
		conn.LastSeen = time.Now()
	}
	cm.mu.Unlock()

	if !ok {
		return
	}

	slog.Debug("Agent last seen updated", "agent_id", agentID)

	// Async DB update (non-blocking)
	if cm.agentService != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := cm.agentService.UpdateLastSeen(ctx, agentID, conn.LastSeen, ""); err != nil {
				slog.Debug("Failed to update last seen in database", "agent_id", agentID, "error", err)
			}
		}()
	}
}

func (cm *ConnectionManager) GetConnection(agentID string) (*AgentConnection, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	conn, ok := cm.agents[agentID]
	return conn, ok
}

func (cm *ConnectionManager) ListConnections() []string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	agentIDs := make([]string, 0, len(cm.agents))
	for id := range cm.agents {
		agentIDs = append(agentIDs, id)
	}
	return agentIDs
}

func (cm *ConnectionManager) Stop() {
	close(cm.stopCh)

	cm.mu.Lock()
	defer cm.mu.Unlock()

	for agentID, conn := range cm.agents {
		conn.cancel()
		close(conn.SendCh)

		// Stop agent HTTP server if manager available
		if cm.agentServerManager != nil && conn.Port != 0 {
			if err := cm.agentServerManager.StopAgentServer(agentID); err != nil {
				slog.Error("Failed to stop agent HTTP server during shutdown",
					"agent_id", agentID,
					"port", conn.Port,
					"error", err)
			}
		}
	}
	cm.agents = make(map[string]*AgentConnection)

	// Shutdown all agent servers if manager available
	if cm.agentServerManager != nil {
		if err := cm.agentServerManager.Shutdown(); err != nil {
			slog.Error("Failed to shutdown agent server manager", "error", err)
		}
	}
}

func (cm *ConnectionManager) cleanupStaleConnections() {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cm.removeStaleConnections()
		case <-cm.stopCh:
			return
		}
	}
}

func (cm *ConnectionManager) removeStaleConnections() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	now := time.Now()
	for agentID, conn := range cm.agents {
		if now.Sub(conn.LastSeen) > staleConnectionTimeout {
			slog.Warn("Removing stale connection",
				"agent_id", agentID,
				"last_seen", conn.LastSeen,
				"port", conn.Port)

			conn.cancel()
			close(conn.SendCh)

			// Stop agent HTTP server if manager available
			if cm.agentServerManager != nil && conn.Port != 0 {
				if err := cm.agentServerManager.StopAgentServer(agentID); err != nil {
					slog.Error("Failed to stop agent HTTP server during stale connection cleanup",
						"agent_id", agentID,
						"port", conn.Port,
						"error", err)
				}
			}

			delete(cm.agents, agentID)
		}
	}
}
