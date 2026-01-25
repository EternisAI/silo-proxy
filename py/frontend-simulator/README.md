# Frontend Simulator

A simple web app that runs behind the silo-proxy agent to demonstrate the proxy system.

## Architecture

```
User Browser
  ↓
Silo Proxy Server :8080
  ↓ gRPC
Agent
  ↓ HTTP
Frontend Simulator :3000 (this app)
```

## Setup

```bash
uv sync
```

## Running

```bash
./run.sh
```

The app runs on port **3000** (not directly accessible from outside the network).

## Access

Users access this app through the proxy:

```
http://localhost:8080/proxy/agent-1/
```

## Features

- Clean web interface showing proxy connection
- API endpoints for testing
- Demonstrates request forwarding through the proxy

## Endpoints

- `GET /` - Web UI
- `GET /api/status` - Status check
- `POST /api/data` - Test POST endpoint
- `GET /health` - Health check
