#!/bin/bash
# Dvarpala OAuth Firewall Configuration
# This script sets up the firewall rules for the captive portal with OAuth access

set -euo pipefail

echo "🔥 Configuring Dvarpala captive portal firewall with OAuth access..."

# Function to configure OAuth firewall rules
configure_oauth_firewall() {
    echo "📋 Setting up OAuth provider access rules..."
    
    # Enable IP forwarding
    echo 1 > /proc/sys/net/ipv4/ip_forward
    echo "net.ipv4.ip_forward=1" >> /etc/sysctl.conf
    sysctl -p
    
    # Create ipset for efficient IP matching
    if command -v ipset &> /dev/null; then
        echo "🔧 Creating IP sets for OAuth providers..."
        
        # Microsoft IPs
        ipset create microsoft_oauth hash:net 2>/dev/null || true
        ipset add microsoft_oauth 20.190.128.0/18
        ipset add microsoft_oauth 40.126.0.0/18
        ipset add microsoft_oauth 13.107.6.0/24
        ipset add microsoft_oauth 13.107.9.0/24
        
        # Google IPs
        ipset create google_oauth hash:net 2>/dev/null || true
        ipset add google_oauth 172.217.0.0/16
        ipset add google_oauth 172.253.0.0/16
        ipset add google_oauth 142.250.0.0/15
        ipset add google_oauth 74.125.0.0/16
        
        # GitHub IPs
        ipset create github_oauth hash:net 2>/dev/null || true
        ipset add github_oauth 140.82.112.0/20
        ipset add github_oauth 192.30.252.0/22
        ipset add github_oauth 185.199.108.0/22
        
        # GitLab IPs
        ipset create gitlab_oauth hash:net 2>/dev/null || true
        ipset add gitlab_oauth 35.231.145.151/32
        ipset add gitlab_oauth 34.74.90.64/28
        ipset add gitlab_oauth 34.74.226.0/24
    fi
    
    # Apply the OAuth firewall rules
    bash /opt/dvarpala/scripts/iptables-oauth-rules.sh
    
    echo "✅ OAuth firewall rules configured"
}

# Function to update DNS for OAuth domains
setup_oauth_dns() {
    echo "🌐 Configuring DNS for OAuth domains..."
    
    # Create a custom dnsmasq configuration for OAuth domains
    cat > /etc/dnsmasq.d/dvarpala-oauth.conf << 'EOF'
# Dvarpala OAuth DNS Configuration
# Ensure OAuth domains resolve correctly

# Microsoft
server=/microsoftonline.com/8.8.8.8
server=/microsoft.com/8.8.8.8
server=/live.com/8.8.8.8
server=/windows.net/8.8.8.8

# Google
server=/google.com/8.8.8.8
server=/googleapis.com/8.8.8.8
server=/gstatic.com/8.8.8.8

# GitHub
server=/github.com/8.8.8.8
server=/githubusercontent.com/8.8.8.8

# GitLab
server=/gitlab.com/8.8.8.8
server=/gitlab-static.net/8.8.8.8

# Captive portal detection
address=/captive.dvarpala.local/172.30.100.1
EOF
    
    # Restart dnsmasq if it's running
    if systemctl is-active --quiet dnsmasq; then
        systemctl restart dnsmasq
    fi
    
    echo "✅ DNS configuration for OAuth completed"
}

# Function to create nginx OAuth proxy configuration
setup_oauth_proxy() {
    echo "🔀 Setting up nginx OAuth proxy configuration..."
    
    cat > /etc/nginx/sites-available/dvarpala-oauth-proxy << 'EOF'
# Dvarpala OAuth Proxy Configuration
# This ensures OAuth redirects work correctly through the VPN

server {
    listen 172.30.100.1:443 ssl;
    server_name oauth.dvarpala.local;
    
    ssl_certificate /opt/dvarpala/certs/oauth-proxy.crt;
    ssl_certificate_key /opt/dvarpala/certs/oauth-proxy.key;
    
    # Proxy OAuth callbacks
    location /oauth/callback {
        proxy_pass http://172.30.100.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
EOF
    
    # Generate self-signed certificate for OAuth proxy
    mkdir -p /opt/dvarpala/certs
    openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
        -keyout /opt/dvarpala/certs/oauth-proxy.key \
        -out /opt/dvarpala/certs/oauth-proxy.crt \
        -subj "/C=US/ST=CA/L=SF/O=Dvarpala/CN=oauth.dvarpala.local"
    
    ln -sf /etc/nginx/sites-available/dvarpala-oauth-proxy /etc/nginx/sites-enabled/
    nginx -t && systemctl reload nginx
    
    echo "✅ OAuth proxy configuration completed"
}

# Function to create OpenVPN client configuration for captive mode
create_captive_client_config() {
    echo "📝 Creating OpenVPN captive portal client configuration..."
    
    cat > /etc/openvpn/server/ccd/DEFAULT << 'EOF'
# Default client configuration for captive portal mode
# This is applied to all connecting clients until they authenticate

# Assign to captive portal subnet
ifconfig-push 172.30.100.2 255.255.255.0

# Push routes for OAuth providers only
push "route 172.30.100.0 255.255.255.0"

# Don't push default gateway - selective routing only
# OAuth provider routes will be added by firewall rules
EOF
    
    mkdir -p /etc/openvpn/server/ccd
    echo "✅ Captive client configuration created"
}

# Function to test OAuth connectivity
test_oauth_connectivity() {
    echo "🧪 Testing OAuth provider connectivity..."
    
    # Test domains
    OAUTH_DOMAINS=(
        "accounts.google.com"
        "login.microsoftonline.com"
        "github.com"
        "gitlab.com"
    )
    
    for domain in "${OAUTH_DOMAINS[@]}"; do
        if curl -s --connect-timeout 5 "https://$domain" > /dev/null; then
            echo "✅ $domain is accessible"
        else
            echo "❌ $domain is NOT accessible"
        fi
    done
}

# Main execution
main() {
    # Check if running as root
    if [[ $EUID -ne 0 ]]; then
       echo "❌ This script must be run as root"
       exit 1
    fi
    
    # Configure components
    configure_oauth_firewall
    setup_oauth_dns
    setup_oauth_proxy
    create_captive_client_config
    
    echo ""
    echo "🎉 Dvarpala OAuth firewall configuration completed!"
    echo ""
    echo "📋 Configuration Summary:"
    echo "  • Captive portal subnet: 172.30.100.0/24"
    echo "  • Captive portal address: 172.30.100.1:8080"
    echo "  • OAuth providers allowed: Microsoft 365, Google, GitHub, GitLab"
    echo "  • Helper script: /usr/local/bin/dvarpala-auth-user"
    echo ""
    echo "🔐 After successful OAuth authentication, grant full access with:"
    echo "   dvarpala-auth-user <client-ip>"
    echo ""
    
    # Test connectivity
    test_oauth_connectivity
}

# Run main function
main "$@"