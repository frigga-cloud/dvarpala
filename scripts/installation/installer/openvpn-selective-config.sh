#!/bin/bash

# Dvarpala Selective VPN Configuration
# This replaces the full captive portal with selective blocking
# Traffic flow: Internet (direct) + Blocked resources (via VPN tunnel)

setup_selective_openvpn() {
    log "Setting up OpenVPN with selective routing..."
    
    # Create enhanced server configuration
    cat > /etc/openvpn/server/server.conf << 'EOF'
# Dvarpala OpenVPN Server - Selective Blocking Configuration
# Allows direct internet access while routing blocked resources through VPN

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

# Always push VPC CIDR for internal resource blocking
push "route 172.30.0.0 255.255.192.0"

# Custom DNS server for domain blocking (dnsmasq on VPN server)
push "dhcp-option DNS 172.30.100.1"

# Do NOT redirect all traffic (this is the key change)
# Commented out: push "redirect-gateway def1 bypass-dhcp"

# Client configuration directory for individual routing
client-config-dir /etc/openvpn/ccd

# Security settings
cipher AES-256-GCM
auth SHA256
tls-version-min 1.2
tls-cipher TLS-ECDHE-ECDSA-WITH-AES-256-GCM-SHA384

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
explicit-exit-notify 1

# Authentication and client management
script-security 3
auth-user-pass-verify /opt/dvarpala/bin/openvpn-auth via-env
client-connect /opt/dvarpala/scripts/client-connect-selective.sh
client-disconnect /opt/dvarpala/scripts/client-disconnect.sh

# Enable username/password auth
username-as-common-name
EOF

    # Create selective client-connect script with database integration
    cat > /opt/dvarpala/scripts/client-connect-selective.sh << 'EOF'
#!/bin/bash
# Dvarpala Selective VPN Client Connect Script
# Routes only blocked resources through VPN, allows direct internet access

LOG_FILE="/var/log/openvpn/client-connect.log"
CONFIG_FILE="$1"
DB_NAME="${DB_NAME:-dvarpala}"
DB_USER="${DB_USER:-dvarpala}"

# Log connection
echo "$(date): Selective VPN connect - User: $common_name, VPN IP: $ifconfig_pool_remote_ip" >> $LOG_FILE

# Check authentication status
AUTH_STATUS_FILE="/tmp/dvarpala-auth-status-$common_name"

if [[ ! -f "$AUTH_STATUS_FILE" ]]; then
    echo "$(date): User $common_name not authenticated - CAPTIVE PORTAL MODE" >> $LOG_FILE
    
    # Captive portal mode: Block everything except portal
    echo "route 172.30.100.1 255.255.255.255" > $CONFIG_FILE
    echo "route-nopull" >> $CONFIG_FILE  # Don't accept any server routes
    
    exit 0
fi

# User is authenticated - apply selective blocking
echo "$(date): User $common_name authenticated - SELECTIVE BLOCKING MODE" >> $LOG_FILE

# Get blocked IPs from database and push as routes
BLOCKED_IPS=$(sudo -u postgres psql -t -d $DB_NAME -c "SELECT * FROM get_blocked_ips();" 2>/dev/null | grep -v '^$' | tr -d ' ')

if [[ -n "$BLOCKED_IPS" ]]; then
    echo "# Blocked IP routes (database-driven)" > $CONFIG_FILE
    while IFS= read -r ip; do
        if [[ -n "$ip" ]]; then
            # Route blocked IPs through VPN tunnel
            if [[ "$ip" == *"/"* ]]; then
                # CIDR notation
                echo "route $ip" >> $CONFIG_FILE
            else
                # Single IP
                echo "route $ip 255.255.255.255" >> $CONFIG_FILE
            fi
            echo "$(date): Routing blocked IP $ip through VPN for $common_name" >> $LOG_FILE
        fi
    done <<< "$BLOCKED_IPS"
else
    echo "# No blocked IPs found in database" > $CONFIG_FILE
    echo "$(date): No blocked IPs configured for $common_name" >> $LOG_FILE
fi

# Log the configuration
echo "$(date): Applied selective blocking routes for $common_name" >> $LOG_FILE
echo "$(date): Internet traffic will go direct, blocked resources through VPN" >> $LOG_FILE

exit 0
EOF

    chmod +x /opt/dvarpala/scripts/client-connect-selective.sh

    # Enhanced client-disconnect script
    cat > /opt/dvarpala/scripts/client-disconnect.sh << 'EOF'
#!/bin/bash
# Dvarpala Selective VPN Client Disconnect Script

LOG_FILE="/var/log/openvpn/client-disconnect.log"
AUTH_STATUS_FILE="/tmp/dvarpala-auth-status-$common_name"

echo "$(date): Selective VPN disconnect - User: $common_name, Duration: ${time_duration}s" >> $LOG_FILE

# Clear authentication status (security feature)
if [[ -f "$AUTH_STATUS_FILE" ]]; then
    rm -f "$AUTH_STATUS_FILE"
    echo "$(date): Cleared auth status for $common_name" >> $LOG_FILE
fi

# Update database with session info
sudo -u postgres psql -d dvarpala -c "
    INSERT INTO resource_audit_log (action_type, resource_value, performed_by, details) 
    VALUES ('VPN_DISCONNECT', '$ifconfig_pool_remote_ip', '$common_name', 
            '{\"duration\": \"${time_duration}s\", \"bytes_received\": \"$bytes_received\", \"bytes_sent\": \"$bytes_sent\"}')
" 2>/dev/null

exit 0
EOF

    chmod +x /opt/dvarpala/scripts/client-disconnect.sh

    # Create client configuration directory
    mkdir -p /etc/openvpn/ccd
    mkdir -p /var/log/openvpn
    
    log "Selective OpenVPN configuration completed"
}

# Configure DNS-based domain blocking with dnsmasq
setup_dns_blocking() {
    log "Setting up DNS-based domain blocking..."
    
    # Install dnsmasq for custom DNS resolution
    if [[ -f /etc/debian_version ]]; then
        apt-get install -y dnsmasq
    elif [[ -f /etc/redhat-release ]]; then
        yum install -y dnsmasq
    fi
    
    # Configure dnsmasq for selective domain blocking
    cat > /etc/dnsmasq.d/dvarpala-blocking.conf << 'EOF'
# Dvarpala DNS Configuration for Domain Blocking
# Listen on VPN interface for VPN clients
interface=tun0
bind-interfaces

# Cache settings
cache-size=1000
neg-ttl=60

# Blocked domains will be resolved to blackhole IP
# This file is dynamically updated by the API
conf-file=/opt/dvarpala/config/blocked-domains.conf

# Forward normal DNS to public resolvers
server=8.8.8.8
server=8.8.4.4
server=1.1.1.1

# Log DNS queries for debugging
log-queries
log-facility=/var/log/dnsmasq-dvarpala.log
EOF

    # Create initial blocked domains configuration
    mkdir -p /opt/dvarpala/config
    cat > /opt/dvarpala/config/blocked-domains.conf << 'EOF'
# Dynamically generated blocked domains
# Format: address=/domain.com/172.30.200.1
# 172.30.200.1 is a blackhole IP that routes through VPN to be blocked

# Initial VPC domain blocking
address=/vpc.internal/172.30.200.1
EOF

    # Script to update DNS blocking from database
    cat > /opt/dvarpala/scripts/update-dns-blocking.sh << 'EOF'
#!/bin/bash
# Update DNS blocking configuration from database

DB_NAME="${DB_NAME:-dvarpala}"
BLOCKED_DOMAINS_FILE="/opt/dvarpala/config/blocked-domains.conf"
BLACKHOLE_IP="172.30.200.1"

# Get blocked domains from database
DOMAINS=$(sudo -u postgres psql -t -d $DB_NAME -c "SELECT * FROM get_blocked_domains();" 2>/dev/null | grep -v '^$' | tr -d ' ')

# Generate new configuration
echo "# Dynamically generated blocked domains - $(date)" > $BLOCKED_DOMAINS_FILE
echo "# Blackhole IP: $BLACKHOLE_IP (routes through VPN to be blocked)" >> $BLOCKED_DOMAINS_FILE
echo "" >> $BLOCKED_DOMAINS_FILE

if [[ -n "$DOMAINS" ]]; then
    while IFS= read -r domain; do
        if [[ -n "$domain" ]]; then
            echo "address=/$domain/$BLACKHOLE_IP" >> $BLOCKED_DOMAINS_FILE
        fi
    done <<< "$DOMAINS"
fi

# Reload dnsmasq to apply changes
systemctl reload dnsmasq

echo "$(date): Updated DNS blocking configuration with $(echo "$DOMAINS" | wc -l) domains"
EOF

    chmod +x /opt/dvarpala/scripts/update-dns-blocking.sh
    
    # Enable and start dnsmasq
    systemctl enable dnsmasq
    systemctl start dnsmasq
    
    log "DNS-based domain blocking configured"
}

# Configure selective iptables rules
setup_selective_firewall() {
    log "Setting up selective firewall rules..."
    
    # Create iptables script for selective blocking
    cat > /opt/dvarpala/scripts/update-iptables-blocking.sh << 'EOF'
#!/bin/bash
# Update iptables rules for selective resource blocking

VPN_SUBNET="172.30.100.0/24"
VPC_SUBNET="172.30.0.0/26"
BLACKHOLE_IP="172.30.200.1"
DB_NAME="${DB_NAME:-dvarpala}"

# Clear existing Dvarpala rules
iptables -D FORWARD -s $VPN_SUBNET -d $VPC_SUBNET -j DROP 2>/dev/null || true
iptables -D FORWARD -s $VPN_SUBNET -d $BLACKHOLE_IP -j DROP 2>/dev/null || true

# Add VPC blocking rule (always block VPC access)
iptables -I FORWARD -s $VPN_SUBNET -d $VPC_SUBNET -j DROP
echo "Blocked VPN access to VPC subnet: $VPC_SUBNET"

# Add blackhole IP blocking (for domain-based blocks)
iptables -I FORWARD -s $VPN_SUBNET -d $BLACKHOLE_IP -j DROP
echo "Blocked VPN access to blackhole IP: $BLACKHOLE_IP"

# Get additional blocked IPs from database and add rules
BLOCKED_IPS=$(sudo -u postgres psql -t -d $DB_NAME -c "
    SELECT resource_value FROM blocked_resources 
    WHERE resource_type IN ('ip', 'cidr') AND is_active = true
" 2>/dev/null | grep -v '^$' | tr -d ' ')

if [[ -n "$BLOCKED_IPS" ]]; then
    while IFS= read -r ip; do
        if [[ -n "$ip" && "$ip" != "172.30.0.0/26" ]]; then  # Skip VPC CIDR (already handled)
            iptables -I FORWARD -s $VPN_SUBNET -d $ip -j DROP 2>/dev/null
            echo "Blocked VPN access to IP: $ip"
        fi
    done <<< "$BLOCKED_IPS"
fi

# Allow all other traffic (this is the key for selective blocking)
iptables -A FORWARD -s $VPN_SUBNET -j ACCEPT 2>/dev/null || true

echo "$(date): Updated iptables for selective blocking"
EOF

    chmod +x /opt/dvarpala/scripts/update-iptables-blocking.sh
    
    # Run initial iptables setup
    /opt/dvarpala/scripts/update-iptables-blocking.sh
    
    log "Selective firewall rules configured"
}