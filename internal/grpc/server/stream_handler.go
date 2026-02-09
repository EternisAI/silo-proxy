package server

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/EternisAI/silo-proxy/internal/agents"
	"github.com/EternisAI/silo-proxy/internal/provisioning"
	"github.com/EternisAI/silo-proxy/proto"
	"github.com/google/uuid"
)

type StreamHandler struct {
	connManager         *ConnectionManager
	server              *Server
	provisioningService *provisioning.Service
	agentService        *agents.Service
}

func NewStreamHandler(
	connManager *ConnectionManager,
	server *Server,
	provisioningService *provisioning.Service,
	agentService *agents.Service,
) *StreamHandler {
	return &StreamHandler{
		connManager:         connManager,
		server:              server,
		provisioningService: provisioningService,
		agentService:        agentService,
	}
}

func (sh *StreamHandler) HandleStream(stream proto.ProxyService_StreamServer) error {
	ctx := stream.Context()

	firstMsg, err := stream.Recv()
	if err != nil {
		return fmt.Errorf("failed to receive first message: %w", err)
	}

	// Extract remote IP from context (if available)
	remoteIP := extractRemoteIP(ctx)

	// Extract provisioning key and agent_id from metadata
	provisioningKey := firstMsg.Metadata["provisioning_key"]
	agentID := firstMsg.Metadata["agent_id"]

	// Connection log ID for tracking
	var connectionLogID string

	if provisioningKey != "" {
		// NEW: Provisioning flow
		slog.Info("Agent provisioning request received", "remote_ip", remoteIP)

		result, err := sh.provisioningService.ProvisionAgent(ctx, provisioningKey, remoteIP)
		if err != nil {
			sh.sendProvisioningError(stream, err)
			return fmt.Errorf("provisioning failed: %w", err)
		}

		sh.sendProvisioningSuccess(stream, result)
		agentID = result.AgentID

		slog.Info("Agent provisioned successfully", "agent_id", agentID, "remote_ip", remoteIP)

	} else if agentID != "" {
		// Established agent: validate against DB
		agent, err := sh.agentService.GetAgentByID(ctx, agentID)
		if err != nil {
			return fmt.Errorf("failed to get agent: %w", err)
		}

		if agent.Status != "active" {
			slog.Warn("Agent connection rejected, status not active",
				"agent_id", agentID,
				"status", agent.Status)
			return fmt.Errorf("agent suspended or inactive")
		}

		slog.Info("Agent authenticated", "agent_id", agentID, "remote_ip", remoteIP)
	} else {
		return fmt.Errorf("either provisioning_key or agent_id required in first message metadata")
	}

	// Create connection log
	logID, err := sh.agentService.CreateConnectionLog(ctx, agentID, time.Now(), remoteIP)
	if err != nil {
		slog.Error("Failed to create connection log", "agent_id", agentID, "error", err)
		// Don't fail the connection, just log the error
	} else {
		connectionLogID = logID
	}

	slog.Info("Agent connection established", "agent_id", agentID)

	// Register with ConnectionManager
	conn, err := sh.connManager.Register(agentID, stream)
	if err != nil {
		return fmt.Errorf("failed to register agent: %w", err)
	}

	defer func() {
		sh.connManager.Deregister(agentID)

		// Update connection log with disconnect information
		if connectionLogID != "" {
			if err := sh.agentService.UpdateConnectionLog(ctx, connectionLogID, time.Now(), "normal disconnect"); err != nil {
				slog.Error("Failed to update connection log", "log_id", connectionLogID, "error", err)
			}
		}

		slog.Info("Agent disconnected", "agent_id", agentID)
	}()

	sh.connManager.UpdateLastSeen(agentID)

	// Update agent last seen in database (async)
	go func() {
		if err := sh.agentService.UpdateLastSeen(context.Background(), agentID, time.Now(), remoteIP); err != nil {
			slog.Error("Failed to update agent last seen", "agent_id", agentID, "error", err)
		}
	}()

	if err := sh.processMessage(agentID, firstMsg); err != nil {
		slog.Error("Failed to process first message", "agent_id", agentID, "error", err)
	}

	done := make(chan struct{})
	errChan := make(chan error, 2)

	go sh.receiveLoop(agentID, stream, done, errChan)
	go sh.sendLoop(agentID, stream, conn.SendCh, done, errChan)

	select {
	case err := <-errChan:
		close(done)
		if err != nil && err != io.EOF {
			return err
		}
		return nil
	case <-conn.ctx.Done():
		close(done)
		return conn.ctx.Err()
	}
}

func (sh *StreamHandler) receiveLoop(agentID string, stream proto.ProxyService_StreamServer, done chan struct{}, errChan chan error) {
	for {
		select {
		case <-done:
			return
		default:
			msg, err := stream.Recv()
			if err != nil {
				if err != io.EOF {
					slog.Error("Error receiving message", "agent_id", agentID, "error", err)
				}
				errChan <- err
				return
			}

			slog.Debug("Message received", "agent_id", agentID, "message_id", msg.Id, "type", msg.Type)

			sh.connManager.UpdateLastSeen(agentID)

			if err := sh.processMessage(agentID, msg); err != nil {
				slog.Error("Failed to process message", "agent_id", agentID, "error", err)
			}
		}
	}
}

func (sh *StreamHandler) sendLoop(agentID string, stream proto.ProxyService_StreamServer, sendCh chan *proto.ProxyMessage, done chan struct{}, errChan chan error) {
	for {
		select {
		case <-done:
			return
		case msg, ok := <-sendCh:
			if !ok {
				return
			}

			slog.Debug("Sending message", "agent_id", agentID, "message_id", msg.Id, "type", msg.Type)

			if err := stream.Send(msg); err != nil {
				slog.Error("Error sending message", "agent_id", agentID, "error", err)
				errChan <- err
				return
			}
		}
	}
}

func (sh *StreamHandler) processMessage(agentID string, msg *proto.ProxyMessage) error {
	switch msg.Type {
	case proto.MessageType_PING:
		slog.Debug("PING received", "agent_id", agentID, "message_id", msg.Id)

		pong := &proto.ProxyMessage{
			Id:       uuid.New().String(),
			Type:     proto.MessageType_PONG,
			Metadata: map[string]string{},
		}

		if err := sh.connManager.SendToAgent(agentID, pong); err != nil {
			return fmt.Errorf("failed to send PONG: %w", err)
		}

		slog.Debug("PONG sent", "agent_id", agentID, "message_id", pong.Id)

	case proto.MessageType_RESPONSE:
		slog.Debug("RESPONSE received", "agent_id", agentID, "message_id", msg.Id)
		sh.server.HandleResponse(msg)

	default:
		slog.Warn("Unknown message type", "agent_id", agentID, "type", msg.Type)
	}

	return nil
}

func (sh *StreamHandler) sendProvisioningSuccess(stream proto.ProxyService_StreamServer, result *provisioning.AgentProvisionResult) error {
	msg := &proto.ProxyMessage{
		Id:   uuid.New().String(),
		Type: proto.MessageType_PONG,
		Metadata: map[string]string{
			"provisioning_status": "success",
			"agent_id":            result.AgentID,
		},
	}

	if result.CertFingerprint != "" {
		msg.Metadata["cert_fingerprint"] = result.CertFingerprint
	}

	if err := stream.Send(msg); err != nil {
		return fmt.Errorf("failed to send provisioning success: %w", err)
	}

	return nil
}

func (sh *StreamHandler) sendProvisioningError(stream proto.ProxyService_StreamServer, err error) {
	msg := &proto.ProxyMessage{
		Id:   uuid.New().String(),
		Type: proto.MessageType_PONG,
		Metadata: map[string]string{
			"provisioning_status": "failed",
			"error":               err.Error(),
		},
	}

	if sendErr := stream.Send(msg); sendErr != nil {
		slog.Error("Failed to send provisioning error message", "error", sendErr)
	}
}

func extractRemoteIP(ctx context.Context) string {
	// This is a placeholder - actual implementation depends on gRPC metadata
	// For now, return empty string
	return ""
}
