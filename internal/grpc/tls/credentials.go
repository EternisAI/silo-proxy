package tls

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"google.golang.org/grpc/credentials"
)

func LoadServerCredentials(certFile, keyFile, caFile string, clientAuth tls.ClientAuthType) (credentials.TransportCredentials, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load server certificate: %w", err)
	}

	config := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   clientAuth,
	}

	if clientAuth != tls.NoClientCert {
		caPool := x509.NewCertPool()
		ca, err := os.ReadFile(caFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA certificate: %w", err)
		}
		if !caPool.AppendCertsFromPEM(ca) {
			return nil, fmt.Errorf("failed to append CA certificate")
		}
		config.ClientCAs = caPool
	}

	return credentials.NewTLS(config), nil
}

func LoadClientCredentials(certFile, keyFile, caFile, serverNameOverride string) (credentials.TransportCredentials, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load client certificate: %w", err)
	}

	caPool := x509.NewCertPool()
	ca, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA certificate: %w", err)
	}
	if !caPool.AppendCertsFromPEM(ca) {
		return nil, fmt.Errorf("failed to append CA certificate")
	}

	config := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caPool,
	}

	if serverNameOverride != "" {
		config.ServerName = serverNameOverride
	}

	return credentials.NewTLS(config), nil
}

func ParseClientAuthType(authType string) (tls.ClientAuthType, error) {
	switch authType {
	case "none":
		return tls.NoClientCert, nil
	case "request":
		return tls.RequestClientCert, nil
	case "require":
		return tls.RequireAndVerifyClientCert, nil
	default:
		return tls.NoClientCert, fmt.Errorf("invalid client auth type: %s (valid: none, request, require)", authType)
	}
}
