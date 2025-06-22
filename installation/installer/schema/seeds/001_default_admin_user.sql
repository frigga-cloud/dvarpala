-- Seed: Create default admin user and groups
-- Description: Insert default admin user and basic groups
-- Created: 2024-06-21

-- Insert default groups
INSERT INTO groups (name, description, permissions, is_active) VALUES
    ('administrators', 'Full system administrators', '{"admin.*", "vpn.*", "users.*", "groups.*"}', true),
    ('vpn_users', 'Standard VPN users', '{"vpn.connect", "vpn.disconnect"}', true),
    ('guest_users', 'Limited access users', '{"vpn.connect"}', true)
ON CONFLICT (name) DO NOTHING;

-- Note: Admin user will be inserted by the installer with actual email/name from config
-- This is handled in the installer to use dynamic values from configuration

-- Insert a sample configuration record for system settings
CREATE TABLE IF NOT EXISTS system_settings (
    id SERIAL PRIMARY KEY,
    setting_key VARCHAR(255) UNIQUE NOT NULL,
    setting_value TEXT,
    description TEXT,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Add triggers to system_settings
CREATE TRIGGER update_system_settings_updated_at 
    BEFORE UPDATE ON system_settings 
    FOR EACH ROW 
    EXECUTE FUNCTION update_updated_at_column();

-- Insert default system settings
INSERT INTO system_settings (setting_key, setting_value, description) VALUES
    ('max_concurrent_sessions', '10', 'Maximum concurrent VPN sessions per user'),
    ('session_timeout_minutes', '1440', 'Session timeout in minutes (24 hours)'),
    ('require_web_auth', 'true', 'Require web authentication before VPN access'),
    ('vpn_network_cidr', '172.30.100.0/24', 'VPN client IP range'),
    ('captive_portal_enabled', 'true', 'Enable captive portal authentication')
ON CONFLICT (setting_key) DO NOTHING;