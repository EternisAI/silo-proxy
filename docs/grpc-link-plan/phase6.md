# Phase 6: Encrypt gRPC Link with TLS/mTLS

## Overview

Phase 6 adds TLS/mTLS encryption to the gRPC communication channel between the server and agents. This secures the connection and provides mutual authentication.

## Implementation

### mTLS Approach

We chose **Mutual TLS (mTLS)** over one-way TLS because:
- Agents connect from remote boxes over potentially untrusted networks
- mTLS provides mutual authentication (server authenticates agent, agent authenticates server)
- Aligns with existing agent_id architecture
- Minimal complexity increase over one-way TLS

### Certificate Structure

The system uses a self-signed CA for development:

```
certs/
├── ca/
│   ├── ca-cert.pem          # CA certificate (shared by all)
│   └── ca-key.pem           # CA private key
├── server/
│   ├── server-cert.pem      # Server certificate (CN=server)
│   └── server-key.pem       # Server private key
└── agents/
    ├── agent-1-cert.pem     # Agent certificates (CN=agent-N)
    ├── agent-1-key.pem
    ├── agent-2-cert.pem
    └── ...
```

## Configuration

### Server Configuration

```yaml
grpc:
  port: 9090
  tls:
    enabled: true
    cert_file: ./certs/server/server-cert.pem
    key_file: ./certs/server/server-key.pem
    ca_file: ./certs/ca/ca-cert.pem
    client_auth: require  # Options: none, request, require
```

### Agent Configuration

```yaml
grpc:
  server_address: localhost:9090
  agent_id: agent-1
  tls:
    enabled: true
    cert_file: ./certs/agents/agent-1-cert.pem
    key_file: ./certs/agents/agent-1-key.pem
    ca_file: ./certs/ca/ca-cert.pem
    server_name_override: ""  # Optional: override server name for verification
```

## Usage

### Generate Certificates

For development, generate self-signed certificates:

```bash
make generate-certs
```

This creates a CA, server certificate, and 3 agent certificates in the `certs/` directory.

### Enable TLS

1. Update `cmd/silo-proxy-server/application.yml` - set `grpc.tls.enabled: true`
2. Update `cmd/silo-proxy-agent/application.yml` - set `grpc.tls.enabled: true`
3. Ensure certificate paths are correct
4. Start server: `make run`
5. Start agent: `make run-agent`

### Verify TLS is Working

Check the logs:
- Server should log: `"Starting gRPC server with TLS"`
- Agent should log: `"Using TLS connection"`
- Connection should establish successfully

### Disable TLS (Backward Compatibility)

Set `tls.enabled: false` in both configuration files to use insecure connections. This is useful for local development without certificates.

## Testing

### Manual Test Flow

1. Generate certificates: `make generate-certs`
2. Enable TLS in both application.yml files
3. Start server: `make run`
4. Start agent: `make run-agent`
5. Start local service: `cd nextjs/frontend-simulator && ./run.sh`
6. Test request: `curl http://localhost:8080/api/status`
7. Verify:
   - Server logs show "Starting gRPC server with TLS"
   - Agent logs show "Using TLS connection"
   - Agent logs show "Connected to server"
   - Request returns 200 OK
   - PING/PONG messages continue
   - Graceful shutdown works

### Test Backward Compatibility

1. Set `tls.enabled: false` in both configs
2. Restart server and agent
3. Verify system still works with insecure connections
4. Check for warning logs about insecure mode

## Implementation Details

### New Components

1. **TLS Helper Package** (`internal/grpc/tls/credentials.go`)
   - `LoadServerCredentials()` - Loads server certificate and CA
   - `LoadClientCredentials()` - Loads client certificate and CA
   - `ParseClientAuthType()` - Converts config string to TLS client auth type

2. **Certificate Generation Script** (`scripts/generate-certs.sh`)
   - Generates self-signed CA
   - Generates server certificate with SAN (localhost, 127.0.0.1)
   - Generates agent certificates (CN=agent-N)

3. **Configuration Updates**
   - Added `TLSConfig` structs to both server and agent configs
   - Wired TLS config through to gRPC server/client initialization

### Modified Components

1. **Server** (`internal/grpc/server/server.go`)
   - Added TLS config field
   - Updated `NewServer()` to accept TLS config
   - Updated `Start()` to conditionally use TLS credentials

2. **Client** (`internal/grpc/client/client.go`)
   - Added TLS config field
   - Updated `NewClient()` to accept TLS config
   - Updated `connect()` to conditionally use TLS credentials

## Security Considerations

### Development Certificates

The generated certificates are **self-signed** and intended for **development only**:
- CA private key is not password-protected
- Certificates have 365-day validity
- No certificate revocation mechanism
- Not suitable for production use

### Production Deployment

For production, use proper certificates:
- Use a trusted CA (Let's Encrypt, internal PKI, etc.)
- Implement certificate rotation
- Protect private keys with proper permissions
- Consider using certificate management tools (cert-manager, Vault, etc.)
- Monitor certificate expiration

### Client Authentication Modes

The server supports three client authentication modes:
- `none` - No client certificate required (one-way TLS only)
- `request` - Client certificate requested but not required
- `require` - Client certificate required and verified (mTLS)

For production, use `require` for maximum security.

## Troubleshooting

### Certificate Errors

If you see certificate errors:
1. Verify certificate files exist at configured paths
2. Check certificate validity: `openssl x509 -in cert.pem -text -noout`
3. Verify CA certificate matches between server and agents
4. Check file permissions on certificate files

### Connection Refused

If agent can't connect:
1. Verify server is running with TLS enabled
2. Check server address in agent config
3. Verify server certificate has correct SAN entries
4. Check firewall rules allow traffic on gRPC port

### Certificate Validation Errors

If you see "certificate signed by unknown authority":
1. Verify both server and agent use the same CA certificate
2. Check CA file path in configuration
3. Regenerate certificates if necessary

### Server Name Mismatch

If you see "server name does not match certificate":
1. Use `server_name_override` in agent config to override verification
2. Or regenerate server certificate with correct SAN entries
3. For localhost testing, ensure certificate includes "localhost" and "127.0.0.1"

## Future Enhancements

Potential improvements for future phases:
- Certificate rotation without downtime
- Integration with external certificate management systems
- Certificate revocation checking
- Automatic certificate renewal
- Support for hardware security modules (HSM)
- Per-agent certificate validation and authorization
