# Phase 5: Request Forwarding

**Status**: ✅ **COMPLETED**

## Tasks

1. ✅ Server accepts HTTP request and converts to REQUEST message
2. ✅ Server sends REQUEST to agent via stream
3. ✅ Agent forwards request to local service
4. ✅ Agent sends RESPONSE back to server
5. ✅ Server returns HTTP response to user

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
make run

# Terminal 3: Start agent
make run-agent

# Terminal 4: Send request
curl http://localhost:8080/proxy/agent-1/test
# Should receive response from local service
```

## Implementation Notes

**Completed**: 2026-01-25

### What Was Implemented

1. **Server-Side Proxy Handler** (`internal/api/http/handler/proxy.go`):
   - Accepts HTTP requests at `/proxy/:agent_id/*path`
   - Converts HTTP request to gRPC REQUEST message
   - Implements request-response correlation with pending requests map
   - Waits for RESPONSE with 30-second timeout
   - Returns HTTP response to client

2. **Request-Response Correlation** (`internal/grpc/server/server.go`):
   - Added `pendingRequests` map to track in-flight requests
   - `SendRequestToAgent()` method sends request and waits for response
   - `HandleResponse()` method routes responses to waiting channels
   - Thread-safe with RWMutex protection

3. **Agent Request Handler** (`internal/grpc/client/request_handler.go`):
   - Receives REQUEST messages from server
   - Forwards HTTP request to local service
   - Converts HTTP response to gRPC RESPONSE message
   - Handles headers, query parameters, and request body
   - 30-second timeout for local HTTP requests

4. **Configuration** (`cmd/silo-proxy-agent/application.yml`):
   ```yaml
   local:
     service_url: http://localhost:3000
   ```

5. **Integration**:
   - Updated agent main.go to pass local service URL to client
   - Updated server router to add proxy endpoint
   - Updated stream handler to process RESPONSE messages
   - Client handles REQUEST messages in processMessage()

### Verified Behaviors

✅ **GET Requests**:
```bash
curl http://localhost:8080/proxy/agent-1/test
# Response: {"message": "Hello from local service", "path": "/test", "method": "GET"}
```

✅ **POST Requests with JSON**:
```bash
curl -X POST -H "Content-Type: application/json" \
  -d '{"name":"test","value":123}' \
  http://localhost:8080/proxy/agent-1/api/data
# Response: {"message": "POST received", "path": "/api/data", "method": "POST", "body": "{\"name\":\"test\",\"value\":123}"}
```

✅ **Query Parameters**:
```bash
curl "http://localhost:8080/proxy/agent-1/search?q=test&page=1"
# Response: {"message": "Hello from local service", "path": "/search?q=test&page=1", "method": "GET"}
```

✅ **Complete Flow**:
- User → Server (HTTP) → Server (gRPC REQUEST) → Agent → Local Service
- Local Service → Agent → Server (gRPC RESPONSE) → Server (HTTP) → User
- Average latency: ~1ms for simple requests

### Architecture

```
User Request (HTTP)
  ↓
Server :8080 (HTTP Handler)
  ↓ Convert to ProxyMessage
Server :9090 (gRPC Server)
  ↓ Send REQUEST via stream
Agent (gRPC Client)
  ↓ Forward to local service
Local Service :3000
  ↓ HTTP Response
Agent (gRPC Client)
  ↓ Send RESPONSE via stream
Server :9090 (gRPC Server)
  ↓ Route to pending request
Server :8080 (HTTP Handler)
  ↓ Return HTTP Response
User Response (HTTP)
```
