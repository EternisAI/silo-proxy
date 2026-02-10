# Phase 6: Configuration

**Status**: Pending

## Summary

Add configuration fields for the CA private key (needed for signing) and provisioning settings. Update config loading and validation.

## Files to Modify

- `cmd/silo-proxy-server/config.go` - Add provision config struct
- `cmd/silo-proxy-server/application.yaml` - Add provision section
- `cmd/silo-proxy-server/main.go` - Wire up KeyStore and CertSigner

## Configuration

### application.yaml additions

```yaml
grpc:
  port: 9090
  tls:
    enabled: true
    cert_file: "certs/server/server-cert.pem"
    key_file: "certs/server/server-key.pem"
    ca_file: "certs/ca/ca-cert.pem"
    ca_key_file: "certs/ca/ca-key.pem"   # NEW
    client_auth: "require"

provision:                          # NEW section
  enabled: false                    # disabled by default
  key_ttl_hours: 24                 # default TTL for provision keys
  cert_validity_days: 365           # validity period for issued certs
  cleanup_interval_minutes: 60      # how often to clean expired keys
```

### Config Struct

```go
type ProvisionConfig struct {
    Enabled                bool `mapstructure:"enabled"`
    KeyTTLHours            int  `mapstructure:"key_ttl_hours"`
    CertValidityDays       int  `mapstructure:"cert_validity_days"`
    CleanupIntervalMinutes int  `mapstructure:"cleanup_interval_minutes"`
}

// Existing TLS config, add one field:
type TLSConfig struct {
    Enabled    bool   `mapstructure:"enabled"`
    CertFile   string `mapstructure:"cert_file"`
    KeyFile    string `mapstructure:"key_file"`
    CAFile     string `mapstructure:"ca_file"`
    CAKeyFile  string `mapstructure:"ca_key_file"`  // NEW
    ClientAuth string `mapstructure:"client_auth"`
}
```

### Environment Variable Overrides

Following existing Viper conventions:
- `PROVISION_ENABLED=true`
- `PROVISION_KEY_TTL_HOURS=48`
- `PROVISION_CERT_VALIDITY_DAYS=730`
- `GRPC_TLS_CA_KEY_FILE=/path/to/ca-key.pem`

### Validation

At startup, if `provision.enabled` is true:
- `grpc.tls.ca_file` must be set and file must exist
- `grpc.tls.ca_key_file` must be set and file must exist
- `provision.key_ttl_hours` must be > 0
- `provision.cert_validity_days` must be > 0

If validation fails, server exits with a clear error message.

### Wiring in main.go

```go
// In server startup, after config load:

if cfg.Provision.Enabled {
    // Initialize KeyStore
    ttl := time.Duration(cfg.Provision.KeyTTLHours) * time.Hour
    keyStore := provision.NewKeyStore(ttl)

    // Start cleanup goroutine
    cleanupInterval := time.Duration(cfg.Provision.CleanupIntervalMinutes) * time.Minute
    go keyStore.StartCleanup(ctx, cleanupInterval)

    // Initialize CertSigner
    certSigner, err := provision.NewCertSigner(
        cfg.GRPC.TLS.CAFile,
        cfg.GRPC.TLS.CAKeyFile,
        cfg.Provision.CertValidityDays,
    )

    // Register provision handler and routes
    provisionHandler := handler.NewProvisionHandler(keyStore, certSigner)
    // ... register routes
}
```

## Test Cases

1. Config loads provision section correctly
2. Default values applied when section is omitted
3. Environment variables override config file
4. Validation fails when provision enabled but CA key file missing
5. Validation fails when provision enabled but CA cert file missing
6. Provision routes not registered when provision disabled

## Next Steps

**Phase 7**: End-to-end testing of the complete provisioning flow.
