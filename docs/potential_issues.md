Potential Issues

1. Missing server certificate regeneration when domain/IP config changes
2. No rate limiting on certificate generation endpoints
3. Agent cert directory cleanup - DeleteAgentCert removes entire directory, which could be dangerous
