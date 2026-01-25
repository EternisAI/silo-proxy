# Frontend Simulator

A web-based frontend simulator for testing the silo-proxy system.

## Features

- 🎨 Clean web UI for testing proxy requests
- 🔄 Supports GET, POST, PUT, DELETE methods
- 📊 Shows request/response details with timing
- ⚡ Real-time error handling
- 🎯 Easy agent ID and path configuration

## Prerequisites

- Python 3.11+
- [uv](https://github.com/astral-sh/uv) (fast Python package manager)

## Setup

Install dependencies:

```bash
cd py/frontend-simulator
uv sync
```

## Running

### Quick Start

```bash
./run.sh
```

Or manually:

```bash
uv run python main.py
```

The simulator will start on **http://localhost:5000**

## Usage

1. **Start the silo-proxy server** (in another terminal):
   ```bash
   cd ../..
   make run
   ```

2. **Start the agent** (in another terminal):
   ```bash
   make run-agent
   ```

3. **Start a local test service** (in another terminal):
   ```bash
   python3 -m http.server 3000
   ```

4. **Open the simulator** in your browser:
   ```
   http://localhost:5000
   ```

5. **Test requests**:
   - Agent ID: `agent-1`
   - Path: `/test` (or any path on your local service)
   - Method: GET, POST, PUT, DELETE
   - Body: JSON payload for POST/PUT requests

## Example Test Flow

1. GET request to `/test`:
   - Agent ID: `agent-1`
   - Path: `/test`
   - Method: GET
   - Click "Send Request"

2. POST request with JSON:
   - Agent ID: `agent-1`
   - Path: `/api/data`
   - Method: POST
   - Body: `{"name": "test", "value": 123}`
   - Click "Send Request"

## Architecture

```
Browser (Frontend Simulator :5000)
  ↓ HTTP Request
Silo Proxy Server :8080
  ↓ gRPC REQUEST
Agent :9090
  ↓ HTTP Forward
Local Service :3000
  ↓ HTTP Response
Agent → Server → Browser
```

## API Endpoints

### Frontend Simulator

- `GET /` - Web UI
- `POST /api/proxy-request` - Send proxy request
- `GET /health` - Health check

### Request Parameters

- `agent_id` - Target agent ID (e.g., "agent-1")
- `method` - HTTP method (GET, POST, PUT, DELETE)
- `path` - Target path on local service
- `body` - Request body (for POST/PUT)

## Development

The simulator is built with:
- **FastAPI** - Modern async web framework
- **httpx** - Async HTTP client
- **Jinja2** - Template engine
- **uvicorn** - ASGI server
