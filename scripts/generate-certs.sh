#!/bin/bash

set -e

CERTS_DIR="certs"
CA_DIR="$CERTS_DIR/ca"
SERVER_DIR="$CERTS_DIR/server"
AGENTS_DIR="$CERTS_DIR/agents"

echo "Generating certificates for development..."

mkdir -p "$CA_DIR" "$SERVER_DIR" "$AGENTS_DIR"

echo "1. Generating CA certificate..."
openssl req -x509 -newkey rsa:4096 -days 365 -nodes \
  -keyout "$CA_DIR/ca-key.pem" \
  -out "$CA_DIR/ca-cert.pem" \
  -subj "/CN=Silo Proxy CA"

echo "2. Generating server certificate..."
openssl req -newkey rsa:4096 -nodes \
  -keyout "$SERVER_DIR/server-key.pem" \
  -out "$SERVER_DIR/server-req.pem" \
  -subj "/CN=server"

cat > "$SERVER_DIR/server-ext.cnf" << EOF
subjectAltName = DNS:localhost,IP:127.0.0.1
EOF

openssl x509 -req -in "$SERVER_DIR/server-req.pem" \
  -days 365 -CA "$CA_DIR/ca-cert.pem" -CAkey "$CA_DIR/ca-key.pem" \
  -CAcreateserial -out "$SERVER_DIR/server-cert.pem" \
  -extfile "$SERVER_DIR/server-ext.cnf"

rm "$SERVER_DIR/server-req.pem" "$SERVER_DIR/server-ext.cnf"

echo "3. Generating agent certificates..."
for i in {1..3}; do
  AGENT_NAME="agent-$i"
  echo "   Generating certificate for $AGENT_NAME..."

  openssl req -newkey rsa:4096 -nodes \
    -keyout "$AGENTS_DIR/$AGENT_NAME-key.pem" \
    -out "$AGENTS_DIR/$AGENT_NAME-req.pem" \
    -subj "/CN=$AGENT_NAME"

  openssl x509 -req -in "$AGENTS_DIR/$AGENT_NAME-req.pem" \
    -days 365 -CA "$CA_DIR/ca-cert.pem" -CAkey "$CA_DIR/ca-key.pem" \
    -CAcreateserial -out "$AGENTS_DIR/$AGENT_NAME-cert.pem"

  rm "$AGENTS_DIR/$AGENT_NAME-req.pem"
done

echo ""
echo "Certificates generated successfully!"
echo ""
echo "Directory structure:"
echo "  $CA_DIR/ca-cert.pem        - CA certificate"
echo "  $SERVER_DIR/server-cert.pem - Server certificate"
echo "  $SERVER_DIR/server-key.pem  - Server private key"
echo "  $AGENTS_DIR/agent-N-cert.pem - Agent certificates"
echo "  $AGENTS_DIR/agent-N-key.pem  - Agent private keys"
echo ""
echo "To use these certificates, update application.yml files with:"
echo "  tls.enabled: true"
echo "  tls.cert_file: ./certs/server/server-cert.pem (server)"
echo "  tls.cert_file: ./certs/agents/agent-1-cert.pem (agent)"
