package server

import (
	"fmt"
	"io"
	"log/slog"

	"github.com/EternisAI/silo-proxy/proto"
	"github.com/google/uuid"
)

type StreamHandler struct {
	connManager *ConnectionManager
}

func NewStreamHandler(connManager *ConnectionManager) *StreamHandler {
	return &StreamHandler{
		connManager: connManager,
	}
}

func (sh *StreamHandler) HandleStream(stream proto.ProxyService_StreamServer) error {
	firstMsg, err := stream.Recv()
	if err != nil {
		return fmt.Errorf("failed to receive first message: %w", err)
	}

	agentID := firstMsg.Metadata["agent_id"]
	if agentID == "" {
		return fmt.Errorf("agent_id not found in first message metadata")
	}

	slog.Info("Agent connection established", "agent_id", agentID)

	conn, err := sh.connManager.Register(agentID, stream)
	if err != nil {
		return fmt.Errorf("failed to register agent: %w", err)
	}

	defer func() {
		sh.connManager.Deregister(agentID)
		slog.Info("Agent disconnected", "agent_id", agentID)
	}()

	sh.connManager.UpdateLastSeen(agentID)

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

	default:
		slog.Warn("Unknown message type", "agent_id", agentID, "type", msg.Type)
	}

	return nil
}
