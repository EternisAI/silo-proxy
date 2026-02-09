# Phase 4: Provision Endpoint

**Status**: Pending

## Summary

Add the `POST /api/v1/provision` endpoint that agents call to exchange a provision key + CSR for a signed certificate. This ties together the KeyStore (phase 1) and CertSigner (phase 3).

## Files to Modify

- `internal/api/http/handler/provision.go` - Add Provision handler method
- `internal/api/http/handler/provision_test.go` - Add tests
- `internal/api/http/router.go` - Register route

### ProvisionHandler Update

Add the `CertSigner` dependency:

```go
type ProvisionHandler struct {
    keyStore   *provision.KeyStore
    certSigner *provision.CertSigner
}

func NewProvisionHandler(keyStore *provision.KeyStore, certSigner *provision.CertSigner) *ProvisionHandler
```

## Implementation

### Provision Handler

```go
func (h *ProvisionHandler) Provision(c *gin.Context)
```

Request:
```json
{
  "provision_key": "sk_a1b2c3d4e5f6...",
  "csr": "-----BEGIN CERTIFICATE REQUEST-----\nMIIE..."
}
```

Flow:
```
1. Parse and validate request body
   - provision_key: required, non-empty
   - csr: required, non-empty

2. Validate provision key
   pk, err := h.keyStore.Validate(req.ProvisionKey)
   - Returns 401 for invalid/expired/used key

3. Sign the CSR
   certPEM, err := h.certSigner.SignCSR([]byte(req.CSR), pk.AgentID)
   - Returns 400 for invalid CSR

4. Mark key as used
   h.keyStore.MarkUsed(req.ProvisionKey)

5. Return signed cert + CA cert
```

Response (200):
```json
{
  "agent_id": "agent-5",
  "agent_cert": "-----BEGIN CERTIFICATE-----\nMIIF...",
  "ca_cert": "-----BEGIN CERTIFICATE-----\nMIID..."
}
```

Error responses:
- 400: `{"error": "provision_key is required"}` / `{"error": "csr is required"}` / `{"error": "invalid CSR: ..."}`
- 401: `{"error": "invalid provision key"}` / `{"error": "provision key expired"}` / `{"error": "provision key already used"}`

### Route Registration

```go
v1.POST("/provision", provisionHandler.Provision)
```

### Important: Key Consumption Ordering

The key is marked as used **after** signing succeeds. If signing fails (bad CSR), the key remains valid so the agent can retry with a corrected CSR. This is intentional — the key is only consumed on successful provisioning.

### Logging

- INFO: `"agent provisioned" agent_id=agent-5`
- WARN: `"provision attempt with invalid key" key_prefix=sk_a1b2...`
- ERROR: `"CSR signing failed" agent_id=agent-5 error=...`

## Test Cases

1. Valid provision key + valid CSR returns 200 + certs
2. Invalid provision key returns 401
3. Expired provision key returns 401
4. Already-used provision key returns 401
5. Valid key + malformed CSR returns 400
6. Missing provision_key field returns 400
7. Missing csr field returns 400
8. Key remains valid if CSR signing fails
9. Key is consumed after successful provisioning
10. Response includes correct agent_id, agent_cert, and ca_cert

## Next Steps

**Phase 5**: Implement the agent-side `provision` CLI subcommand.
