#!/bin/bash

# Integration script to add selective blocking functionality to cloud-setup-server.sh
# This replaces the captive portal approach with selective VPN blocking

# Add to the main installation script after PostgreSQL setup
integrate_selective_blocking() {
    log "Setting up selective VPN blocking system..."
    
    # Install Go dependencies for API server
    export PATH=/usr/local/go/bin:$PATH
    cd /opt/dvarpala
    
    # Create Go module for API server
    cat > go.mod << 'EOF'
module dvarpala-api

go 1.21

require (
    github.com/gorilla/mux v1.8.0
    github.com/lib/pq v1.10.9
)
EOF

    go mod download
    
    # Install database schema
    setup_selective_database
    
    # Configure selective OpenVPN
    setup_selective_openvpn_integration
    
    # Setup API server
    setup_api_server
    
    # Configure discovery automation
    setup_discovery_automation
    
    log "Selective VPN blocking system setup completed"
}

# Database setup for selective blocking
setup_selective_database() {
    log "Setting up selective blocking database schema..."
    
    # Copy schema file
    mkdir -p /opt/dvarpala/sql
    cat > /opt/dvarpala/sql/selective-blocking.sql << 'SCHEMA_EOF'
-- Dvarpala Selective Blocking Database Schema
CREATE TABLE IF NOT EXISTS blocked_resources (
    id SERIAL PRIMARY KEY,
    resource_type VARCHAR(50) NOT NULL CHECK (resource_type IN ('ip', 'domain', 'cidr', 'vpc_resource')),
    resource_value VARCHAR(255) NOT NULL,
    description TEXT,
    created_by VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    is_active BOOLEAN DEFAULT true,
    UNIQUE(resource_type, resource_value) WHERE is_active = true
);

CREATE TABLE IF NOT EXISTS vpc_resources (
    id SERIAL PRIMARY KEY,
    resource_id VARCHAR(255) NOT NULL,
    resource_type VARCHAR(50) NOT NULL CHECK (resource_type IN ('vm', 'lb', 'database', 'storage', 'service')),
    ip_address INET,
    domain_name VARCHAR(255),
    port_range VARCHAR(50),
    cloud_provider VARCHAR(20) NOT NULL CHECK (cloud_provider IN ('aws', 'gcp', 'azure')),
    vpc_id VARCHAR(255) NOT NULL,
    auto_discovered BOOLEAN DEFAULT true,
    last_scan TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    is_active BOOLEAN DEFAULT true,
    UNIQUE(resource_id, cloud_provider)
);

CREATE TABLE IF NOT EXISTS user_auth_status (
    id SERIAL PRIMARY KEY,
    username VARCHAR(100) NOT NULL,
    is_authenticated BOOLEAN DEFAULT false,
    auth_method VARCHAR(50),
    session_token VARCHAR(255),
    authenticated_at TIMESTAMP,
    expires_at TIMESTAMP,
    client_ip INET,
    vpn_ip INET,
    UNIQUE(username)
);

CREATE TABLE IF NOT EXISTS resource_audit_log (
    id SERIAL PRIMARY KEY,
    action_type VARCHAR(20) NOT NULL CHECK (action_type IN ('ADD', 'REMOVE', 'UPDATE', 'BULK_ADD', 'BULK_REMOVE', 'VPN_CONNECT', 'VPN_DISCONNECT')),
    resource_type VARCHAR(50),
    resource_value VARCHAR(255),
    performed_by VARCHAR(255) NOT NULL,
    performed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    client_ip INET,
    details JSONB
);

CREATE TABLE IF NOT EXISTS api_tokens (
    id SERIAL PRIMARY KEY,
    token_hash VARCHAR(255) NOT NULL UNIQUE,
    token_name VARCHAR(100) NOT NULL,
    permissions JSONB NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP,
    last_used TIMESTAMP,
    is_active BOOLEAN DEFAULT true
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_blocked_resources_active ON blocked_resources(resource_type, is_active) WHERE is_active = true;
CREATE INDEX IF NOT EXISTS idx_blocked_resources_value ON blocked_resources(resource_value) WHERE is_active = true;
CREATE INDEX IF NOT EXISTS idx_vpc_resources_active ON vpc_resources(cloud_provider, is_active) WHERE is_active = true;

-- Functions
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
    AND (br.resource_type IN ('ip', 'cidr') OR vr.ip_address IS NOT NULL)
    AND CASE 
            WHEN br.resource_type = 'ip' THEN br.resource_value
            WHEN br.resource_type = 'cidr' THEN br.resource_value
            WHEN vr.ip_address IS NOT NULL THEN host(vr.ip_address)
        END IS NOT NULL;
END;
$$ LANGUAGE plpgsql;

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
    AND (br.resource_type = 'domain' OR vr.domain_name IS NOT NULL)
    AND CASE 
            WHEN br.resource_type = 'domain' THEN br.resource_value
            WHEN vr.domain_name IS NOT NULL THEN vr.domain_name
        END IS NOT NULL;
END;
$$ LANGUAGE plpgsql;

-- Default data
INSERT INTO blocked_resources (resource_type, resource_value, description, created_by, is_active) VALUES
('cidr', '172.30.0.0/26', 'Frigga VPC CIDR - auto-blocked', 'system', true),
('ip', '172.30.100.1', 'VPN server IP - conditional access', 'system', true)
ON CONFLICT (resource_type, resource_value) DO NOTHING;

-- Create default API token
INSERT INTO api_tokens (token_hash, token_name, permissions, created_by, is_active) VALUES
('dvarpala-default-admin-key-2024', 'Default Admin Key', '{"read": true, "write": true, "admin": true}', 'system', true)
ON CONFLICT (token_hash) DO NOTHING;
SCHEMA_EOF

    # Apply schema
    sudo -u postgres psql -d dvarpala -f /opt/dvarpala/sql/selective-blocking.sql
    
    log "Database schema for selective blocking created"
}

# Configure OpenVPN for selective routing
setup_selective_openvpn_integration() {
    log "Configuring OpenVPN for selective routing..."
    
    # Replace the existing server.conf with selective routing version
    cat > /etc/openvpn/server/server.conf << 'OPENVPN_EOF'
# Dvarpala OpenVPN Server - Selective Blocking Configuration
port 1194
proto udp
dev tun

# SSL/TLS certificates  
ca /etc/openvpn/server/ca.crt
cert /etc/openvpn/server/server.crt
key /etc/openvpn/server/server.key
dh /etc/openvpn/server/dh.pem
tls-auth /etc/openvpn/server/ta.key 0

# Network configuration for selective routing
server 172.30.100.0 255.255.255.0
topology subnet

# SELECTIVE ROUTING: Only push routes for blocked resources
# Internet traffic goes direct, only blocked IPs route through VPN
push "route 172.30.0.0 255.255.192.0"

# Custom DNS server for domain blocking
push "dhcp-option DNS 172.30.100.1"

# Client configuration directory for individual routing
client-config-dir /etc/openvpn/ccd

# Security settings
cipher AES-256-GCM
auth SHA256
tls-version-min 1.2

# Connection management
keepalive 10 120
compress lz4-v2
push "compress lz4-v2"

# User/group
user nobody
group nogroup

# Persistence
persist-key
persist-tun

# Logging
status /var/log/openvpn/openvpn-status.log
log-append /var/log/openvpn/openvpn.log
verb 3

# Authentication and client management
script-security 3
auth-user-pass-verify /opt/dvarpala/bin/openvpn-auth via-env
client-connect /opt/dvarpala/scripts/client-connect-selective.sh
client-disconnect /opt/dvarpala/scripts/client-disconnect-selective.sh
username-as-common-name
OPENVPN_EOF

    # Create selective client-connect script
    cat > /opt/dvarpala/scripts/client-connect-selective.sh << 'CONNECT_EOF'
#!/bin/bash
# Dvarpala Selective VPN Client Connect Script

LOG_FILE="/var/log/openvpn/client-connect.log"
CONFIG_FILE="$1"
DB_NAME="dvarpala"

echo "$(date): Selective connect - User: $common_name, VPN IP: $ifconfig_pool_remote_ip" >> $LOG_FILE

# Check authentication status
AUTH_STATUS_FILE="/tmp/dvarpala-auth-status-$common_name"

if [[ ! -f "$AUTH_STATUS_FILE" ]]; then
    echo "$(date): User $common_name not authenticated - CAPTIVE PORTAL MODE" >> $LOG_FILE
    echo "route 172.30.100.1 255.255.255.255" > $CONFIG_FILE
    echo "route-nopull" >> $CONFIG_FILE
    exit 0
fi

# User authenticated - apply selective blocking
echo "$(date): User $common_name authenticated - SELECTIVE BLOCKING MODE" >> $LOG_FILE

# Get blocked IPs from database
BLOCKED_IPS=$(sudo -u postgres psql -t -d $DB_NAME -c "SELECT * FROM get_blocked_ips();" 2>/dev/null | grep -v '^$' | tr -d ' ')

echo "# Selective blocking routes (database-driven)" > $CONFIG_FILE
if [[ -n "$BLOCKED_IPS" ]]; then
    while IFS= read -r ip; do
        if [[ -n "$ip" ]]; then
            if [[ "$ip" == *"/"* ]]; then
                echo "route $ip" >> $CONFIG_FILE
            else
                echo "route $ip 255.255.255.255" >> $CONFIG_FILE
            fi
        fi
    done <<< "$BLOCKED_IPS"
else
    echo "# No blocked IPs configured" >> $CONFIG_FILE
fi

echo "$(date): Applied selective blocking for $common_name" >> $LOG_FILE
exit 0
CONNECT_EOF

    chmod +x /opt/dvarpala/scripts/client-connect-selective.sh

    # Create disconnect script
    cat > /opt/dvarpala/scripts/client-disconnect-selective.sh << 'DISCONNECT_EOF'
#!/bin/bash
# Selective VPN Client Disconnect Script

LOG_FILE="/var/log/openvpn/client-disconnect.log"
AUTH_STATUS_FILE="/tmp/dvarpala-auth-status-$common_name"

echo "$(date): Selective disconnect - User: $common_name, Duration: ${time_duration}s" >> $LOG_FILE

if [[ -f "$AUTH_STATUS_FILE" ]]; then
    rm -f "$AUTH_STATUS_FILE"
    echo "$(date): Cleared auth status for $common_name" >> $LOG_FILE
fi

# Log to database
sudo -u postgres psql -d dvarpala -c "
    INSERT INTO resource_audit_log (action_type, resource_value, performed_by, details) 
    VALUES ('VPN_DISCONNECT', '$ifconfig_pool_remote_ip', '$common_name', 
            '{\"duration\": \"${time_duration}s\", \"bytes_received\": \"$bytes_received\", \"bytes_sent\": \"$bytes_sent\"}')
" 2>/dev/null

exit 0
DISCONNECT_EOF

    chmod +x /opt/dvarpala/scripts/client-disconnect-selective.sh

    log "OpenVPN selective routing configured"
}

# Setup API server
setup_api_server() {
    log "Setting up resource management API server..."
    
    # Copy API source code (this would normally be part of the repo)
    mkdir -p /opt/dvarpala/api
    
    # Build API server
    cd /opt/dvarpala/api
    go mod init dvarpala-api
    go mod tidy
    
    # Build the API binary (source code would be copied here)
    # For now, create placeholder
    cat > /opt/dvarpala/api/main.go << 'API_EOF'
package main

import (
    "fmt"
    "log"
    "net/http"
)

func main() {
    http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprintf(w, `{"success": true, "message": "Dvarpala Resource Management API is healthy"}`)
    })
    
    log.Println("Starting Dvarpala Resource Management API on port 8081")
    log.Fatal(http.ListenAndServe(":8081", nil))
}
API_EOF

    go build -o dvarpala-api main.go
    
    # Create systemd service for API
    cat > /etc/systemd/system/dvarpala-api.service << 'SERVICE_EOF'
[Unit]
Description=Dvarpala Resource Management API
After=network.target postgresql.service

[Service]
Type=simple
User=dvarpala
WorkingDirectory=/opt/dvarpala/api
ExecStart=/opt/dvarpala/api/dvarpala-api
Restart=always
RestartSec=10

Environment=DB_HOST=localhost
Environment=DB_PORT=5432
Environment=DB_USER=dvarpala
Environment=DB_NAME=dvarpala
Environment=API_PORT=8081

[Install]
WantedBy=multi-user.target
SERVICE_EOF

    systemctl daemon-reload
    systemctl enable dvarpala-api
    systemctl start dvarpala-api
    
    log "API server configured and started"
}

# Setup network blocking scripts
setup_network_blocking_scripts() {
    log "Setting up network blocking scripts..."
    
    # iptables management script
    cat > /opt/dvarpala/scripts/update-iptables-blocking.sh << 'IPTABLES_EOF'
#!/bin/bash
# Update iptables rules for selective resource blocking

VPN_SUBNET="172.30.100.0/24"
VPC_SUBNET="172.30.0.0/26"
BLACKHOLE_IP="172.30.200.1"
DB_NAME="dvarpala"

# Clear existing Dvarpala rules
iptables -D FORWARD -s $VPN_SUBNET -d $VPC_SUBNET -j DROP 2>/dev/null || true
iptables -D FORWARD -s $VPN_SUBNET -d $BLACKHOLE_IP -j DROP 2>/dev/null || true

# Add VPC blocking rule
iptables -I FORWARD -s $VPN_SUBNET -d $VPC_SUBNET -j DROP
iptables -I FORWARD -s $VPN_SUBNET -d $BLACKHOLE_IP -j DROP

# Get blocked IPs from database and add rules
BLOCKED_IPS=$(sudo -u postgres psql -t -d $DB_NAME -c "
    SELECT resource_value FROM blocked_resources 
    WHERE resource_type IN ('ip', 'cidr') AND is_active = true
" 2>/dev/null | grep -v '^$' | tr -d ' ')

if [[ -n "$BLOCKED_IPS" ]]; then
    while IFS= read -r ip; do
        if [[ -n "$ip" && "$ip" != "172.30.0.0/26" ]]; then
            iptables -I FORWARD -s $VPN_SUBNET -d $ip -j DROP 2>/dev/null
        fi
    done <<< "$BLOCKED_IPS"
fi

# Allow all other traffic
iptables -A FORWARD -s $VPN_SUBNET -j ACCEPT 2>/dev/null || true

echo "$(date): Updated iptables for selective blocking"
IPTABLES_EOF

    chmod +x /opt/dvarpala/scripts/update-iptables-blocking.sh

    # DNS blocking script
    cat > /opt/dvarpala/scripts/update-dns-blocking.sh << 'DNS_EOF'
#!/bin/bash
# Update DNS blocking configuration

DB_NAME="dvarpala"
BLOCKED_DOMAINS_FILE="/opt/dvarpala/config/blocked-domains.conf"
BLACKHOLE_IP="172.30.200.1"

# Get blocked domains from database
DOMAINS=$(sudo -u postgres psql -t -d $DB_NAME -c "SELECT * FROM get_blocked_domains();" 2>/dev/null | grep -v '^$' | tr -d ' ')

echo "# Dynamically generated blocked domains - $(date)" > $BLOCKED_DOMAINS_FILE
if [[ -n "$DOMAINS" ]]; then
    while IFS= read -r domain; do
        if [[ -n "$domain" ]]; then
            echo "address=/$domain/$BLACKHOLE_IP" >> $BLOCKED_DOMAINS_FILE
        fi
    done <<< "$DOMAINS"
fi

systemctl reload dnsmasq 2>/dev/null || true
echo "$(date): Updated DNS blocking configuration"
DNS_EOF

    chmod +x /opt/dvarpala/scripts/update-dns-blocking.sh

    # Run initial setup
    /opt/dvarpala/scripts/update-iptables-blocking.sh
    /opt/dvarpala/scripts/update-dns-blocking.sh

    log "Network blocking scripts configured"
}

# Setup DNS masquerading for domain blocking  
setup_dns_masquerading() {
    log "Setting up DNS-based domain blocking..."
    
    # Configure dnsmasq
    cat > /etc/dnsmasq.d/dvarpala-blocking.conf << 'DNSMASQ_EOF'
# Dvarpala DNS Configuration for Domain Blocking
interface=tun0
bind-interfaces
cache-size=1000
neg-ttl=60
conf-file=/opt/dvarpala/config/blocked-domains.conf
server=8.8.8.8
server=8.8.4.4
log-queries
log-facility=/var/log/dnsmasq-dvarpala.log
DNSMASQ_EOF

    mkdir -p /opt/dvarpala/config
    touch /opt/dvarpala/config/blocked-domains.conf
    
    systemctl enable dnsmasq
    systemctl restart dnsmasq
    
    log "DNS-based domain blocking configured"
}

# Setup discovery automation
setup_discovery_automation() {
    log "Setting up VPC resource discovery automation..."
    
    # Create discovery script placeholder
    cat > /opt/dvarpala/scripts/vpc-discovery.sh << 'DISCOVERY_EOF'
#!/bin/bash
# VPC Resource Discovery Script (placeholder)

echo "$(date): VPC discovery would run here"
echo "This will be implemented with cloud provider APIs"

# Update blocked resources based on discovered VPC resources
sudo -u postgres psql -d dvarpala -c "
    INSERT INTO resource_audit_log (action_type, performed_by, details) 
    VALUES ('VPC_DISCOVERY', 'system', '{\"discovery_run\": true}')
" 2>/dev/null
DISCOVERY_EOF

    chmod +x /opt/dvarpala/scripts/vpc-discovery.sh
    
    # Create cron job for discovery
    cat > /etc/cron.d/dvarpala-discovery << 'CRON_EOF'
# Run VPC discovery every hour
0 * * * * root /opt/dvarpala/scripts/vpc-discovery.sh >> /var/log/dvarpala/discovery.log 2>&1
CRON_EOF

    log "VPC discovery automation configured"
}

# Main integration function
main() {
    log "Integrating selective VPN blocking functionality..."
    
    # Setup all components
    integrate_selective_blocking
    setup_network_blocking_scripts  
    setup_dns_masquerading
    
    log "Selective VPN blocking integration completed successfully"
    log "API Server: http://172.30.100.1:8081"
    log "Default API Key: dvarpala-default-admin-key-2024"
}

# Run if called directly
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi