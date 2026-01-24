# Phase 3: Agent Implementation

**Status**: 🔲 Not Started

## Tasks

1. Create gRPC client (`internal/grpc/client/client.go`)
2. Implement stream handler (`internal/grpc/client/stream_handler.go`)
3. Add reconnection logic (exponential backoff)
4. Update agent main.go
5. Update agent config

## Key Components

### Client (`internal/grpc/client/client.go`)
```go
type Client struct {
    serverAddr string
    conn       *grpc.ClientConn
    stream     proto.ProxyService_StreamClient
    logger     *logger.Logger
    sendCh     chan *proto.ProxyMessage
    stopCh     chan struct{}
}

func (c *Client) Connect() error
func (c *Client) reconnect() error  // Exponential backoff: 1s → 30s max
```

### Config Update (`cmd/silo-proxy-agent/config.go`)
```go
type GRPCConfig struct {
    ServerAddress string `mapstructure:"server_address"`
}
```

### Main Update (`cmd/silo-proxy-agent/main.go`)
```go
// Start gRPC client on startup
grpcClient := client.NewClient(config.GRPC.ServerAddress, logger)
go grpcClient.Connect()
```

## Verification

```bash
# Terminal 1: Start server
./bin/silo-proxy-server

# Terminal 2: Start agent
./bin/silo-proxy-agent
# Should see: "Connected to server at localhost:9090"
```
