package server

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/EternisAI/silo-proxy/proto"
)

const (
	sendChannelBuffer      = 100
	sendTimeout            = 5 * time.Second
	staleConnectionTimeout = 2 * time.Minute
	cleanupInterval        = 30 * time.Second
)

type AgentConnection struct {
	ID       string
	Stream   proto.ProxyService_StreamServer
	SendCh   chan *proto.ProxyMessage
	LastSeen time.Time
	ctx      context.Context
	cancel   context.CancelFunc
}

type ConnectionManager struct {
	agents map[string]*AgentConnection
	mu     sync.RWMutex
	stopCh chan struct{}
}

func NewConnectionManager() *ConnectionManager {
	cm := &ConnectionManager{
		agents: make(map[string]*AgentConnection),
		stopCh: make(chan struct{}),
	}
	go cm.cleanupStaleConnections()
	return cm
}

func (cm *ConnectionManager) Register(agentID string, stream proto.ProxyService_StreamServer) (*AgentConnection, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if existing, ok := cm.agents[agentID]; ok {
		slog.Warn("Agent already connected, replacing connection", "agent_id", agentID)
		existing.cancel()
		close(existing.SendCh)
		delete(cm.agents, agentID)
	}

	ctx, cancel := context.WithCancel(context.Background())
	conn := &AgentConnection{
		ID:       agentID,
		Stream:   stream,
		SendCh:   make(chan *proto.ProxyMessage, sendChannelBuffer),
		LastSeen: time.Now(),
		ctx:      ctx,
		cancel:   cancel,
	}

	cm.agents[agentID] = conn
	slog.Info("Agent registered", "agent_id", agentID, "total_connections", len(cm.agents))

	return conn, nil
}

func (cm *ConnectionManager) Deregister(agentID string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if conn, ok := cm.agents[agentID]; ok {
		conn.cancel()
		close(conn.SendCh)
		delete(cm.agents, agentID)
		slog.Info("Agent deregistered", "agent_id", agentID, "total_connections", len(cm.agents))
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
	defer cm.mu.Unlock()

	if conn, ok := cm.agents[agentID]; ok {
		conn.LastSeen = time.Now()
		slog.Debug("Agent last seen updated", "agent_id", agentID)
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

	for _, conn := range cm.agents {
		conn.cancel()
		close(conn.SendCh)
	}
	cm.agents = make(map[string]*AgentConnection)
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
			slog.Warn("Removing stale connection", "agent_id", agentID, "last_seen", conn.LastSeen)
			conn.cancel()
			close(conn.SendCh)
			delete(cm.agents, agentID)
		}
	}
}
