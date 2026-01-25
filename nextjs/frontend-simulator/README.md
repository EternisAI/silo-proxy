# Frontend Simulator (Next.js)

A simple Next.js application used for testing the silo-proxy gRPC forwarding functionality.

## Features

- **Homepage**: Simple UI showing service status
- **API Endpoints**:
  - `GET /api/status` - Returns service status and timestamp
  - `POST /api/data` - Accepts JSON data and echoes it back
  - `GET /api/health` - Health check endpoint

## Running

```bash
./run.sh
```

The server will start on http://localhost:3000

**Note**: No BASE_PATH configuration needed for proxy usage. The silo-proxy server handles routing transparently.

## Testing

```bash
# Health check
curl http://localhost:3000/api/health

# Status endpoint
curl http://localhost:3000/api/status

# POST data
curl -X POST -H "Content-Type: application/json" \
  -d '{"test":"data"}' http://localhost:3000/api/data
```

## Via Proxy

When the silo-proxy server and agent are running, you can access this service through the proxy:

```bash
# Root path (routes to agent-1)
curl http://localhost:8080/api/status

# Or use multi-agent routing
curl http://localhost:8080/proxy/agent-1/api/status
```

Or open in browser:
```
http://localhost:8080/
```
