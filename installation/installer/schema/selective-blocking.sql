-- Dvarpala Selective Blocking Database Schema
-- This schema supports dynamic resource blocking with API management

-- Main blocked resources table
CREATE TABLE IF NOT EXISTS blocked_resources (
    id SERIAL PRIMARY KEY,
    resource_type VARCHAR(50) NOT NULL CHECK (resource_type IN ('ip', 'domain', 'cidr', 'vpc_resource')),
    resource_value VARCHAR(255) NOT NULL,
    description TEXT,
    created_by VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    is_active BOOLEAN DEFAULT true,
    
    -- Ensure no duplicate active resources
    UNIQUE(resource_type, resource_value) WHERE is_active = true
);

-- VPC resources auto-discovery table
CREATE TABLE IF NOT EXISTS vpc_resources (
    id SERIAL PRIMARY KEY,
    resource_id VARCHAR(255) NOT NULL,
    resource_type VARCHAR(50) NOT NULL CHECK (resource_type IN ('vm', 'lb', 'database', 'storage', 'service')),
    ip_address INET,
    domain_name VARCHAR(255),
    port_range VARCHAR(50), -- e.g., "80,443,8080-8090"
    cloud_provider VARCHAR(20) NOT NULL CHECK (cloud_provider IN ('aws', 'gcp', 'azure')),
    vpc_id VARCHAR(255) NOT NULL,
    auto_discovered BOOLEAN DEFAULT true,
    last_scan TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    is_active BOOLEAN DEFAULT true,
    
    UNIQUE(resource_id, cloud_provider)
);

-- User authentication status (enhanced for selective blocking)
CREATE TABLE IF NOT EXISTS user_auth_status (
    id SERIAL PRIMARY KEY,
    username VARCHAR(100) NOT NULL,
    is_authenticated BOOLEAN DEFAULT false,
    auth_method VARCHAR(50), -- 'oauth', 'saml', 'local'
    session_token VARCHAR(255),
    authenticated_at TIMESTAMP,
    expires_at TIMESTAMP,
    client_ip INET,
    vpn_ip INET,
    
    UNIQUE(username)
);

-- Audit log for resource management
CREATE TABLE IF NOT EXISTS resource_audit_log (
    id SERIAL PRIMARY KEY,
    action_type VARCHAR(20) NOT NULL CHECK (action_type IN ('ADD', 'REMOVE', 'UPDATE', 'BULK_ADD', 'BULK_REMOVE')),
    resource_type VARCHAR(50),
    resource_value VARCHAR(255),
    performed_by VARCHAR(255) NOT NULL,
    performed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    client_ip INET,
    details JSONB
);

-- API tokens for resource management
CREATE TABLE IF NOT EXISTS api_tokens (
    id SERIAL PRIMARY KEY,
    token_hash VARCHAR(255) NOT NULL UNIQUE,
    token_name VARCHAR(100) NOT NULL,
    permissions JSONB NOT NULL, -- {"read": true, "write": true, "admin": false}
    created_by VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP,
    last_used TIMESTAMP,
    is_active BOOLEAN DEFAULT true
);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_blocked_resources_active ON blocked_resources(resource_type, is_active) WHERE is_active = true;
CREATE INDEX IF NOT EXISTS idx_blocked_resources_value ON blocked_resources(resource_value) WHERE is_active = true;
CREATE INDEX IF NOT EXISTS idx_vpc_resources_active ON vpc_resources(cloud_provider, is_active) WHERE is_active = true;
CREATE INDEX IF NOT EXISTS idx_vpc_resources_ip ON vpc_resources(ip_address) WHERE is_active = true;
CREATE INDEX IF NOT EXISTS idx_user_auth_username ON user_auth_status(username);
CREATE INDEX IF NOT EXISTS idx_user_auth_session ON user_auth_status(session_token) WHERE is_authenticated = true;

-- Default VPC resources (auto-populated during installation)
INSERT INTO blocked_resources (resource_type, resource_value, description, created_by, is_active) VALUES
('cidr', '172.30.0.0/26', 'Frigga VPC CIDR - auto-blocked', 'system', true),
('ip', '172.30.100.1', 'VPN server IP - conditional access', 'system', true)
ON CONFLICT (resource_type, resource_value) DO NOTHING;

-- Create functions for resource management
CREATE OR REPLACE FUNCTION update_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Trigger to update timestamp
CREATE TRIGGER update_blocked_resources_timestamp
    BEFORE UPDATE ON blocked_resources
    FOR EACH ROW
    EXECUTE FUNCTION update_timestamp();

-- Function to get active blocked IPs for client-connect script
CREATE OR REPLACE FUNCTION get_blocked_ips()
RETURNS TABLE(ip_address TEXT) AS $$
BEGIN
    RETURN QUERY
    SELECT DISTINCT 
        CASE 
            WHEN br.resource_type = 'ip' THEN br.resource_value
            WHEN br.resource_type = 'cidr' THEN br.resource_value
            WHEN vr.ip_address IS NOT NULL THEN host(vr.ip_address)
        END as ip_address
    FROM blocked_resources br
    LEFT JOIN vpc_resources vr ON br.resource_value = vr.resource_id
    WHERE br.is_active = true 
    AND (br.resource_type IN ('ip', 'cidr') OR vr.ip_address IS NOT NULL);
END;
$$ LANGUAGE plpgsql;

-- Function to get blocked domains for DNS configuration
CREATE OR REPLACE FUNCTION get_blocked_domains()
RETURNS TABLE(domain_name TEXT) AS $$
BEGIN
    RETURN QUERY
    SELECT DISTINCT 
        CASE 
            WHEN br.resource_type = 'domain' THEN br.resource_value
            WHEN vr.domain_name IS NOT NULL THEN vr.domain_name
        END as domain_name
    FROM blocked_resources br
    LEFT JOIN vpc_resources vr ON br.resource_value = vr.resource_id
    WHERE br.is_active = true 
    AND (br.resource_type = 'domain' OR vr.domain_name IS NOT NULL);
END;
$$ LANGUAGE plpgsql;