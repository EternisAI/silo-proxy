# Phase 7: Testing

**Status**: Pending

## Summary

End-to-end testing of the full provisioning flow, plus integration tests that verify a provisioned agent can establish an mTLS gRPC connection.

## Files to Add

- `internal/provision/integration_test.go` - Full flow integration tests

## Test Plan

### Unit Test Summary (from earlier phases)

| Phase | Package | Tests |
|-------|---------|-------|
| 1 | `provision.KeyStore` | 10 cases (CRUD, expiry, concurrency) |
| 2 | `handler.ProvisionHandler` | 7 cases (admin API endpoints) |
| 3 | `provision.CertSigner` | 10 cases (signing, validation, error paths) |
| 4 | `handler.Provision` | 10 cases (provision endpoint) |
| 5 | `provision.Client` | 7 cases (agent client, key generation) |

### Integration Tests

#### Test 1: Full Provisioning Flow

```
1. Start server with TLS + provisioning enabled (test CA certs)
2. POST /api/v1/provision-keys {agent_id: "test-agent"}
3. Verify 201 response with key
4. Generate key + CSR on "agent side"
5. POST /api/v1/provision {provision_key, csr}
6. Verify 200 response with agent_cert + ca_cert
7. Verify agent_cert:
   - CN = "test-agent"
   - Signed by test CA
   - ExtKeyUsage = ClientAuth
   - Valid time range
8. Verify provision key is now consumed (second attempt returns 401)
```

#### Test 2: Provisioned Agent Connects via mTLS

```
1. Start gRPC server with mTLS (RequireAndVerifyClientCert)
2. Provision agent cert via API
3. Create gRPC client using provisioned certs
4. Connect to gRPC server
5. Verify bidirectional stream works
6. Verify server sees correct agent ID from cert CN
```

#### Test 3: Key Lifecycle

```
1. Create provision key
2. Verify it appears in list
3. Revoke the key
4. Verify it no longer appears in list
5. Attempt to provision with revoked key → 401
```

#### Test 4: Key Expiry

```
1. Create provision key with 1-second TTL
2. Wait 2 seconds
3. Attempt to provision → 401 (expired)
```

#### Test 5: Concurrent Provisioning

```
1. Create 10 provision keys for 10 different agents
2. Provision all 10 concurrently
3. Verify all 10 get valid, unique certificates
4. Verify all 10 keys are consumed
```

#### Test 6: Provisioning Disabled

```
1. Start server with provision.enabled = false
2. POST /api/v1/provision-keys → 404 (route not registered)
3. POST /api/v1/provision → 404 (route not registered)
```

### Manual Testing Checklist

```bash
# 1. Generate CA certs
make generate-certs

# 2. Start server with provisioning enabled
# Edit application.yaml:
#   provision:
#     enabled: true
#   grpc.tls:
#     enabled: true
#     ca_key_file: certs/ca/ca-key.pem
make run

# 3. Create a provision key
curl -X POST http://localhost:8080/api/v1/provision-keys \
  -H "Content-Type: application/json" \
  -d '{"agent_id": "agent-test"}'
# Save the returned provision_key

# 4. Provision from agent device
silo-proxy-agent provision \
  --server http://localhost:8080 \
  --key sk_<key_from_step_3> \
  --cert-dir ./test-certs

# 5. Verify cert files created
ls -la ./test-certs/
# agent-key.pem (0600)
# agent-cert.pem (0644)
# ca-cert.pem (0644)

# 6. Inspect the issued certificate
openssl x509 -in ./test-certs/agent-cert.pem -text -noout
# Verify: Subject: CN = agent-test
# Verify: Issuer: CN = Silo Proxy CA
# Verify: X509v3 Extended Key Usage: TLS Web Client Authentication

# 7. Start agent with provisioned certs
# Update agent application.yaml with cert paths
make run-agent

# 8. Verify agent connects and is functional
curl http://localhost:8100/health

# 9. Verify provision key is consumed
curl -X POST http://localhost:8080/api/v1/provision \
  -H "Content-Type: application/json" \
  -d '{"provision_key": "sk_<same_key>", "csr": "..."}'
# Should return 401: provision key already used

# 10. List keys and verify status
curl http://localhost:8080/api/v1/provision-keys
```

## Success Criteria

- All unit tests pass across phases 1-5
- Integration tests verify end-to-end provisioning
- Provisioned agents connect via mTLS successfully
- One-time use enforcement works
- Key expiry works
- Concurrent provisioning is safe
- Provisioning can be disabled via config
