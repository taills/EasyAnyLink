package crypto

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"time"

	"google.golang.org/grpc/credentials"
)

// LoadServerTLSConfig loads server TLS configuration for QUIC (one-way TLS)
func LoadServerTLSConfig(certFile, keyFile string) (*tls.Config, error) {
	// Load server certificate and private key
	serverCert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load server certificate: %w", err)
	}

	// Configure TLS for one-way authentication (server provides cert, client verifies)
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.NoClientCert, // No client certificate required
		MinVersion:   tls.VersionTLS13, // Enforce TLS 1.3+
		CipherSuites: getSecureCipherSuites(),
		NextProtos:   []string{"h3"}, // HTTP/3 for QUIC
	}

	return tlsConfig, nil
}

// LoadClientTLSConfig loads client TLS configuration for QUIC (one-way TLS)
// Uses system root CAs to verify server certificate (e.g., Let's Encrypt)
// If insecureSkipVerify is true, skips certificate verification (for debugging only)
func LoadClientTLSConfig(serverName string, insecureSkipVerify bool) (*tls.Config, error) {
	// Configure TLS for one-way authentication (client verifies server)
	tlsConfig := &tls.Config{
		ServerName:         serverName,
		MinVersion:         tls.VersionTLS13, // Enforce TLS 1.3+
		CipherSuites:       getSecureCipherSuites(),
		NextProtos:         []string{"h3"}, // HTTP/3 for QUIC
		InsecureSkipVerify: insecureSkipVerify,
	}

	if !insecureSkipVerify {
		// Use system root CA pool for verifying server certificates
		rootCAs, err := x509.SystemCertPool()
		if err != nil {
			return nil, fmt.Errorf("failed to load system cert pool: %w", err)
		}
		tlsConfig.RootCAs = rootCAs
	}

	return tlsConfig, nil
}

// NewQUICServerCredentials creates gRPC credentials using QUIC transport
func NewQUICServerCredentials(certFile, keyFile string) (credentials.TransportCredentials, error) {
	tlsConfig, err := LoadServerTLSConfig(certFile, keyFile)
	if err != nil {
		return nil, err
	}

	return &quicServerCreds{tlsConfig: tlsConfig}, nil
}

// NewQUICClientCredentials creates gRPC credentials using QUIC transport
func NewQUICClientCredentials(serverName string, insecureSkipVerify bool) (credentials.TransportCredentials, error) {
	tlsConfig, err := LoadClientTLSConfig(serverName, insecureSkipVerify)
	if err != nil {
		return nil, err
	}

	return &quicClientCreds{
		tlsConfig:  tlsConfig,
		serverName: serverName,
	}, nil
}

// quicServerCreds implements credentials.TransportCredentials for QUIC server
type quicServerCreds struct {
	tlsConfig *tls.Config
}

func (c *quicServerCreds) ClientHandshake(ctx context.Context, authority string, rawConn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	return nil, nil, fmt.Errorf("ClientHandshake not supported on server credentials")
}

func (c *quicServerCreds) ServerHandshake(rawConn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	return rawConn, nil, nil
}

func (c *quicServerCreds) Info() credentials.ProtocolInfo {
	return credentials.ProtocolInfo{
		SecurityProtocol: "quic",
		SecurityVersion:  "1.3",
		ServerName:       "",
	}
}

func (c *quicServerCreds) Clone() credentials.TransportCredentials {
	return &quicServerCreds{tlsConfig: c.tlsConfig.Clone()}
}

func (c *quicServerCreds) OverrideServerName(serverNameOverride string) error {
	c.tlsConfig.ServerName = serverNameOverride
	return nil
}

// quicClientCreds implements credentials.TransportCredentials for QUIC client
type quicClientCreds struct {
	tlsConfig  *tls.Config
	serverName string
}

func (c *quicClientCreds) ClientHandshake(ctx context.Context, authority string, rawConn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	return rawConn, nil, nil
}

func (c *quicClientCreds) ServerHandshake(rawConn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	return nil, nil, fmt.Errorf("ServerHandshake not supported on client credentials")
}

func (c *quicClientCreds) Info() credentials.ProtocolInfo {
	return credentials.ProtocolInfo{
		SecurityProtocol: "quic",
		SecurityVersion:  "1.3",
		ServerName:       c.serverName,
	}
}

func (c *quicClientCreds) Clone() credentials.TransportCredentials {
	return &quicClientCreds{
		tlsConfig:  c.tlsConfig.Clone(),
		serverName: c.serverName,
	}
}

func (c *quicClientCreds) OverrideServerName(serverNameOverride string) error {
	c.tlsConfig.ServerName = serverNameOverride
	c.serverName = serverNameOverride
	return nil
}

// GetCertificateFingerprint calculates SHA256 fingerprint of a certificate
// Note: This is now optional since we're using one-way TLS
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

// VerifyCertificateChain is kept for backward compatibility but not used in one-way TLS
// In one-way TLS, client uses system root CAs to verify server certificate
func VerifyCertificateChain(certFile, caFile string) error {
	return nil // No-op for one-way TLS
}

// getSecureCipherSuites returns a list of secure cipher suites
func getSecureCipherSuites() []uint16 {
	return []uint16{
		tls.TLS_AES_128_GCM_SHA256,
		tls.TLS_AES_256_GCM_SHA384,
		tls.TLS_CHACHA20_POLY1305_SHA256,
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
