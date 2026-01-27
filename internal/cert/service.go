package cert

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log/slog"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

type Service struct {
	CaCertPath     string
	CaKeyPath      string
	ServerCertPath string
	ServerKeyPath  string
	DomainNames    []string
	IPAddresses    []net.IP
}

type Options struct {
	DomainNames []string
	IPAddresses []net.IP
}

func New(caCertPath, caKeyPath, serverCertPath, serverKeyPath string, opts *Options) (*Service, error) {
	s := &Service{
		CaCertPath:     caCertPath,
		CaKeyPath:      caKeyPath,
		ServerCertPath: serverCertPath,
		ServerKeyPath:  serverKeyPath,
	}

	if opts != nil {
		s.DomainNames = opts.DomainNames
		s.IPAddresses = opts.IPAddresses
	}

	if len(s.DomainNames) == 0 {
		s.DomainNames = []string{"localhost"}
	}

	if len(s.IPAddresses) == 0 {
		s.IPAddresses = []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")}
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
		caCert, caKey, err = generateCA()
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

		serverCert, serverKey, err := generateServerCert(caCert, caKey, s.DomainNames, s.IPAddresses)
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

func generateCA() (*x509.Certificate, *rsa.PrivateKey, error) {
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

func generateServerCert(caCert *x509.Certificate, caKey *rsa.PrivateKey, domainNames []string, ipAddresses []net.IP) (*x509.Certificate, *rsa.PrivateKey, error) {
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

func loadCA(certPath, keyPath string) (*x509.Certificate, *rsa.PrivateKey, error) {
	certBytes, err := os.ReadFile(certPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read CA certificate: %w", err)
	}

	certBlock, _ := pem.Decode(certBytes)
	if certBlock == nil {
		return nil, nil, fmt.Errorf("failed to decode CA certificate PEM")
	}

	caCert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse CA certificate: %w", err)
	}

	keyBytes, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read CA key: %w", err)
	}

	keyBlock, _ := pem.Decode(keyBytes)
	if keyBlock == nil {
		return nil, nil, fmt.Errorf("failed to decode CA key PEM")
	}

	caKey, err := x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse CA key: %w", err)
	}

	return caCert, caKey, nil
}

func writeCertToFile(cert *x509.Certificate, path string) error {
	certFile, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create certificate file: %w", err)
	}
	defer certFile.Close()

	if err := pem.Encode(certFile, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Raw,
	}); err != nil {
		return fmt.Errorf("failed to encode certificate: %w", err)
	}

	return nil
}

func writeKeyToFile(key *rsa.PrivateKey, path string) error {
	keyFile, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create key file: %w", err)
	}
	defer keyFile.Close()

	if err := pem.Encode(keyFile, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}); err != nil {
		return fmt.Errorf("failed to encode key: %w", err)
	}

	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
