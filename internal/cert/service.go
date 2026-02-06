package cert

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/EternisAI/silo-proxy/internal/db/sqlc"
)

type Service struct {
	CaCertPath     string
	CaKeyPath      string
	ServerCertPath string
	ServerKeyPath  string
	AgentCertDir   string
	DomainNames    []string
	IPAddresses    []net.IP
	db             *pgxpool.Pool
	queries        *sqlc.Queries
}

func New(caCertPath, caKeyPath, serverCertPath, serverKeyPath, agentCertDir, domainNamesConfig, IPAddressesConfig string, db *pgxpool.Pool) (*Service, error) {
	s := &Service{
		CaCertPath:     caCertPath,
		CaKeyPath:      caKeyPath,
		ServerCertPath: serverCertPath,
		ServerKeyPath:  serverKeyPath,
		AgentCertDir:   agentCertDir,
		db:             db,
		queries:        sqlc.New(db),
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

func (s *Service) GenerateAgentCertWithDB(ctx context.Context, agentID string, userID pgtype.UUID) (*x509.Certificate, *rsa.PrivateKey, error) {
	if err := ValidateAgentID(agentID); err != nil {
		return nil, nil, fmt.Errorf("invalid agent ID: %w", err)
	}

	existingCert, err := s.queries.GetCertificateByAgentID(ctx, agentID)
	if err == nil {
		return nil, nil, fmt.Errorf("certificate already exists for agent %s", existingCert.AgentID)
	}

	agentCert, agentKey, err := s.GenerateAgentCert(agentID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate certificate: %w", err)
	}

	certPEM, err := CertToPEM(agentCert)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to encode certificate: %w", err)
	}

	keyPEM, err := KeyToPEM(agentKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to encode key: %w", err)
	}

	serialNumberStr := agentCert.SerialNumber.String()

	_, err = s.queries.CreateAgentCertificate(ctx, sqlc.CreateAgentCertificateParams{
		UserID:            userID,
		AgentID:           agentID,
		SerialNumber:      serialNumberStr,
		SubjectCommonName: agentCert.Subject.CommonName,
		NotBefore: pgtype.Timestamp{
			Time:  agentCert.NotBefore,
			Valid: true,
		},
		NotAfter: pgtype.Timestamp{
			Time:  agentCert.NotAfter,
			Valid: true,
		},
		CertPem: string(certPEM),
		KeyPem:  string(keyPEM),
	})
	if err != nil {
		if deleteErr := s.DeleteAgentCert(agentID); deleteErr != nil {
			slog.Error("Failed to cleanup agent certificate after DB insert failure", "error", deleteErr, "agent_id", agentID)
		}
		return nil, nil, fmt.Errorf("failed to store certificate in database: %w", err)
	}

	slog.Info("Generated and stored agent certificate", "agent_id", agentID, "user_id", userID)
	return agentCert, agentKey, nil
}

func (s *Service) GetAgentCertFromDB(ctx context.Context, agentID string) (certBytes, keyBytes []byte, err error) {
	cert, err := s.queries.GetCertificateByAgentID(ctx, agentID)
	if err != nil {
		return nil, nil, fmt.Errorf("certificate not found in database: %w", err)
	}

	return []byte(cert.CertPem), []byte(cert.KeyPem), nil
}

func (s *Service) DeleteAgentCertFromDB(ctx context.Context, agentID string, userID pgtype.UUID) error {
	cert, err := s.queries.GetCertificateByAgentID(ctx, agentID)
	if err != nil {
		return fmt.Errorf("certificate not found: %w", err)
	}

	if cert.UserID != userID {
		return fmt.Errorf("permission denied: certificate belongs to different user")
	}

	if err := s.queries.DeleteCertificate(ctx, agentID); err != nil {
		return fmt.Errorf("failed to delete certificate from database: %w", err)
	}

	if err := s.DeleteAgentCert(agentID); err != nil {
		slog.Warn("Failed to delete certificate files from filesystem", "error", err, "agent_id", agentID)
	}

	slog.Info("Deleted agent certificate", "agent_id", agentID, "user_id", userID)
	return nil
}

func (s *Service) RevokeAgentCert(ctx context.Context, agentID string, userID pgtype.UUID, reason string) error {
	cert, err := s.queries.GetCertificateByAgentID(ctx, agentID)
	if err != nil {
		return fmt.Errorf("certificate not found: %w", err)
	}

	if cert.UserID != userID {
		return fmt.Errorf("permission denied: certificate belongs to different user")
	}

	_, err = s.queries.RevokeCertificate(ctx, sqlc.RevokeCertificateParams{
		AgentID: agentID,
		RevokedReason: pgtype.Text{
			String: reason,
			Valid:  true,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to revoke certificate: %w", err)
	}

	slog.Info("Revoked agent certificate", "agent_id", agentID, "user_id", userID, "reason", reason)
	return nil
}

func (s *Service) ValidateAgentCert(ctx context.Context, agentID string) (bool, error) {
	cert, err := s.queries.CheckCertificateValid(ctx, agentID)
	if err != nil {
		return false, fmt.Errorf("certificate validation failed: %w", err)
	}

	return cert.IsActive && !cert.RevokedAt.Valid, nil
}

func (s *Service) ListUserAgentCerts(ctx context.Context, userID pgtype.UUID) ([]sqlc.AgentCertificate, error) {
	certs, err := s.queries.ListCertificatesByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list certificates: %w", err)
	}
	return certs, nil
}
