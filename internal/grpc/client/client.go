package client

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/EternisAI/silo-proxy/proto"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	sendChannelBuffer = 100
	pingInterval      = 30 * time.Second
	initialDelay      = 1 * time.Second
	maxDelay          = 30 * time.Second
	backoffFactor     = 2
)

type Client struct {
	serverAddr string
	agentID    string
	conn       *grpc.ClientConn
	stream     proto.ProxyService_StreamClient

	sendCh chan *proto.ProxyMessage
	stopCh chan struct{}
	doneCh chan struct{}

	reconnectDelay    time.Duration
	maxReconnectDelay time.Duration

	requestHandler *RequestHandler

	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.RWMutex
}

func NewClient(serverAddr, agentID, localURL string) *Client {
	ctx, cancel := context.WithCancel(context.Background())
	return &Client{
		serverAddr:        serverAddr,
		agentID:           agentID,
		sendCh:            make(chan *proto.ProxyMessage, sendChannelBuffer),
		stopCh:            make(chan struct{}),
		doneCh:            make(chan struct{}),
		reconnectDelay:    initialDelay,
		maxReconnectDelay: maxDelay,
		requestHandler:    NewRequestHandler(localURL),
		ctx:               ctx,
		cancel:            cancel,
	}
}

func (c *Client) Start() error {
	go c.connectionLoop()
	return nil
}

func (c *Client) Stop() error {
	slog.Info("Stopping gRPC client")
	close(c.stopCh)
	c.cancel()
	<-c.doneCh
	slog.Info("gRPC client stopped")
	return nil
}

func (c *Client) Send(msg *proto.ProxyMessage) error {
	select {
	case c.sendCh <- msg:
		return nil
	default:
		return fmt.Errorf("send channel full")
	}
}

func (c *Client) connectionLoop() {
	defer close(c.doneCh)

	for {
		select {
		case <-c.stopCh:
			c.disconnect()
			return
		default:
			if err := c.connect(); err != nil {
				slog.Error("Connection failed", "error", err, "retry_in", c.reconnectDelay)
				select {
				case <-time.After(c.reconnectDelay):
					c.increaseReconnectDelay()
					continue
				case <-c.stopCh:
					return
				}
			}

			c.reconnectDelay = initialDelay

			if err := c.handleStream(); err != nil {
				if err == io.EOF {
					slog.Info("Server closed connection")
				} else {
					slog.Error("Stream error", "error", err)
				}
			}

			c.disconnect()

			select {
			case <-c.stopCh:
				return
			default:
				slog.Info("Reconnecting", "delay", c.reconnectDelay)
				time.Sleep(c.reconnectDelay)
				c.increaseReconnectDelay()
			}
		}
	}
}

func (c *Client) connect() error {
	slog.Info("Connecting to server", "address", c.serverAddr)

	conn, err := grpc.NewClient(c.serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to dial server: %w", err)
	}

	client := proto.NewProxyServiceClient(conn)
	stream, err := client.Stream(c.ctx)
	if err != nil {
		conn.Close()
		return fmt.Errorf("failed to create stream: %w", err)
	}

	firstMsg := &proto.ProxyMessage{
		Id:   uuid.New().String(),
		Type: proto.MessageType_PING,
		Metadata: map[string]string{
			"agent_id": c.agentID,
		},
	}

	if err := stream.Send(firstMsg); err != nil {
		stream.CloseSend()
		conn.Close()
		return fmt.Errorf("failed to send first message: %w", err)
	}

	c.mu.Lock()
	c.conn = conn
	c.stream = stream
	c.mu.Unlock()

	slog.Info("Connected to server", "address", c.serverAddr)
	return nil
}

func (c *Client) disconnect() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stream != nil {
		c.stream.CloseSend()
		c.stream = nil
	}

	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
}

func (c *Client) increaseReconnectDelay() {
	c.reconnectDelay = c.reconnectDelay * backoffFactor
	if c.reconnectDelay > c.maxReconnectDelay {
		c.reconnectDelay = c.maxReconnectDelay
	}
}

func (c *Client) handleStream() error {
	done := make(chan struct{})
	errChan := make(chan error, 3)

	go c.receiveLoop(done, errChan)
	go c.sendLoop(done, errChan)
	go c.pingLoop(done, errChan)

	err := <-errChan
	close(done)
	return err
}

func (c *Client) receiveLoop(done chan struct{}, errChan chan error) {
	for {
		select {
		case <-done:
			return
		default:
			c.mu.RLock()
			stream := c.stream
			c.mu.RUnlock()

			if stream == nil {
				errChan <- fmt.Errorf("stream is nil")
				return
			}

			msg, err := stream.Recv()
			if err != nil {
				if err != io.EOF {
					slog.Error("Error receiving message", "error", err)
				}
				errChan <- err
				return
			}

			slog.Debug("Message received", "message_id", msg.Id, "type", msg.Type)

			if err := c.processMessage(msg); err != nil {
				slog.Error("Failed to process message", "error", err)
			}
		}
	}
}

func (c *Client) sendLoop(done chan struct{}, errChan chan error) {
	for {
		select {
		case <-done:
			return
		case msg, ok := <-c.sendCh:
			if !ok {
				return
			}

			c.mu.RLock()
			stream := c.stream
			c.mu.RUnlock()

			if stream == nil {
				errChan <- fmt.Errorf("stream is nil")
				return
			}

			slog.Debug("Sending message", "message_id", msg.Id, "type", msg.Type)

			if err := stream.Send(msg); err != nil {
				slog.Error("Error sending message", "error", err)
				errChan <- err
				return
			}
		}
	}
}

func (c *Client) pingLoop(done chan struct{}, errChan chan error) {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			ping := &proto.ProxyMessage{
				Id:       uuid.New().String(),
				Type:     proto.MessageType_PING,
				Metadata: map[string]string{},
			}

			if err := c.Send(ping); err != nil {
				slog.Error("Failed to send PING", "error", err)
				errChan <- err
				return
			}

			slog.Debug("PING sent", "message_id", ping.Id)
		}
	}
}

func (c *Client) processMessage(msg *proto.ProxyMessage) error {
	switch msg.Type {
	case proto.MessageType_PONG:
		slog.Debug("PONG received", "message_id", msg.Id)

	case proto.MessageType_REQUEST:
		slog.Debug("REQUEST received", "message_id", msg.Id)
		go c.handleRequest(msg)

	default:
		slog.Warn("Unknown message type", "type", msg.Type)
	}

	return nil
}

func (c *Client) handleRequest(msg *proto.ProxyMessage) {
	response, err := c.requestHandler.HandleRequest(msg)
	if err != nil {
		slog.Error("Failed to handle request", "error", err, "message_id", msg.Id)

		errorResponse := &proto.ProxyMessage{
			Id:      msg.Id,
			Type:    proto.MessageType_RESPONSE,
			Payload: []byte(err.Error()),
			Metadata: map[string]string{
				"status_code": "502",
				"error":       err.Error(),
			},
		}

		if sendErr := c.Send(errorResponse); sendErr != nil {
			slog.Error("Failed to send error response", "error", sendErr)
		}
		return
	}

	if err := c.Send(response); err != nil {
		slog.Error("Failed to send response", "error", err, "message_id", msg.Id)
	}
}
