#!/bin/bash
AGENT_ID=${1:-agent-1}
API_KEY=${ADMIN_API_KEY:-your-api-key-here}
curl -X GET "http://localhost:8080/agents/${AGENT_ID}/certificate" \
  -H "X-API-Key: ${API_KEY}" \
  -o "${AGENT_ID}-certs.zip"
