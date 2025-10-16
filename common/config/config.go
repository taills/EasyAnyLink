package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// ServerConfig represents the server configuration
type ServerConfig struct {
	Listen   string         `json:"listen"`
	Database DatabaseConfig `json:"database"`
	Log      LogConfig      `json:"log"`
	TLS      TLSConfig      `json:"tls"`
	Network  NetworkConfig  `json:"network"`
	Security SecurityConfig `json:"security"`
}

// DatabaseConfig represents database connection settings
type DatabaseConfig struct {
	Type            string        `json:"type"`
	Host            string        `json:"host"`
	Port            int           `json:"port"`
	User            string        `json:"user"`
	Password        string        `json:"password"`
	Database        string        `json:"database"`
	Charset         string        `json:"charset"`
	MaxOpenConns    int           `json:"max_open_conns"`
	MaxIdleConns    int           `json:"max_idle_conns"`
	ConnMaxLifetime time.Duration `json:"conn_max_lifetime"`
}

// LogConfig represents logging configuration
type LogConfig struct {
	Level  string `json:"level"`
	File   string `json:"file"`
	Format string `json:"format"` // json or text
}

// TLSConfig represents TLS/mTLS configuration
type TLSConfig struct {
	CertFile   string `json:"cert_file"`
	KeyFile    string `json:"key_file"`
	CAFile     string `json:"ca_file"`
	MinVersion string `json:"min_version"` // TLS1.2 or TLS1.3
}

// NetworkConfig represents network-related settings
type NetworkConfig struct {
	OverlayCIDR       string `json:"overlay_cidr"`       // e.g., "10.200.0.0/16"
	GatewayIP         string `json:"gateway_ip"`         // e.g., "10.200.0.1"
	MTU               int    `json:"mtu"`                // default 1400
	KeepaliveInterval int    `json:"keepalive_interval"` // seconds
	KeepaliveTimeout  int    `json:"keepalive_timeout"`  // seconds
}

// SecurityConfig represents security-related settings
type SecurityConfig struct {
	SessionTimeout     int  `json:"session_timeout"`      // minutes
	MaxFailedAuth      int  `json:"max_failed_auth"`      // max failed auth attempts
	RequireClientCerts bool `json:"require_client_certs"` // enforce mTLS
}

// AgentConfig represents the agent configuration
type AgentConfig struct {
	Mode      string        `json:"mode"` // "client" or "gateway"
	Server    string        `json:"server"`
	UserKey   string        `json:"user_key"`
	AgentID   string        `json:"id"`
	Bandwidth int           `json:"bandwidth"` // KB/s, 0 for unlimited
	Log       LogConfig     `json:"log"`
	TLS       TLSConfig     `json:"tls"`
	Rules     []RoutingRule `json:"rules,omitempty"` // Only for client mode
}

// RoutingRule represents a routing policy
type RoutingRule struct {
	Action      string `json:"action"`      // "forward", "direct", "deny"
	Destination string `json:"destination"` // CIDR notation
	Gateway     string `json:"gateway,omitempty"`
	Priority    int    `json:"priority"`
}

// LoadServerConfig loads server configuration from file
func LoadServerConfig(path string) (*ServerConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config ServerConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set defaults
	if config.Network.MTU == 0 {
		config.Network.MTU = 1400
	}
	if config.Network.KeepaliveInterval == 0 {
		config.Network.KeepaliveInterval = 30
	}
	if config.Network.KeepaliveTimeout == 0 {
		config.Network.KeepaliveTimeout = 90
	}
	if config.Security.SessionTimeout == 0 {
		config.Security.SessionTimeout = 1440 // 24 hours
	}
	if config.Security.MaxFailedAuth == 0 {
		config.Security.MaxFailedAuth = 5
	}
	if config.Log.Format == "" {
		config.Log.Format = "json"
	}
	if config.TLS.MinVersion == "" {
		config.TLS.MinVersion = "TLS1.3"
	}

	return &config, nil
}

// LoadAgentConfig loads agent configuration from file
func LoadAgentConfig(path string) (*AgentConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config AgentConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Validate
	if config.Mode != "client" && config.Mode != "gateway" {
		return nil, fmt.Errorf("invalid mode: must be 'client' or 'gateway'")
	}

	if config.Server == "" {
		return nil, fmt.Errorf("server address is required")
	}

	if config.Mode == "client" && config.UserKey == "" {
		return nil, fmt.Errorf("user_key is required for client mode")
	}

	if config.Mode == "gateway" && config.AgentID == "" {
		return nil, fmt.Errorf("id is required for gateway mode")
	}

	// Set defaults
	if config.Log.Level == "" {
		config.Log.Level = "info"
	}
	if config.Log.Format == "" {
		config.Log.Format = "json"
	}

	return &config, nil
}

// Validate validates the server configuration
func (c *ServerConfig) Validate() error {
	if c.Listen == "" {
		return fmt.Errorf("listen address is required")
	}
	if c.Database.Host == "" {
		return fmt.Errorf("database host is required")
	}
	if c.TLS.CertFile == "" || c.TLS.KeyFile == "" {
		return fmt.Errorf("TLS certificate and key are required")
	}
	if c.Network.OverlayCIDR == "" {
		return fmt.Errorf("overlay CIDR is required")
	}
	return nil
}

// Validate validates the agent configuration
func (c *AgentConfig) Validate() error {
	if c.Mode == "" {
		return fmt.Errorf("mode is required")
	}
	if c.Server == "" {
		return fmt.Errorf("server address is required")
	}
	if c.TLS.CAFile == "" {
		return fmt.Errorf("CA certificate is required for TLS verification")
	}
	return nil
}
