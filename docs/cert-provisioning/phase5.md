# Phase 5: Agent CLI

**Status**: Pending

## Summary

Add a `provision` subcommand to the agent binary. This command generates a local private key, creates a CSR, calls the server's provision API, and saves the returned certificates to disk.

## Files to Add

- `internal/provision/client.go` - HTTP client for the provision API
- `internal/provision/client_test.go` - Unit tests

## Files to Modify

- `cmd/silo-proxy-agent/main.go` - Add `provision` subcommand handling

## Implementation

### Provision Client

```go
type ProvisionClient struct {
    serverURL  string
    httpClient *http.Client
}

func NewProvisionClient(serverURL string) *ProvisionClient
func (c *ProvisionClient) Provision(provisionKey string, csrPEM []byte) (*ProvisionResponse, error)

type ProvisionResponse struct {
    AgentID   string `json:"agent_id"`
    AgentCert string `json:"agent_cert"`
    CACert    string `json:"ca_cert"`
}
```

### Key and CSR Generation

```go
func GenerateKeyAndCSR() (keyPEM []byte, csrPEM []byte, err error) {
    // 1. Generate RSA 4096-bit private key
    //    key, err := rsa.GenerateKey(rand.Reader, 4096)
    //
    // 2. Create CSR template
    //    template := &x509.CertificateRequest{
    //        Subject: pkix.Name{CommonName: "silo-proxy-agent"},
    //    }
    //
    // 3. Create CSR
    //    csrDER, err := x509.CreateCertificateRequest(rand.Reader, template, key)
    //
    // 4. PEM-encode both key and CSR
    //    Return keyPEM, csrPEM
}
```

The CN in the CSR doesn't matter — the server overrides it with the agent ID from the provision key.

### CLI Subcommand

```bash
silo-proxy-agent provision \
  --server https://server:8080 \
  --key sk_a1b2c3d4e5f6... \
  --cert-dir ~/.silo-proxy/certs
```

Flags:
- `--server` (required): Server base URL
- `--key` (required): Provision key
- `--cert-dir` (optional, default: `./certs`): Directory to save certificates

### Provision Flow

```go
func runProvision(serverURL, provisionKey, certDir string) error {
    // 1. Generate private key and CSR
    keyPEM, csrPEM, err := GenerateKeyAndCSR()

    // 2. Call provision API
    client := NewProvisionClient(serverURL)
    resp, err := client.Provision(provisionKey, csrPEM)

    // 3. Create cert directory
    os.MkdirAll(certDir, 0700)

    // 4. Write files with restrictive permissions
    os.WriteFile(certDir+"/agent-key.pem", keyPEM, 0600)
    os.WriteFile(certDir+"/agent-cert.pem", []byte(resp.AgentCert), 0644)
    os.WriteFile(certDir+"/ca-cert.pem", []byte(resp.CACert), 0644)

    // 5. Print summary
    fmt.Printf("Provisioned as: %s\n", resp.AgentID)
    fmt.Printf("Certificates saved to: %s\n", certDir)
    fmt.Printf("\nAdd to application.yml:\n")
    fmt.Printf("  grpc:\n")
    fmt.Printf("    tls:\n")
    fmt.Printf("      enabled: true\n")
    fmt.Printf("      cert_file: %s/agent-cert.pem\n", certDir)
    fmt.Printf("      key_file: %s/agent-key.pem\n", certDir)
    fmt.Printf("      ca_file: %s/ca-cert.pem\n", certDir)
}
```

### File Permissions

| File | Permissions | Reason |
|------|------------|--------|
| `agent-key.pem` | `0600` | Private key — owner read/write only |
| `agent-cert.pem` | `0644` | Public certificate — world readable |
| `ca-cert.pem` | `0644` | Public CA cert — world readable |
| cert directory | `0700` | Owner only |

### Error Handling

- Server unreachable: `"failed to connect to server: ..."`
- Invalid provision key: print the server's error message (401 response body)
- File write failure: `"failed to save certificates: ..."`
- Cert directory already has files: warn but overwrite (agent may be re-provisioning)

## Test Cases

1. GenerateKeyAndCSR produces valid PEM key and CSR
2. CSR signature is valid (verifiable)
3. ProvisionClient sends correct request format
4. ProvisionClient handles 200 response correctly
5. ProvisionClient handles 401 response with error message
6. ProvisionClient handles network errors
7. Files written with correct permissions

## Next Steps

**Phase 6**: Add configuration fields for CA key path and provision settings.
