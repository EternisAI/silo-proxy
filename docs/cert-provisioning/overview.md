# Automated Certificate Provisioning

## Overview

Automate mTLS certificate issuance for agents using a provision key workflow. Instead of manually generating and distributing certificates, an admin creates a one-time provision key on the server, shares it with the agent operator, and the agent uses it to obtain a signed certificate via a REST API.

The agent's private key never leaves the device (CSR-based signing).

## Motivation

Current certificate workflow requires manual steps:
1. Run `make generate-certs` on the server
2. Copy `agent-cert.pem`, `agent-key.pem`, and `ca-cert.pem` to each agent device
3. Configure the agent with cert file paths

This doesn't scale. Each new agent requires SSH access to the server, manual cert generation, and secure file transfer. The provisioning API eliminates this by letting agents self-enroll with a pre-authorized key.

## Architecture

### Flow

```
 Admin                    Server                     Agent Device
   |                        |                            |
   | POST /api/v1/          |                            |
   |   provision-keys       |                            |
   | {agent_id: "agent-5"}  |                            |
   |----------------------->|                            |
   |   provision_key:       |                            |
   |   "sk_a1b2c3..."      |                            |
   |<-----------------------|                            |
   |                        |                            |
   |  (copy key to device)  |                            |
   |~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~>|
   |                        |                            |
   |                        |  1. Generate private key   |
   |                        |  2. Create CSR             |
   |                        |                            |
   |                        |  POST /api/v1/provision    |
   |                        |  {key, csr}                |
   |                        |<---------------------------|
   |                        |                            |
   |                        |  3. Validate key           |
   |                        |  4. Sign CSR with CA       |
   |                        |  5. Mark key as used       |
   |                        |                            |
   |                        |  {agent_cert, ca_cert}     |
   |                        |--------------------------->|
   |                        |                            |
   |                        |  6. Save certs to disk     |
   |                        |  7. Connect via mTLS       |
   |                        |<===========================|
   |                        |     (gRPC stream)          |
```

### Design Decisions

**CSR-based signing** (not server-generated keys): The agent generates its own private key locally and sends only a Certificate Signing Request. The private key never crosses the network.

**One-time use keys**: Each provision key can only be used once. After successful provisioning, the key is marked as consumed. This prevents replay attacks.

**Key expiry**: Provision keys expire after a configurable TTL (default: 24 hours). Unused keys are cleaned up automatically.

**Agent ID embedded in certificate CN**: The server sets the certificate's Common Name to the agent ID when signing. During mTLS handshake, the server can verify agent identity directly from the certificate rather than trusting a metadata field.

**Provision API uses standard TLS (not mTLS)**: The agent doesn't have certificates yet at provisioning time, so the provision endpoint must be accessible without client certs. Use standard HTTPS for transport security.

## API Design

### Create Provision Key

Admin creates a key tied to a specific agent ID.

```
POST /api/v1/provision-keys
```

Request:
```json
{
  "agent_id": "agent-5",
  "ttl_hours": 24
}
```

Response:
```json
{
  "provision_key": "sk_a1b2c3d4e5f6...",
  "agent_id": "agent-5",
  "expires_at": "2026-02-10T12:00:00Z"
}
```

### List Provision Keys

Admin lists active (unused, unexpired) provision keys.

```
GET /api/v1/provision-keys
```

Response:
```json
{
  "keys": [
    {
      "agent_id": "agent-5",
      "expires_at": "2026-02-10T12:00:00Z",
      "used": false
    }
  ]
}
```

The `provision_key` value is not returned in list responses (write-only).

### Revoke Provision Key

Admin revokes a key before it's used.

```
DELETE /api/v1/provision-keys/:agent_id
```

### Provision Agent Certificate

Agent exchanges provision key + CSR for a signed certificate.

```
POST /api/v1/provision
```

Request:
```json
{
  "provision_key": "sk_a1b2c3d4e5f6...",
  "csr": "-----BEGIN CERTIFICATE REQUEST-----\nMIIE..."
}
```

Response:
```json
{
  "agent_id": "agent-5",
  "agent_cert": "-----BEGIN CERTIFICATE-----\nMIIF...",
  "ca_cert": "-----BEGIN CERTIFICATE-----\nMIID..."
}
```

Error responses:
```json
{"error": "invalid or expired provision key"}
{"error": "provision key already used"}
{"error": "invalid CSR format"}
```

## Data Model

### ProvisionKey

```go
type ProvisionKey struct {
    Key       string    // random 32-byte hex token, prefixed "sk_"
    AgentID   string    // which agent this key provisions
    ExpiresAt time.Time // auto-expires after TTL
    Used      bool      // one-time use flag
    CreatedAt time.Time
}
```

### ProvisionKeyStore

```go
type ProvisionKeyStore struct {
    keys map[string]*ProvisionKey // key string -> ProvisionKey
    mu   sync.RWMutex
}

func (s *ProvisionKeyStore) Create(agentID string, ttl time.Duration) *ProvisionKey
func (s *ProvisionKeyStore) Validate(key string) (*ProvisionKey, error)
func (s *ProvisionKeyStore) MarkUsed(key string)
func (s *ProvisionKeyStore) Revoke(agentID string)
func (s *ProvisionKeyStore) List() []*ProvisionKey
func (s *ProvisionKeyStore) CleanupExpired()
```

## Agent CLI

The agent adds a `provision` subcommand:

```bash
silo-proxy-agent provision \
  --server https://server:8080 \
  --key sk_a1b2c3d4e5f6... \
  --cert-dir ~/.silo-proxy/certs
```

This command:
1. Generates an RSA 4096-bit private key
2. Creates a CSR from the key
3. Calls `POST /api/v1/provision` with the key + CSR
4. Saves the returned files:
   ```
   ~/.silo-proxy/certs/
   ├── agent-key.pem    # generated locally
   ├── agent-cert.pem   # signed by server CA
   └── ca-cert.pem      # for verifying server
   ```
5. Prints the config snippet for `application.yml`

After provisioning, the agent connects normally:

```bash
silo-proxy-agent --config ~/.silo-proxy/application.yml
```

## Server-Side CA Management

The server needs access to the CA private key to sign CSRs. New config fields:

```yaml
grpc:
  tls:
    enabled: true
    cert_file: "certs/server/server-cert.pem"
    key_file: "certs/server/server-key.pem"
    ca_file: "certs/ca/ca-cert.pem"
    ca_key_file: "certs/ca/ca-key.pem"   # NEW: needed for signing
    client_auth: "require"

provision:
  enabled: true
  key_ttl_hours: 24          # default TTL for provision keys
  cert_validity_days: 365    # validity period for issued certs
```

### Certificate Signing

```go
func SignCSR(csrPEM []byte, caCert *x509.Certificate, caKey crypto.PrivateKey, agentID string, validityDays int) ([]byte, error) {
    // 1. Parse CSR
    // 2. Verify CSR signature (proves agent owns the private key)
    // 3. Create x509 certificate template:
    //    - Subject CN = agentID (overrides CSR subject)
    //    - Serial number = random
    //    - NotBefore = now
    //    - NotAfter = now + validityDays
    // 4. Sign with CA private key
    // 5. Return PEM-encoded certificate
}
```

The server overrides the CSR's subject CN with the agent ID from the provision key. This prevents an agent from claiming a different identity.

## Security Considerations

| Concern | Mitigation |
|---------|------------|
| Key interception | Provision keys should be shared over a secure channel (HTTPS dashboard, encrypted message). Keys are one-time use, limiting window of attack. |
| Replay attack | One-time use flag prevents reuse of a provision key. |
| Stale keys | TTL-based expiry (default 24h). Background cleanup goroutine. |
| Rogue CSR | Server overrides the CN with the agent ID from the provision key. Agent cannot claim another identity. |
| CA key compromise | CA key only needed on the server. Restrict file permissions (`chmod 600`). Consider HSM for production. |
| Provisioning endpoint abuse | Rate limit `/api/v1/provision`. Consider IP allowlisting. Invalid key attempts should be logged. |

## Implementation Phases

- [ ] **Phase 1**: Provision key store (in-memory CRUD + expiry cleanup)
- [ ] **Phase 2**: Admin API endpoints (create, list, revoke keys)
- [ ] **Phase 3**: CSR signing logic (parse CSR, sign with CA, return cert)
- [ ] **Phase 4**: Provision endpoint (`POST /api/v1/provision`)
- [ ] **Phase 5**: Agent CLI `provision` subcommand
- [ ] **Phase 6**: Configuration updates (ca_key_file, provision section)
- [ ] **Phase 7**: Testing and documentation

## Detailed Phase Documentation

- [Phase 1: Provision Key Store](./phase1.md)
- [Phase 2: Admin API](./phase2.md)
- [Phase 3: CSR Signing](./phase3.md)
- [Phase 4: Provision Endpoint](./phase4.md)
- [Phase 5: Agent CLI](./phase5.md)
- [Phase 6: Configuration](./phase6.md)
- [Phase 7: Testing](./phase7.md)
