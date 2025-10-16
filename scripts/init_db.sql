-- EasyAnyLink Database Schema
-- MariaDB 10.5+ with utf8mb4 support

-- Create database if not exists
CREATE DATABASE IF NOT EXISTS easy_any_link
CHARACTER SET utf8mb4
COLLATE utf8mb4_unicode_ci;

USE easy_any_link;

-- Users table: User accounts and authentication
CREATE TABLE IF NOT EXISTS users (
    id VARCHAR(36) PRIMARY KEY COMMENT 'UUID format',
    username VARCHAR(255) UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE,
    password_hash VARCHAR(255) NOT NULL COMMENT 'bcrypt hash',
    api_key VARCHAR(64) UNIQUE NOT NULL COMMENT 'API authentication key',
    status ENUM('active', 'suspended', 'disabled') DEFAULT 'active' NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_api_key (api_key),
    INDEX idx_status (status),
    INDEX idx_username (username)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci 
COMMENT='User accounts and authentication';

-- Agents table: All registered agents (client and gateway)
CREATE TABLE IF NOT EXISTS agents (
    id VARCHAR(36) PRIMARY KEY COMMENT 'UUID format',
    user_id VARCHAR(36) NOT NULL,
    name VARCHAR(255) COMMENT 'Human-readable agent name',
    type ENUM('client', 'gateway') NOT NULL,
    status ENUM('online', 'offline', 'error') DEFAULT 'offline' NOT NULL,
    ip_address VARCHAR(45) COMMENT 'Assigned overlay IP (IPv4 or IPv6)',
    public_ip VARCHAR(45) COMMENT 'Public IP address of the agent',
    last_heartbeat TIMESTAMP NULL DEFAULT NULL,
    bandwidth_limit INT UNSIGNED COMMENT 'KB/s, NULL for unlimited',
    certificate_fingerprint VARCHAR(64) COMMENT 'SHA256 fingerprint of client cert',
    metadata JSON COMMENT 'Additional agent info (OS, version, etc.)',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    INDEX idx_user_id (user_id),
    INDEX idx_type (type),
    INDEX idx_status (status),
    INDEX idx_last_heartbeat (last_heartbeat)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci 
COMMENT='Registered agents (client and gateway)';

-- Routing rules table: Client routing policies
CREATE TABLE IF NOT EXISTS routing_rules (
    id INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    agent_id VARCHAR(36) NOT NULL,
    action ENUM('forward', 'direct', 'deny') NOT NULL,
    destination VARCHAR(45) NOT NULL COMMENT 'CIDR notation (e.g., 10.0.0.0/8)',
    gateway_id VARCHAR(36) COMMENT 'NULL for direct and deny actions',
    priority INT NOT NULL DEFAULT 100 COMMENT 'Lower number = higher priority',
    enabled TINYINT(1) NOT NULL DEFAULT 1 COMMENT '1=enabled, 0=disabled',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE,
    FOREIGN KEY (gateway_id) REFERENCES agents(id) ON DELETE SET NULL,
    INDEX idx_agent_id (agent_id),
    INDEX idx_priority (priority),
    INDEX idx_enabled (enabled)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci 
COMMENT='Client routing policies';

-- Sessions table: Active agent connections
CREATE TABLE IF NOT EXISTS sessions (
    id VARCHAR(36) PRIMARY KEY COMMENT 'Session UUID',
    agent_id VARCHAR(36) NOT NULL,
    connection_id VARCHAR(64) UNIQUE NOT NULL COMMENT 'gRPC stream identifier',
    connected_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_activity TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    bytes_sent BIGINT UNSIGNED NOT NULL DEFAULT 0,
    bytes_received BIGINT UNSIGNED NOT NULL DEFAULT 0,
    FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE,
    INDEX idx_agent_id (agent_id),
    INDEX idx_connection_id (connection_id),
    INDEX idx_last_activity (last_activity)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci 
COMMENT='Active agent connections';

-- Audit logs table: Security and operational audit trail
CREATE TABLE IF NOT EXISTS audit_logs (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    user_id VARCHAR(36) COMMENT 'NULL if system action',
    agent_id VARCHAR(36) COMMENT 'NULL if not agent-related',
    action VARCHAR(100) NOT NULL,
    resource_type VARCHAR(50) COMMENT 'e.g., agent, user, rule',
    resource_id VARCHAR(36) COMMENT 'ID of affected resource',
    ip_address VARCHAR(45) COMMENT 'Source IP of the action',
    status ENUM('success', 'failure') NOT NULL,
    details JSON COMMENT 'Additional context and parameters',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL,
    FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE SET NULL,
    INDEX idx_user_id (user_id),
    INDEX idx_agent_id (agent_id),
    INDEX idx_created_at (created_at),
    INDEX idx_action (action),
    INDEX idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci 
COMMENT='Security and operational audit trail';

-- Insert default admin user (password: admin123 - CHANGE IN PRODUCTION!)
-- Password hash generated with bcrypt cost 10
INSERT INTO users (id, username, email, password_hash, api_key, status) VALUES
('00000000-0000-0000-0000-000000000001', 
 'admin', 
 'admin@easyanylink.local',
 '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy',
 'dev_admin_key_change_in_production_00000000',
 'active')
ON DUPLICATE KEY UPDATE username=username;

-- Create view for active sessions with agent details
CREATE OR REPLACE VIEW active_sessions AS
SELECT 
    s.id AS session_id,
    s.connection_id,
    s.connected_at,
    s.last_activity,
    s.bytes_sent,
    s.bytes_received,
    a.id AS agent_id,
    a.name AS agent_name,
    a.type AS agent_type,
    a.ip_address AS agent_ip,
    a.user_id,
    u.username
FROM sessions s
JOIN agents a ON s.agent_id = a.id
JOIN users u ON a.user_id = u.id;

-- Create view for agent statistics
CREATE OR REPLACE VIEW agent_statistics AS
SELECT 
    a.id AS agent_id,
    a.name,
    a.type,
    a.status,
    a.ip_address,
    a.last_heartbeat,
    COUNT(DISTINCT s.id) AS active_sessions,
    COALESCE(SUM(s.bytes_sent), 0) AS total_bytes_sent,
    COALESCE(SUM(s.bytes_received), 0) AS total_bytes_received,
    a.user_id,
    u.username
FROM agents a
LEFT JOIN sessions s ON a.id = s.agent_id
JOIN users u ON a.user_id = u.id
GROUP BY a.id, a.name, a.type, a.status, a.ip_address, a.last_heartbeat, a.user_id, u.username;
