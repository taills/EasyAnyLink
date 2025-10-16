package crypto

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"time"

	"google.golang.org/grpc/credentials"
)

// TLSConfig holds TLS configuration for both server and client
type TLSConfig struct {
	CertFile   string
	KeyFile    string
	CAFile     string
	MinVersion uint16
}

// LoadServerTLSCredentials loads server TLS credentials with mTLS support
func LoadServerTLSCredentials(cfg TLSConfig) (credentials.TransportCredentials, error) {
	// Load server certificate and private key
	serverCert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load server certificate: %w", err)
	}

	// Load CA certificate for client verification
	caCert, err := os.ReadFile(cfg.CAFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load CA certificate: %w", err)
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to add CA certificate to pool")
	}

	// Configure TLS with mTLS
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    certPool,
		MinVersion:   cfg.MinVersion,
		CipherSuites: getSecureCipherSuites(),
	}

	return credentials.NewTLS(tlsConfig), nil
}

// LoadClientTLSCredentials loads client TLS credentials with mTLS support
func LoadClientTLSCredentials(cfg TLSConfig, serverName string) (credentials.TransportCredentials, error) {
	// Load client certificate and private key
	clientCert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load client certificate: %w", err)
	}

	// Load CA certificate for server verification
	caCert, err := os.ReadFile(cfg.CAFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load CA certificate: %w", err)
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to add CA certificate to pool")
	}

	// Configure TLS with mTLS
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      certPool,
		ServerName:   serverName,
		MinVersion:   cfg.MinVersion,
		CipherSuites: getSecureCipherSuites(),
	}

	return credentials.NewTLS(tlsConfig), nil
}

// GetCertificateFingerprint calculates SHA256 fingerprint of a certificate
func GetCertificateFingerprint(certFile string) (string, error) {
	certPEM, err := os.ReadFile(certFile)
	if err != nil {
		return "", fmt.Errorf("failed to read certificate: %w", err)
	}

	// Parse certificate
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return "", fmt.Errorf("failed to decode PEM block")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("failed to parse certificate: %w", err)
	}

	// Calculate SHA256 fingerprint
	fingerprint := sha256.Sum256(cert.Raw)
	return fmt.Sprintf("%x", fingerprint), nil
}

// ValidateCertificate checks if a certificate is valid and not expired
func ValidateCertificate(certFile string) error {
	certPEM, err := os.ReadFile(certFile)
	if err != nil {
		return fmt.Errorf("failed to read certificate: %w", err)
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		return fmt.Errorf("failed to decode PEM block")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse certificate: %w", err)
	}

	// Check expiration
	now := time.Now()
	if now.Before(cert.NotBefore) {
		return fmt.Errorf("certificate not yet valid (valid from %s)", cert.NotBefore)
	}
	if now.After(cert.NotAfter) {
		return fmt.Errorf("certificate expired on %s", cert.NotAfter)
	}

	// Warn if expiring soon (within 30 days)
	daysUntilExpiry := int(cert.NotAfter.Sub(now).Hours() / 24)
	if daysUntilExpiry <= 30 {
		return fmt.Errorf("certificate will expire in %d days", daysUntilExpiry)
	}

	return nil
}

// getSecureCipherSuites returns a list of secure cipher suites
func getSecureCipherSuites() []uint16 {
	return []uint16{
		tls.TLS_AES_128_GCM_SHA256,
		tls.TLS_AES_256_GCM_SHA384,
		tls.TLS_CHACHA20_POLY1305_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
	}
}

// GetTLSVersion converts string to tls.Version constant
func GetTLSVersion(version string) uint16 {
	switch version {
	case "TLS1.3":
		return tls.VersionTLS13
	case "TLS1.2":
		return tls.VersionTLS12
	default:
		return tls.VersionTLS13 // Default to TLS 1.3
	}
}

// DecodePEM is a helper function to extract first PEM block
func DecodePEM(pemData []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	return cert, nil
}

// VerifyCertificateChain verifies the certificate chain
func VerifyCertificateChain(certFile, caFile string) error {
	// Load certificate
	certPEM, err := os.ReadFile(certFile)
	if err != nil {
		return fmt.Errorf("failed to read certificate: %w", err)
	}

	cert, err := DecodePEM(certPEM)
	if err != nil {
		return err
	}

	// Load CA certificate
	caCertPEM, err := os.ReadFile(caFile)
	if err != nil {
		return fmt.Errorf("failed to read CA certificate: %w", err)
	}

	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caCertPEM) {
		return fmt.Errorf("failed to add CA certificate to pool")
	}

	// Verify certificate chain
	opts := x509.VerifyOptions{
		Roots:     roots,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
	}

	if _, err := cert.Verify(opts); err != nil {
		return fmt.Errorf("certificate verification failed: %w", err)
	}

	return nil
}
