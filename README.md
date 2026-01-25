# Silo Proxy

A gRPC-based reverse proxy that enables access to services behind NAT/firewalls without port forwarding or VPN configuration.

## The Problem: NAT Traversal

Most devices sit behind NAT (Network Address Translation) - your home router, corporate firewall, etc. NAT allows multiple devices to share one public IP address, but it blocks incoming connections from the internet.

### Why Traditional Approaches Fail

**Scenario**: You want to access a service running on your home computer from the cloud.

```
                                    ┌─────────────┐
                                    │   Router    │
Internet ──────X──────────────────> │    (NAT)    │
               ❌                   │             │
        Cannot connect              └──────┬──────┘
        directly!                          │
                                    ┌──────▼──────┐
                                    │   Silo Box  │
                                    │  (Service)  │
                                    └─────────────┘
```

**Problems**:

- ❌ Router blocks incoming connections
- ❌ No public IP address for your device
- ❌ Port forwarding is complex and insecure
- ❌ VPNs add overhead and configuration complexity

## Our Solution: Agent-Initiated Connection

Instead of trying to connect TO the device behind NAT, we flip it around - the device connects OUT to the cloud server, then we use that existing connection for bidirectional communication.

### Architecture

```
┌──────────┐              ┌─────────────────┐              ┌─────────────┐
│   User   │              │  Cloud Server   │              │   Router    │
│          │              │                 │              │    (NAT)    │
└────┬─────┘              └────┬────────┬───┘              └──────┬──────┘
     │                         │        │                         │
     │  1. HTTP Request        │        │   2. Agent initiates    │
     │     (via proxy)         │        │      gRPC connection    │
     ├────────────────────────>│        │<────────────────────────┤
     │                         │        │      (outbound OK!)     │
     │                         │        │                    ┌────▼─────┐
     │                         │        │   3. Bidirectional │   Agent  │
     │                         │        │      gRPC Stream   │ (Client) │
     │                         │        │<===================│          │
     │                         │        │                    └────┬─────┘
     │                         │        │                         │
     │  6. HTTP Response       │        │   4. Forward request    │
     │<────────────────────────┤        │      to local service   │
     │                         │        │                    ┌────▼─────┐
     │                         │        │   5. Return reply  │  Local   │
     │                         │        │                    │ Service  │
     └─────────────────────────┘        └────────────────────│  :3000   │
                                                             └──────────┘
```

**Key Insight**: Since the agent initiates the connection (outbound), the router/NAT allows it. Once connected, the bidirectional gRPC stream lets the server send requests back through that same connection.

### How It Works

```
Step 1: Agent Connects (Outbound - Always Allowed)
┌────────────┐         gRPC Stream          ┌──────────┐
│   Server   │<═══════════════════════════  │  Agent   │
│  (Cloud)   │                              │ (Behind  │
│  :9090     │                              │   NAT)   │
└────────────┘                              └──────────┘

Step 2: Server Sends Request Over Existing Stream
┌────────────┐                              ┌──────────┐
│   Server   │  ───[REQUEST: GET /api]────> │  Agent   │
└────────────┘                              └──────────┘

Step 3: Agent Forwards to Local Service
                                            ┌──────────┐      ┌─────────┐
                                            │  Agent   │─────>│  Local  │
                                            └──────────┘      │  :3000  │
                                                              └─────────┘

Step 4: Response Returns the Same Way
┌────────────┐                              ┌──────────┐
│   Server   │<───[RESPONSE: 200 OK]─────── │  Agent   │
└────────────┘                              └──────────┘
```

## Use Cases

**Home Lab Access**

```
You (anywhere) → Cloud Server → Your Home Computer → Local Services
                                                      (databases, apps, etc.)
```

**IoT Device Management**

```
Control Panel → Cloud Server → IoT Device (behind NAT) → Sensors/Actuators
```

**Remote Development**

```
Browser → Cloud Server → Dev Machine (behind firewall) → Local Dev Server
```

## Quick Start

### 1. Start Server (Cloud)

```bash
make run
# HTTP: localhost:8080
# gRPC: localhost:9090
```

### 2. Start Agent (Behind NAT)

```bash
make run-agent
# Connects to server, forwards to localhost:3000
```

### 3. Start Local Service

```bash
cd nextjs/frontend-simulator && ./run-proxy.sh
# Service runs on :3000
```

### 4. Access via Proxy

```bash
# User accesses through server
curl http://localhost:8080/proxy/agent-1/api/status

# Request flows: User → Server → Agent → Local Service → Agent → Server → User
```

## Features

- ✅ **Zero Configuration**: No port forwarding, no VPN, no static IP needed
- ✅ **NAT Traversal**: Works through any router/firewall automatically
- ✅ **Auto Reconnect**: Exponential backoff (1s → 30s) if connection drops
- ✅ **Keep-Alive**: PING/PONG every 30s to detect dead connections
- ✅ **Graceful Shutdown**: Coordinated cleanup on termination
- ✅ **Low Latency**: ~1ms average overhead for request forwarding

## Configuration

**Server** (`cmd/silo-proxy-server/application.yml`):

```yaml
server:
  port: 8080
grpc:
  port: 9090
```

**Agent** (`cmd/silo-proxy-agent/application.yml`):

```yaml
grpc:
  server_address: "localhost:9090"
  agent_id: "agent-1"
local:
  service_url: "http://localhost:3000"
```

## Message Types

The system uses 4 message types over the gRPC stream:

```
Agent → Server:  PING      (keep-alive heartbeat)
Server → Agent:  PONG      (keep-alive response)
Server → Agent:  REQUEST   (HTTP request to forward)
Agent → Server:  RESPONSE  (HTTP response to return)
```
