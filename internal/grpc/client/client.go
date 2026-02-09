package client

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/EternisAI/silo-proxy/proto"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"gopkg.in/yaml.v3"

	grpctls "github.com/EternisAI/silo-proxy/internal/grpc/tls"
)

const (
	sendChannelBuffer = 100
	pingInterval      = 30 * time.Second
	initialDelay      = 1 * time.Second
	maxDelay          = 30 * time.Second
	backoffFactor     = 2
)

type Client struct {
	serverAddr      string
	agentID         string
	provisioningKey string
	configPath      string // Path to config file for persistence
	tlsConfig       *TLSConfig
	conn            *grpc.ClientConn
	stream          proto.ProxyService_StreamClient

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

type TLSConfig struct {
	Enabled            bool
	CertFile           string
	KeyFile            string
	CAFile             string
	ServerNameOverride string
}

func NewClient(serverAddr, agentID, provisioningKey, localURL, configPath string, tlsConfig *TLSConfig) *Client {
	ctx, cancel := context.WithCancel(context.Background())
	return &Client{
		serverAddr:        serverAddr,
		agentID:           agentID,
		provisioningKey:   provisioningKey,
		configPath:        configPath,
		tlsConfig:         tlsConfig,
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

	var opts []grpc.DialOption

	if c.tlsConfig != nil && c.tlsConfig.Enabled {
		creds, err := grpctls.LoadClientCredentials(
			c.tlsConfig.CertFile,
			c.tlsConfig.KeyFile,
			c.tlsConfig.CAFile,
			c.tlsConfig.ServerNameOverride,
		)
		if err != nil {
			return fmt.Errorf("failed to load TLS credentials: %w", err)
		}

		opts = append(opts, grpc.WithTransportCredentials(creds))
		slog.Info("Using TLS connection")
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
		slog.Warn("Using insecure connection (TLS disabled)")
	}

	conn, err := grpc.NewClient(c.serverAddr, opts...)
	if err != nil {
		return fmt.Errorf("failed to dial server: %w", err)
	}

	client := proto.NewProxyServiceClient(conn)
	stream, err := client.Stream(c.ctx)
	if err != nil {
		conn.Close()
		return fmt.Errorf("failed to create stream: %w", err)
	}

	// Build first message with either provisioning_key or agent_id
	firstMsg := &proto.ProxyMessage{
		Id:       uuid.New().String(),
		Type:     proto.MessageType_PING,
		Metadata: make(map[string]string),
	}

	if c.provisioningKey != "" {
		// Provisioning flow: send provisioning_key
		firstMsg.Metadata["provisioning_key"] = c.provisioningKey
		slog.Info("Attempting to provision agent with key")
	} else if c.agentID != "" {
		// Established agent: send agent_id
		firstMsg.Metadata["agent_id"] = c.agentID
		slog.Info("Connecting with agent_id", "agent_id", c.agentID)
	} else {
		stream.CloseSend()
		conn.Close()
		return fmt.Errorf("either agent_id or provisioning_key is required")
	}

	if err := stream.Send(firstMsg); err != nil {
		stream.CloseSend()
		conn.Close()
		return fmt.Errorf("failed to send first message: %w", err)
	}

	// Wait for response if provisioning
	if c.provisioningKey != "" {
		resp, err := stream.Recv()
		if err != nil {
			stream.CloseSend()
			conn.Close()
			return fmt.Errorf("failed to receive provisioning response: %w", err)
		}

		if err := c.handleProvisioningResponse(resp); err != nil {
			stream.CloseSend()
			conn.Close()
			return fmt.Errorf("provisioning failed: %w", err)
		}

		slog.Info("Agent provisioned successfully", "agent_id", c.agentID)
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

func (c *Client) handleProvisioningResponse(msg *proto.ProxyMessage) error {
	status := msg.Metadata["provisioning_status"]
	if status != "success" {
		errorMsg := msg.Metadata["error"]
		if errorMsg == "" {
			errorMsg = "unknown provisioning error"
		}
		return fmt.Errorf("provisioning failed: %s", errorMsg)
	}

	agentID := msg.Metadata["agent_id"]
	if agentID == "" {
		return fmt.Errorf("provisioning response missing agent_id")
	}

	// Update agent ID
	c.mu.Lock()
	c.agentID = agentID
	c.mu.Unlock()

	// Persist to config file
	if c.configPath != "" {
		if err := c.saveAgentIDToConfig(agentID); err != nil {
			slog.Error("Failed to persist agent_id to config", "error", err)
			// Don't fail provisioning, agent can reconnect with same key
		} else {
			slog.Info("Agent ID persisted to config", "config_path", c.configPath)
		}
	}

	return nil
}

func (c *Client) saveAgentIDToConfig(agentID string) error {
	if c.configPath == "" {
		return fmt.Errorf("config path not set")
	}

	// Read current config file
	data, err := os.ReadFile(c.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse YAML
	var config map[string]interface{}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	// Update grpc section
	grpcConfig, ok := config["grpc"].(map[string]interface{})
	if !ok {
		grpcConfig = make(map[string]interface{})
		config["grpc"] = grpcConfig
	}

	// Set agent_id and remove provisioning_key
	grpcConfig["agent_id"] = agentID
	delete(grpcConfig, "provisioning_key")

	// Convert back to YAML
	updatedData, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Add comment at the top
	comment := "# Agent provisioned successfully on " + time.Now().Format(time.RFC3339) + "\n"
	finalData := comment + string(updatedData)

	// Write back to file
	if err := os.WriteFile(c.configPath, []byte(finalData), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	// Clear provisioning_key from memory
	c.mu.Lock()
	c.provisioningKey = ""
	c.mu.Unlock()

	return nil
}

func (c *Client) GetAgentID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.agentID
}
