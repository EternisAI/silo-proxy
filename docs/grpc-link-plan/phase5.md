# Phase 5: Request Forwarding

**Status**: 🔲 Not Started

## Tasks

1. Server accepts HTTP request and converts to REQUEST message
2. Server sends REQUEST to agent via stream
3. Agent forwards request to local service
4. Agent sends RESPONSE back to server
5. Server returns HTTP response to user

## Implementation

### Server: HTTP Handler
```go
func (h *Handler) ProxyRequest(c *gin.Context) {
    body, _ := ioutil.ReadAll(c.Request.Body)

    requestMsg := &proto.ProxyMessage{
        Id:   generateRequestID(),
        Type: proto.MessageType_REQUEST,
        Payload: body,
        Metadata: map[string]string{
            "method": c.Request.Method,
            "path":   c.Request.URL.Path,
        },
    }

    response, err := grpcServer.SendToAgent(c.Request.Context(), requestMsg)
    c.Data(response.StatusCode, response.ContentType, response.Payload)
}
```

### Server: Request-Response Correlation
```go
type Server struct {
    pendingRequests map[string]chan *proto.ProxyMessage
    pendingMu       sync.RWMutex
}

func (s *Server) SendToAgent(ctx context.Context, msg *proto.ProxyMessage) (*proto.ProxyMessage, error) {
    respCh := make(chan *proto.ProxyMessage, 1)
    s.pendingRequests[msg.Id] = respCh

    agent.Send(msg)

    select {
    case response := <-respCh:
        return response, nil
    case <-time.After(30 * time.Second):
        return nil, errors.New("timeout")
    }
}
```

### Agent: Request Handler
```go
type RequestHandler struct {
    httpClient *http.Client
    localURL   string
}

func (rh *RequestHandler) HandleRequest(msg *proto.ProxyMessage) (*proto.ProxyMessage, error) {
    // Build HTTP request
    url := rh.localURL + msg.Metadata["path"]
    req, _ := http.NewRequest(msg.Metadata["method"], url, bytes.NewReader(msg.Payload))

    // Forward to local service
    resp, _ := rh.httpClient.Do(req)
    body, _ := ioutil.ReadAll(resp.Body)

    // Return response
    return &proto.ProxyMessage{
        Id:      msg.Id,
        Type:    proto.MessageType_RESPONSE,
        Payload: body,
        Metadata: map[string]string{
            "status_code": strconv.Itoa(resp.StatusCode),
        },
    }, nil
}
```

### Agent: Message Handler
```go
func (c *Client) handleMessage(msg *proto.ProxyMessage) {
    switch msg.Type {
    case proto.MessageType_REQUEST:
        go func() {
            response, _ := c.requestHandler.HandleRequest(msg)
            c.SendMessage(response)
        }()
    }
}
```

## Config Updates

**Agent** (`application.yml`):
```yaml
grpc:
  server_address: "localhost:9090"
local:
  service_url: "http://localhost:3000"
```

## Verification

```bash
# Terminal 1: Local service on port 3000
python3 -m http.server 3000

# Terminal 2: Start server
./bin/silo-proxy-server

# Terminal 3: Start agent
./bin/silo-proxy-agent

# Terminal 4: Send request
curl http://localhost:8080/api/test
# Should receive response from local service
```
