#!/bin/bash

set -e

if [ "$#" -ne 1 ]; then
    echo "Usage: $0 <agent-name>"
    echo "Example: $0 agent-4"
    exit 1
fi

AGENT_NAME=$1
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
CERTS_DIR="$PROJECT_ROOT/certs"
CA_DIR="$CERTS_DIR/ca"
AGENTS_DIR="$CERTS_DIR/agents"

echo "Generating certificate for $AGENT_NAME..."

if [ ! -f "$CA_DIR/ca-cert.pem" ] || [ ! -f "$CA_DIR/ca-key.pem" ]; then
    echo "Error: CA certificate not found!"
    echo "Please run 'make generate-certs' first to create the CA."
    exit 1
fi

mkdir -p "$AGENTS_DIR"

if [ -f "$AGENTS_DIR/$AGENT_NAME-cert.pem" ]; then
    echo "Warning: Certificate for $AGENT_NAME already exists."
    read -p "Overwrite? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Aborted."
        exit 0
    fi
fi

echo "Generating private key and CSR for $AGENT_NAME..."
openssl req -newkey rsa:4096 -nodes \
  -keyout "$AGENTS_DIR/$AGENT_NAME-key.pem" \
  -out "$AGENTS_DIR/$AGENT_NAME-req.pem" \
  -subj "/CN=$AGENT_NAME"

echo "Signing certificate with CA..."
openssl x509 -req -in "$AGENTS_DIR/$AGENT_NAME-req.pem" \
  -days 365 -CA "$CA_DIR/ca-cert.pem" -CAkey "$CA_DIR/ca-key.pem" \
  -CAcreateserial -out "$AGENTS_DIR/$AGENT_NAME-cert.pem"

rm "$AGENTS_DIR/$AGENT_NAME-req.pem"

echo "✓ Successfully generated certificate for $AGENT_NAME"
echo "  Certificate: $AGENTS_DIR/$AGENT_NAME-cert.pem"
echo "  Private key: $AGENTS_DIR/$AGENT_NAME-key.pem"
echo ""
echo "To use this certificate, update your agent's application.yml:"
echo "  tls:"
echo "    enabled: true"
echo "    cert_file: certs/agents/$AGENT_NAME-cert.pem"
echo "    key_file: certs/agents/$AGENT_NAME-key.pem"
echo "    ca_file: certs/ca/ca-cert.pem"
