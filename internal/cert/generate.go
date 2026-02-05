package cert

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"log/slog"
	"math/big"
	"net"
	"time"
)

func (s *Service) GenerateServerCert(caCert *x509.Certificate, caKey *rsa.PrivateKey, domainNames []string, ipAddresses []net.IP) (*x509.Certificate, *rsa.PrivateKey, error) {
	serverKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate server key: %w", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate serial number: %w", err)
	}

	commonName := "localhost"
	if len(domainNames) > 0 {
		commonName = domainNames[0]
	}

	serverTemplate := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Silo Proxy"},
			CommonName:   commonName,
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              domainNames,
		IPAddresses:           ipAddresses,
	}

	serverCertBytes, err := x509.CreateCertificate(rand.Reader, serverTemplate, caCert, &serverKey.PublicKey, caKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create server certificate: %w", err)
	}

	serverCert, err := x509.ParseCertificate(serverCertBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse server certificate: %w", err)
	}

	return serverCert, serverKey, nil
}

func (s *Service) GenerateCA() (*x509.Certificate, *rsa.PrivateKey, error) {
	caKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate CA key: %w", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate serial number: %w", err)
	}

	caTemplate := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Silo Proxy CA"},
			CommonName:   "Silo Proxy Root CA",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLenZero:        true,
	}

	caCertBytes, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create CA certificate: %w", err)
	}

	caCert, err := x509.ParseCertificate(caCertBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse CA certificate: %w", err)
	}

	return caCert, caKey, nil
}

func (s *Service) GenerateAgentCert(agentID string) (*x509.Certificate, *rsa.PrivateKey, error) {
	slog.Info("Generating agent certificate", "agent_id", agentID)

	caCert, caKey, err := loadCA(s.CaCertPath, s.CaKeyPath)
	if err != nil {
		slog.Error("Failed to load CA for agent cert generation", "error", err, "agent_id", agentID)
		return nil, nil, fmt.Errorf("failed to load CA: %w", err)
	}

	agentKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate agent key: %w", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate serial number: %w", err)
	}

	agentTemplate := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Silo Proxy"},
			CommonName:   agentID,
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	agentCertBytes, err := x509.CreateCertificate(rand.Reader, agentTemplate, caCert, &agentKey.PublicKey, caKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create agent certificate: %w", err)
	}

	agentCert, err := x509.ParseCertificate(agentCertBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse agent certificate: %w", err)
	}

	certPath := s.GetAgentCertPath(agentID)
	keyPath := s.GetAgentKeyPath(agentID)

	if err := s.ensureDirectory(certPath); err != nil {
		return nil, nil, fmt.Errorf("failed to create agent cert directory: %w", err)
	}

	if err := writeCertToFile(agentCert, certPath); err != nil {
		slog.Error("Failed to write agent certificate", "error", err, "path", certPath)
		return nil, nil, fmt.Errorf("failed to write agent certificate: %w", err)
	}

	if err := writeKeyToFile(agentKey, keyPath); err != nil {
		slog.Error("Failed to write agent key", "error", err, "path", keyPath)
		return nil, nil, fmt.Errorf("failed to write agent key: %w", err)
	}

	slog.Info("Generated and saved agent certificate", "agent_id", agentID, "cert_path", certPath, "key_path", keyPath)
	return agentCert, agentKey, nil
}
