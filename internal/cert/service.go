package cert

import (
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
)

type Service struct {
	CaCertPath     string
	CaKeyPath      string
	ServerCertPath string
	ServerKeyPath  string
	AgentCertDir   string
	DomainNames    []string
	IPAddresses    []net.IP
}

func New(caCertPath, caKeyPath, serverCertPath, serverKeyPath, agentCertDir, domainNamesConfig, IPAddressesConfig string) (*Service, error) {
	s := &Service{
		CaCertPath:     caCertPath,
		CaKeyPath:      caKeyPath,
		ServerCertPath: serverCertPath,
		ServerKeyPath:  serverKeyPath,
		AgentCertDir:   agentCertDir,
	}

	domainNames := ParseCommaSeparated(domainNamesConfig)
	if len(domainNames) > 0 {
		s.DomainNames = domainNames
	}

	ipAddresses := ParseCommaSeparated(IPAddressesConfig)
	if len(ipAddresses) > 0 {
		for _, ipStr := range ipAddresses {
			if ip := net.ParseIP(ipStr); ip != nil {
				s.IPAddresses = append(s.IPAddresses, ip)
			} else {
				slog.Warn("Invalid IP address in configuration, skipping", "ip", ipStr)
			}
		}
	}

	if err := s.ensureCertificates(); err != nil {
		return nil, fmt.Errorf("failed to ensure certificates: %w", err)
	}

	return s, nil
}

func (s *Service) ensureCertificates() error {
	caCertExists := fileExists(s.CaCertPath)
	caKeyExists := fileExists(s.CaKeyPath)

	var caCert *x509.Certificate
	var caKey *rsa.PrivateKey

	if !caCertExists || !caKeyExists {
		slog.Info("CA certificate not found, generating new CA", "cert_path", s.CaCertPath)

		var err error
		caCert, caKey, err = s.GenerateCA()
		if err != nil {
			slog.Error("Failed to generate CA certificate", "error", err)
			return fmt.Errorf("failed to generate CA certificate: %w", err)
		}

		if err := s.ensureDirectory(s.CaCertPath); err != nil {
			return err
		}

		if err := writeCertToFile(caCert, s.CaCertPath); err != nil {
			slog.Error("Failed to write CA certificate", "error", err, "path", s.CaCertPath)
			return fmt.Errorf("failed to write CA certificate: %w", err)
		}

		if err := s.ensureDirectory(s.CaKeyPath); err != nil {
			return err
		}

		if err := writeKeyToFile(caKey, s.CaKeyPath); err != nil {
			slog.Error("Failed to write CA key", "error", err, "path", s.CaKeyPath)
			return fmt.Errorf("failed to write CA key: %w", err)
		}

		slog.Info("Generated CA certificate", "cert_path", s.CaCertPath, "key_path", s.CaKeyPath)
	} else {
		slog.Debug("Using existing CA certificate", "cert_path", s.CaCertPath)

		var err error
		caCert, caKey, err = loadCA(s.CaCertPath, s.CaKeyPath)
		if err != nil {
			slog.Error("Failed to load existing CA certificate", "error", err)
			return fmt.Errorf("failed to load existing CA certificate: %w", err)
		}
	}

	serverCertExists := fileExists(s.ServerCertPath)
	serverKeyExists := fileExists(s.ServerKeyPath)

	if !serverCertExists || !serverKeyExists {
		slog.Info("Server certificate not found, generating new server certificate",
			"cert_path", s.ServerCertPath,
			"domains", s.DomainNames,
			"ips", s.IPAddresses)

		serverCert, serverKey, err := s.GenerateServerCert(caCert, caKey, s.DomainNames, s.IPAddresses)
		if err != nil {
			slog.Error("Failed to generate server certificate", "error", err)
			return fmt.Errorf("failed to generate server certificate: %w", err)
		}

		if err := s.ensureDirectory(s.ServerCertPath); err != nil {
			return err
		}

		if err := writeCertToFile(serverCert, s.ServerCertPath); err != nil {
			slog.Error("Failed to write server certificate", "error", err, "path", s.ServerCertPath)
			return fmt.Errorf("failed to write server certificate: %w", err)
		}

		if err := s.ensureDirectory(s.ServerKeyPath); err != nil {
			return err
		}

		if err := writeKeyToFile(serverKey, s.ServerKeyPath); err != nil {
			slog.Error("Failed to write server key", "error", err, "path", s.ServerKeyPath)
			return fmt.Errorf("failed to write server key: %w", err)
		}

		slog.Info("Generated server certificate", "cert_path", s.ServerCertPath, "key_path", s.ServerKeyPath)
	} else {
		slog.Debug("Using existing server certificate", "cert_path", s.ServerCertPath)
	}

	return nil
}

func (s *Service) ensureDirectory(filePath string) error {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		slog.Error("Failed to create directory", "error", err, "path", dir)
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}
	return nil
}

func (s *Service) GetCACert() ([]byte, error) {
	certBytes, err := os.ReadFile(s.CaCertPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA certificate: %w", err)
	}
	return certBytes, nil
}

func (s *Service) GetAgentCertDir(agentID string) string {
	return filepath.Join(s.AgentCertDir, agentID)
}

func (s *Service) GetAgentCertPath(agentID string) string {
	return filepath.Join(s.GetAgentCertDir(agentID), fmt.Sprintf("%s-cert.pem", agentID))
}

func (s *Service) GetAgentKeyPath(agentID string) string {
	return filepath.Join(s.GetAgentCertDir(agentID), fmt.Sprintf("%s-key.pem", agentID))
}

func (s *Service) AgentCertExists(agentID string) bool {
	certPath := s.GetAgentCertPath(agentID)
	keyPath := s.GetAgentKeyPath(agentID)
	return fileExists(certPath) && fileExists(keyPath)
}

func (s *Service) GetAgentCert(agentID string) (certBytes, keyBytes []byte, err error) {
	certPath := s.GetAgentCertPath(agentID)
	keyPath := s.GetAgentKeyPath(agentID)

	certBytes, err = os.ReadFile(certPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read agent certificate: %w", err)
	}

	keyBytes, err = os.ReadFile(keyPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read agent key: %w", err)
	}

	return certBytes, keyBytes, nil
}

func (s *Service) DeleteAgentCert(agentID string) error {
	certDir := s.GetAgentCertDir(agentID)

	if !fileExists(certDir) {
		return fmt.Errorf("agent certificate directory does not exist")
	}

	if err := os.RemoveAll(certDir); err != nil {
		return fmt.Errorf("failed to delete agent certificate directory: %w", err)
	}

	slog.Info("Deleted agent certificate", "agent_id", agentID, "path", certDir)
	return nil
}

func (s *Service) ListAgentCerts() ([]string, error) {
	if !fileExists(s.AgentCertDir) {
		return []string{}, nil
	}

	entries, err := os.ReadDir(s.AgentCertDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read agent cert directory: %w", err)
	}

	var agentIDs []string
	for _, entry := range entries {
		if entry.IsDir() {
			agentID := entry.Name()
			if s.AgentCertExists(agentID) {
				agentIDs = append(agentIDs, agentID)
			}
		}
	}

	return agentIDs, nil
}
