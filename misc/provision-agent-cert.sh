#!/bin/bash
curl -X POST http://localhost:8080/cert/agent -H "Content-Type: application/json" -d '{"agent_id": "agent-1"}' --output agent-1-certs.zip
