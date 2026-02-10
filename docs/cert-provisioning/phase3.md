# Phase 3: CSR Signing

**Status**: Pending

## Summary

Implement the certificate signing logic: parse a PEM-encoded CSR, verify it, and sign it with the server's CA key to produce an agent certificate. The agent ID from the provision key is embedded as the certificate's Common Name.

## Files to Add

- `internal/provision/signer.go` - CSR parsing and signing
- `internal/provision/signer_test.go` - Unit tests

## Implementation

### CertSigner Struct

```go
type CertSigner struct {
    caCert       *x509.Certificate
    caKey        crypto.PrivateKey
    validityDays int
}

func NewCertSigner(caCertFile, caKeyFile string, validityDays int) (*CertSigner, error)
func (s *CertSigner) SignCSR(csrPEM []byte, agentID string) ([]byte, error)
func (s *CertSigner) CACertPEM() []byte
```

### NewCertSigner

Loads the CA certificate and private key from disk at startup:

```go
func NewCertSigner(caCertFile, caKeyFile string, validityDays int) (*CertSigner, error) {
    // 1. Read and parse CA cert PEM
    // 2. Read and parse CA private key PEM
    // 3. Verify key matches cert (optional sanity check)
    // 4. Return CertSigner
}
```

Fails fast at server startup if CA files are missing or invalid.

### SignCSR

```go
func (s *CertSigner) SignCSR(csrPEM []byte, agentID string) ([]byte, error) {
    // 1. PEM-decode the CSR
    // 2. Parse the CSR with x509.ParseCertificateRequest
    // 3. Verify CSR signature (csr.CheckSignature())
    //    - This proves the agent owns the private key
    // 4. Generate random serial number
    // 5. Build x509.Certificate template:
    //    - SerialNumber: random
    //    - Subject.CommonName: agentID (overrides CSR subject)
    //    - NotBefore: time.Now()
    //    - NotAfter: time.Now().Add(validityDays)
    //    - KeyUsage: x509.KeyUsageDigitalSignature
    //    - ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
    // 6. Sign: x509.CreateCertificate(rand.Reader, template, s.caCert, csr.PublicKey, s.caKey)
    // 7. PEM-encode and return
}
```

Key details:
- **CN override**: The agent cannot choose its own identity. The CN is always set from the provision key's agent ID.
- **Client auth only**: The `ExtKeyUsage` is `ClientAuth` — these certs are only valid for mTLS client authentication, not for serving TLS.
- **CSR signature check**: Verifies the agent actually holds the private key for the public key in the CSR.

### CACertPEM

Returns the PEM-encoded CA certificate. Used by the provision endpoint to include the CA cert in the response so the agent can verify the server.

```go
func (s *CertSigner) CACertPEM() []byte {
    return pem.EncodeToMemory(&pem.Block{
        Type:  "CERTIFICATE",
        Bytes: s.caCert.Raw,
    })
}
```

## Test Cases

1. Sign valid CSR returns valid certificate
2. Signed cert has correct CN (matches agentID, not CSR subject)
3. Signed cert has `ExtKeyUsageClientAuth`
4. Signed cert is verifiable against CA
5. Signed cert has correct validity period
6. Reject invalid PEM input
7. Reject malformed CSR
8. Reject CSR with bad signature
9. NewCertSigner fails with missing CA cert file
10. NewCertSigner fails with missing CA key file

### Test Helpers

Tests will need to generate test CA certs and CSRs programmatically:

```go
func generateTestCA() (*x509.Certificate, crypto.PrivateKey, error)
func generateTestCSR(cn string) ([]byte, crypto.PrivateKey, error)
```

## Next Steps

**Phase 4**: Build the provision endpoint that ties together KeyStore validation and CSR signing.
