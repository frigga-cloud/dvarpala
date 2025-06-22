-- Migration: Create sessions table
-- Description: User sessions table for authentication and VPN access tracking
-- Created: 2024-06-21

CREATE TABLE IF NOT EXISTS sessions (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    session_token VARCHAR(255) UNIQUE NOT NULL,
    vpn_client_ip INET, -- IP assigned by OpenVPN
    vpn_connected_at TIMESTAMP,
    vpn_disconnected_at TIMESTAMP,
    web_session_data JSONB, -- Store additional session metadata
    user_agent TEXT,
    client_ip INET, -- Original client IP before VPN
    is_active BOOLEAN DEFAULT true,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for better performance
CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_token ON sessions(session_token);
CREATE INDEX IF NOT EXISTS idx_sessions_vpn_client_ip ON sessions(vpn_client_ip);
CREATE INDEX IF NOT EXISTS idx_sessions_is_active ON sessions(is_active);
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);

-- Add triggers to update updated_at timestamp
CREATE TRIGGER update_sessions_updated_at 
    BEFORE UPDATE ON sessions 
    FOR EACH ROW 
    EXECUTE FUNCTION update_updated_at_column();

-- Create VPN access logs table for audit trail
CREATE TABLE IF NOT EXISTS vpn_access_logs (
    id SERIAL PRIMARY KEY,
    session_id INTEGER NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    event_type VARCHAR(50) NOT NULL, -- 'connect', 'disconnect', 'auth_success', 'auth_fail'
    vpn_client_ip INET,
    client_real_ip INET,
    user_agent TEXT,
    event_details JSONB, -- Additional event metadata
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for the access logs
CREATE INDEX IF NOT EXISTS idx_vpn_logs_session_id ON vpn_access_logs(session_id);
CREATE INDEX IF NOT EXISTS idx_vpn_logs_user_id ON vpn_access_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_vpn_logs_event_type ON vpn_access_logs(event_type);
CREATE INDEX IF NOT EXISTS idx_vpn_logs_created_at ON vpn_access_logs(created_at);