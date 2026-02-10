#!/bin/bash
AGENT_ID=${1:-agent-1}
API_KEY=${ADMIN_API_KEY:-some-secret-key}
BASE_URL=${BASE_URL:-http://localhost:8080}

response=$(curl -s -w '\n%{http_code}' -X POST "${BASE_URL}/api/v1/provision-keys" \
  -H "X-API-Key: ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d "{\"agent_id\": \"${AGENT_ID}\"}")

http_code=$(echo "$response" | tail -n1)
body=$(echo "$response" | sed '$d')

if [[ "$http_code" -ge 200 && "$http_code" -lt 300 ]]; then
  echo "$body" | jq .
else
  echo "Error: HTTP ${http_code}" >&2
  echo "$body" >&2
  exit 1
fi
