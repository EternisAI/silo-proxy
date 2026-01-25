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

Or manually:

```bash
npm run dev
```

The server will start on http://localhost:3000

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
curl http://localhost:8080/proxy/agent-1/api/status
```

Or open in browser:
```
http://localhost:8080/proxy/agent-1/
```
