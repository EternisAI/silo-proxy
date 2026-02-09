package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/EternisAI/silo-proxy/internal/api/http/dto"
)

func runProvision(args []string) error {
	fs := flag.NewFlagSet("provision", flag.ExitOnError)
	server := fs.String("server", "", "Server URL (e.g., https://server:8080)")
	key := fs.String("key", "", "Provision key")
	certDir := fs.String("cert-dir", "./certs", "Directory to save certificates")
	insecure := fs.Bool("insecure", false, "Skip TLS certificate verification (for development only)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *server == "" {
		return fmt.Errorf("--server is required")
	}
	if *key == "" {
		return fmt.Errorf("--key is required")
	}

	if *insecure {
		fmt.Fprintln(os.Stderr, "WARNING: Using insecure TLS mode. This is unsafe for production.")
	}

	reqBody, err := json.Marshal(dto.ProvisionRequest{Key: *key})
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: *insecure,
			},
		},
	}

	url := *server + "/api/v1/provision"
	resp, err := client.Post(url, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("provisioning failed (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var provResp dto.ProvisionResponse
	if err := json.Unmarshal(body, &provResp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	agentCertDir := filepath.Join(*certDir, "agents", provResp.AgentID)
	caCertDir := filepath.Join(*certDir, "ca")

	if err := os.MkdirAll(agentCertDir, 0700); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", agentCertDir, err)
	}
	if err := os.MkdirAll(caCertDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", caCertDir, err)
	}

	certPath := filepath.Join(agentCertDir, provResp.AgentID+"-cert.pem")
	keyPath := filepath.Join(agentCertDir, provResp.AgentID+"-key.pem")
	caPath := filepath.Join(caCertDir, "ca-cert.pem")

	if err := os.WriteFile(certPath, []byte(provResp.CertPEM), 0644); err != nil {
		return fmt.Errorf("failed to write cert: %w", err)
	}
	if err := os.WriteFile(keyPath, []byte(provResp.KeyPEM), 0600); err != nil {
		return fmt.Errorf("failed to write key: %w", err)
	}
	if err := os.WriteFile(caPath, []byte(provResp.CACertPEM), 0644); err != nil {
		return fmt.Errorf("failed to write CA cert: %w", err)
	}

	fmt.Println("Provisioning successful!")
	fmt.Printf("  Agent ID: %s\n", provResp.AgentID)
	fmt.Printf("  Cert:     %s\n", certPath)
	fmt.Printf("  Key:      %s\n", keyPath)
	fmt.Printf("  CA Cert:  %s\n", caPath)
	fmt.Println()
	fmt.Println("Add the following to your agent application.yaml:")
	fmt.Println()
	fmt.Printf("grpc:\n")
	fmt.Printf("  agent_id: \"%s\"\n", provResp.AgentID)
	fmt.Printf("  tls:\n")
	fmt.Printf("    enabled: true\n")
	fmt.Printf("    cert_file: %s\n", certPath)
	fmt.Printf("    key_file: %s\n", keyPath)
	fmt.Printf("    ca_file: %s\n", caPath)

	return nil
}
