package server

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/taills/EasyAnyLink/common/config"
)

// Database represents the database connection
type Database struct {
	db *sql.DB
}

// NewDatabase creates a new database connection
func NewDatabase(cfg config.DatabaseConfig) (*Database, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=true&loc=Local",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Database,
		cfg.Charset,
	)

	db, err := sql.Open(cfg.Type, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Set connection pool parameters
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &Database{db: db}, nil
}

// Close closes the database connection
func (d *Database) Close() error {
	return d.db.Close()
}

// User represents a user record
type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	APIKey       string    `json:"api_key"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Agent represents an agent record
type Agent struct {
	ID                     string    `json:"id"`
	UserID                 string    `json:"user_id"`
	Name                   string    `json:"name"`
	Type                   string    `json:"type"`
	Status                 string    `json:"status"`
	IPAddress              string    `json:"ip_address"`
	PublicIP               string    `json:"public_ip"`
	LastHeartbeat          time.Time `json:"last_heartbeat"`
	BandwidthLimit         int       `json:"bandwidth_limit"`
	CertificateFingerprint string    `json:"certificate_fingerprint"`
	Metadata               string    `json:"metadata"` // JSON string
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}

// Session represents an active session
type Session struct {
	ID            string    `json:"id"`
	AgentID       string    `json:"agent_id"`
	ConnectionID  string    `json:"connection_id"`
	ConnectedAt   time.Time `json:"connected_at"`
	LastActivity  time.Time `json:"last_activity"`
	BytesSent     uint64    `json:"bytes_sent"`
	BytesReceived uint64    `json:"bytes_received"`
}

// RoutingRule represents a routing rule
type RoutingRule struct {
	ID          int       `json:"id"`
	AgentID     string    `json:"agent_id"`
	Action      string    `json:"action"`
	Destination string    `json:"destination"`
	GatewayID   string    `json:"gateway_id"`
	Priority    int       `json:"priority"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// GetUserByAPIKey retrieves a user by API key
func (d *Database) GetUserByAPIKey(apiKey string) (*User, error) {
	user := &User{}
	err := d.db.QueryRow(`
		SELECT id, username, email, password_hash, api_key, status, created_at, updated_at
		FROM users WHERE api_key = ? AND status = 'active'
	`, apiKey).Scan(
		&user.ID, &user.Username, &user.Email, &user.PasswordHash,
		&user.APIKey, &user.Status, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return user, nil
}

// GetAgentByID retrieves an agent by ID
func (d *Database) GetAgentByID(agentID string) (*Agent, error) {
	agent := &Agent{}
	var lastHeartbeat sql.NullTime
	var bandwidthLimit sql.NullInt64

	err := d.db.QueryRow(`
		SELECT id, user_id, name, type, status, ip_address, public_ip, 
		       last_heartbeat, bandwidth_limit, certificate_fingerprint, 
		       metadata, created_at, updated_at
		FROM agents WHERE id = ?
	`, agentID).Scan(
		&agent.ID, &agent.UserID, &agent.Name, &agent.Type, &agent.Status,
		&agent.IPAddress, &agent.PublicIP, &lastHeartbeat, &bandwidthLimit,
		&agent.CertificateFingerprint, &agent.Metadata, &agent.CreatedAt, &agent.UpdatedAt,
	)

	if lastHeartbeat.Valid {
		agent.LastHeartbeat = lastHeartbeat.Time
	}
	if bandwidthLimit.Valid {
		agent.BandwidthLimit = int(bandwidthLimit.Int64)
	}

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("agent not found")
		}
		return nil, fmt.Errorf("failed to get agent: %w", err)
	}
	return agent, nil
}

// CreateAgent creates a new agent
func (d *Database) CreateAgent(agent *Agent) error {
	_, err := d.db.Exec(`
		INSERT INTO agents (id, user_id, name, type, status, ip_address, 
		                   certificate_fingerprint, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, agent.ID, agent.UserID, agent.Name, agent.Type, agent.Status,
		agent.IPAddress, agent.CertificateFingerprint, agent.Metadata)

	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}
	return nil
}

// UpdateAgentStatus updates agent status and heartbeat
func (d *Database) UpdateAgentStatus(agentID, status string) error {
	_, err := d.db.Exec(`
		UPDATE agents 
		SET status = ?, last_heartbeat = NOW()
		WHERE id = ?
	`, status, agentID)

	if err != nil {
		return fmt.Errorf("failed to update agent status: %w", err)
	}
	return nil
}

// CreateSession creates a new session
func (d *Database) CreateSession(session *Session) error {
	_, err := d.db.Exec(`
		INSERT INTO sessions (id, agent_id, connection_id)
		VALUES (?, ?, ?)
	`, session.ID, session.AgentID, session.ConnectionID)

	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	return nil
}

// DeleteSession deletes a session
func (d *Database) DeleteSession(sessionID string) error {
	_, err := d.db.Exec(`DELETE FROM sessions WHERE id = ?`, sessionID)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}
	return nil
}

// GetRoutingRulesByAgentID retrieves routing rules for an agent
func (d *Database) GetRoutingRulesByAgentID(agentID string) ([]*RoutingRule, error) {
	rows, err := d.db.Query(`
		SELECT id, agent_id, action, destination, gateway_id, priority, enabled, created_at, updated_at
		FROM routing_rules
		WHERE agent_id = ? AND enabled = 1
		ORDER BY priority ASC
	`, agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get routing rules: %w", err)
	}
	defer rows.Close()

	var rules []*RoutingRule
	for rows.Next() {
		rule := &RoutingRule{}
		var gatewayID sql.NullString

		err := rows.Scan(
			&rule.ID, &rule.AgentID, &rule.Action, &rule.Destination,
			&gatewayID, &rule.Priority, &rule.Enabled, &rule.CreatedAt, &rule.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan routing rule: %w", err)
		}

		if gatewayID.Valid {
			rule.GatewayID = gatewayID.String
		}

		rules = append(rules, rule)
	}

	return rules, nil
}

// GetOnlineAgents retrieves all online agents
func (d *Database) GetOnlineAgents() ([]*Agent, error) {
	rows, err := d.db.Query(`
		SELECT id, user_id, name, type, status, ip_address, public_ip,
		       last_heartbeat, bandwidth_limit, certificate_fingerprint,
		       metadata, created_at, updated_at
		FROM agents
		WHERE status = 'online'
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to get online agents: %w", err)
	}
	defer rows.Close()

	var agents []*Agent
	for rows.Next() {
		agent := &Agent{}
		var lastHeartbeat sql.NullTime
		var bandwidthLimit sql.NullInt64

		err := rows.Scan(
			&agent.ID, &agent.UserID, &agent.Name, &agent.Type, &agent.Status,
			&agent.IPAddress, &agent.PublicIP, &lastHeartbeat, &bandwidthLimit,
			&agent.CertificateFingerprint, &agent.Metadata, &agent.CreatedAt, &agent.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan agent: %w", err)
		}

		if lastHeartbeat.Valid {
			agent.LastHeartbeat = lastHeartbeat.Time
		}
		if bandwidthLimit.Valid {
			agent.BandwidthLimit = int(bandwidthLimit.Int64)
		}

		agents = append(agents, agent)
	}

	return agents, nil
}

// UpdateSessionStats updates session statistics
func (d *Database) UpdateSessionStats(sessionID string, bytesSent, bytesReceived uint64) error {
	_, err := d.db.Exec(`
		UPDATE sessions 
		SET bytes_sent = ?, bytes_received = ?, last_activity = NOW()
		WHERE id = ?
	`, bytesSent, bytesReceived, sessionID)

	if err != nil {
		return fmt.Errorf("failed to update session stats: %w", err)
	}
	return nil
}
