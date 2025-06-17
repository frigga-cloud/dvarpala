#!/bin/bash

# Dvarpala VPN Server Provisioning Script
# This script sets up a complete Dvarpala VPN server from scratch
# Run with: bash <(curl -fsSL https://raw.githubusercontent.com/yourcompany/dvarpala/main/scripts/provisioning/setup-server.sh)

set -euo pipefail

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
DVARPALA_USER="dvarpala"
DVARPALA_HOME="/opt/dvarpala"
POSTGRES_VERSION="15"
REDIS_VERSION="7"
OPENVPN_PORT="1194"
DVARPALA_WEB_PORT="8080"

# Global variables for admin configuration
ADMIN_EMAIL=""
ADMIN_FULL_NAME=""

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if running as root
check_root() {
    if [[ $EUID -ne 0 ]]; then
        log_error "This script must be run as root (use sudo)"
        exit 1
    fi
}

# Collect admin information
collect_admin_information() {
    echo
    echo "=================================================================="
    echo -e "${BLUE}👨‍💼 Administrator Configuration${NC}"
    echo "=================================================================="
    echo
    echo "The admin user will have full access to the Dvarpala VPN system and"
    echo "will be able to manage users, groups, and system configuration."
    echo
    echo "This email address will be used to:"
    echo "• Create the first administrative user account"
    echo "• Validate admin access via OAuth authentication"
    echo "• Manage other users and system settings"
    echo
    
    # Email validation loop
    while true; do
        read -p "Enter administrator email address: " ADMIN_EMAIL
        
        # Basic email validation
        if [[ -z "$ADMIN_EMAIL" ]]; then
            echo -e "${RED}❌${NC} Email address cannot be empty"
            continue
        fi
        
        # Check email format
        if [[ ! "$ADMIN_EMAIL" =~ ^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$ ]]; then
            echo -e "${RED}❌${NC} Invalid email format. Please enter a valid email address"
            continue
        fi
        
        # Confirm email
        echo
        read -p "Confirm administrator email ($ADMIN_EMAIL)? (y/n): " confirm
        if [[ $confirm =~ ^[Yy]$ ]]; then
            break
        fi
    done
    
    # Collect admin full name (optional)
    echo
    read -p "Enter administrator full name (optional): " ADMIN_FULL_NAME
    
    # If no name provided, extract from email
    if [[ -z "$ADMIN_FULL_NAME" ]]; then
        ADMIN_FULL_NAME=$(echo "$ADMIN_EMAIL" | cut -d'@' -f1)
        ADMIN_FULL_NAME=$(echo "$ADMIN_FULL_NAME" | tr '.' ' ' | tr '_' ' ')
        echo "Using default name: $ADMIN_FULL_NAME"
    fi
    
    echo
    echo -e "${GREEN}✅${NC} Administrator configured:"
    echo "   Email: $ADMIN_EMAIL"
    echo "   Name: $ADMIN_FULL_NAME"
    echo
    
    log_success "Administrator information collected"
}

# Detect OS
detect_os() {
    if [[ -f /etc/os-release ]]; then
        . /etc/os-release
        OS=$ID
        VERSION=$VERSION_ID
    else
        log_error "Cannot detect OS. This script supports Ubuntu/Debian/CentOS/RHEL"
        exit 1
    fi
    
    log_info "Detected OS: $OS $VERSION"
}

# Update system packages
update_system() {
    log_info "Updating system packages..."
    
    case $OS in
        ubuntu|debian)
            apt-get update -y
            apt-get upgrade -y
            ;;
        centos|rhel|rocky|almalinux)
            yum update -y
            ;;
        *)
            log_error "Unsupported OS: $OS"
            exit 1
            ;;
    esac
    
    log_success "System packages updated"
}

# Install essential packages
install_essentials() {
    log_info "Installing essential packages..."
    
    case $OS in
        ubuntu|debian)
            apt-get install -y \
                curl wget gnupg2 software-properties-common \
                ca-certificates lsb-release \
                ufw iptables-persistent \
                systemd-resolved \
                zip unzip \
                openssl \
                net-tools
            ;;
        centos|rhel|rocky|almalinux)
            yum install -y \
                curl wget gnupg2 \
                ca-certificates \
                firewalld \
                zip unzip \
                openssl \
                net-tools
            ;;
    esac
    
    log_success "Essential packages installed"
}

# Create dvarpala user
create_dvarpala_user() {
    log_info "Creating dvarpala system user..."
    
    if ! id "$DVARPALA_USER" &>/dev/null; then
        useradd -r -s /bin/bash -d "$DVARPALA_HOME" -m "$DVARPALA_USER"
        log_success "Created user: $DVARPALA_USER"
    else
        log_warning "User $DVARPALA_USER already exists"
    fi
    
    # Create necessary directories
    mkdir -p "$DVARPALA_HOME"/{bin,config,logs,certs,data}
    chown -R "$DVARPALA_USER:$DVARPALA_USER" "$DVARPALA_HOME"
    
    # Store admin information
    cat > "$DVARPALA_HOME/config/.admin_info" << EOF
ADMIN_EMAIL="$ADMIN_EMAIL"
ADMIN_FULL_NAME="$ADMIN_FULL_NAME"
ADMIN_CREATED_AT="$(date)"
EOF
    
    chown "$DVARPALA_USER:$DVARPALA_USER" "$DVARPALA_HOME/config/.admin_info"
    chmod 600 "$DVARPALA_HOME/config/.admin_info"
}

# Install PostgreSQL
install_postgresql() {
    log_info "Installing PostgreSQL..."
    
    case $OS in
        ubuntu|debian)
            # Add PostgreSQL official repository
            curl -fsSL https://www.postgresql.org/media/keys/ACCC4CF8.asc | gpg --dearmor -o /usr/share/keyrings/postgresql-keyring.gpg
            echo "deb [signed-by=/usr/share/keyrings/postgresql-keyring.gpg] http://apt.postgresql.org/pub/repos/apt $(lsb_release -cs)-pgdg main" > /etc/apt/sources.list.d/postgresql.list
            apt-get update -y
            apt-get install -y postgresql-$POSTGRES_VERSION postgresql-client-$POSTGRES_VERSION postgresql-contrib-$POSTGRES_VERSION
            ;;
        centos|rhel|rocky|almalinux)
            # Install PostgreSQL repository
            yum install -y https://download.postgresql.org/pub/repos/yum/reporpms/EL-$(rpm -E %{rhel})-x86_64/pgdg-redhat-repo-latest.noarch.rpm
            yum install -y postgresql${POSTGRES_VERSION}-server postgresql${POSTGRES_VERSION}-contrib
            /usr/pgsql-${POSTGRES_VERSION}/bin/postgresql-${POSTGRES_VERSION}-setup initdb
            ;;
    esac
    
    # Start and enable PostgreSQL
    systemctl start postgresql
    systemctl enable postgresql
    
    log_success "PostgreSQL installed and started"
}

# Configure PostgreSQL for Dvarpala
configure_postgresql() {
    log_info "Configuring PostgreSQL for Dvarpala..."
    
    # Generate random password
    POSTGRES_PASSWORD=$(openssl rand -base64 32)
    
    # Create database and user
    sudo -u postgres psql << EOF
CREATE DATABASE dvarpala;
CREATE USER dvarpala WITH ENCRYPTED PASSWORD '$POSTGRES_PASSWORD';
GRANT ALL PRIVILEGES ON DATABASE dvarpala TO dvarpala;
ALTER USER dvarpala CREATEDB;
EOF

    # Save credentials
    cat > "$DVARPALA_HOME/config/.db_credentials" << EOF
DB_HOST=localhost
DB_PORT=5432
DB_NAME=dvarpala
DB_USER=dvarpala
DB_PASSWORD=$POSTGRES_PASSWORD
EOF

    chown "$DVARPALA_USER:$DVARPALA_USER" "$DVARPALA_HOME/config/.db_credentials"
    chmod 600 "$DVARPALA_HOME/config/.db_credentials"
    
    log_success "PostgreSQL configured for Dvarpala"
}

# Install Redis
install_redis() {
    log_info "Installing Redis..."
    
    case $OS in
        ubuntu|debian)
            apt-get install -y redis-server
            ;;
        centos|rhel|rocky|almalinux)
            yum install -y redis
            ;;
    esac
    
    # Configure Redis
    sed -i 's/^# maxmemory <bytes>/maxmemory 256mb/' /etc/redis/redis.conf 2>/dev/null || \
    sed -i 's/^# maxmemory <bytes>/maxmemory 256mb/' /etc/redis.conf
    
    systemctl start redis
    systemctl enable redis
    
    log_success "Redis installed and configured"
}

# Install Go
install_go() {
    log_info "Installing Go..."
    
    GO_VERSION="1.21.5"
    GO_TARBALL="go${GO_VERSION}.linux-amd64.tar.gz"
    
    # Download and install Go
    cd /tmp
    wget -q "https://golang.org/dl/${GO_TARBALL}"
    tar -xzf "$GO_TARBALL"
    rm -rf /usr/local/go
    mv go /usr/local/
    
    # Set up Go environment
    cat > /etc/profile.d/go.sh << 'EOF'
export GOROOT=/usr/local/go
export GOPATH=/opt/go
export PATH=$GOROOT/bin:$GOPATH/bin:$PATH
EOF

    source /etc/profile.d/go.sh
    mkdir -p /opt/go
    chown "$DVARPALA_USER:$DVARPALA_USER" /opt/go
    
    log_success "Go installed (version $GO_VERSION)"
}

# Install OpenVPN server
install_openvpn() {
    log_info "Installing OpenVPN server..."
    
    case $OS in
        ubuntu|debian)
            apt-get install -y openvpn easy-rsa
            ;;
        centos|rhel|rocky|almalinux)
            yum install -y epel-release
            yum install -y openvpn easy-rsa
            ;;
    esac
    
    log_success "OpenVPN server installed"
}

# Configure OpenVPN server with 2-step access
configure_openvpn() {
    log_info "Configuring OpenVPN server with 2-step access..."
    
    # Get server IP
    SERVER_IP=$(curl -s http://ipv4.icanhazip.com || curl -s http://checkip.amazonaws.com || echo "127.0.0.1")
    
    # Set up Easy-RSA
    cp -r /usr/share/easy-rsa /etc/openvpn/
    cd /etc/openvpn/easy-rsa
    
    # Configure Easy-RSA variables
    cat > vars << EOF
set_var EASYRSA_REQ_COUNTRY    "US"
set_var EASYRSA_REQ_PROVINCE   "California"
set_var EASYRSA_REQ_CITY       "San Francisco"
set_var EASYRSA_REQ_ORG        "Dvarpala VPN"
set_var EASYRSA_REQ_EMAIL      "admin@dvarpala.local"
set_var EASYRSA_REQ_OU         "Dvarpala VPN CA"
set_var EASYRSA_KEY_SIZE       2048
set_var EASYRSA_ALGO           rsa
set_var EASYRSA_CA_EXPIRE      3650
set_var EASYRSA_CERT_EXPIRE    365  # Shorter expiry for temporary certificates
EOF

    # Initialize PKI
    ./easyrsa init-pki
    echo -e "\n" | ./easyrsa build-ca nopass
    ./easyrsa gen-req server nopass
    ./easyrsa sign-req server server
    ./easyrsa gen-dh
    openvpn --genkey secret pki/ta.key
    
    # Generate admin client certificate (permanent)
    ./easyrsa gen-req admin nopass
    ./easyrsa sign-req client admin
    
    # Create directories for temporary certificates
    mkdir -p /etc/openvpn/temp-certs
    mkdir -p /etc/openvpn/client-configs
    
    # Create OpenVPN server configuration with 2-step access
    cat > /etc/openvpn/server.conf << EOF
# OpenVPN Server Configuration for Dvarpala 2-Step Access
port $OPENVPN_PORT
proto udp
dev tun

# Certificates and keys
ca /etc/openvpn/easy-rsa/pki/ca.crt
cert /etc/openvpn/easy-rsa/pki/issued/server.crt
key /etc/openvpn/easy-rsa/pki/private/server.key
dh /etc/openvpn/easy-rsa/pki/dh.pem
tls-auth /etc/openvpn/easy-rsa/pki/ta.key 0

# Network configuration - Captive Portal Network (Initial Access)
server 192.168.100.0 255.255.255.0
ifconfig-pool-persist /var/log/openvpn/ipp.txt

# Initial captive portal routes - Only allow access to Dvarpala dashboard
# No internet access until authenticated
push "route 192.168.100.1 255.255.255.255"
push "dhcp-option DNS 192.168.100.1"

# Client configuration
duplicate-cn
client-to-client

# Security
keepalive 10 120
tls-version-min 1.2
cipher AES-256-GCM
auth SHA256
user nobody
group nogroup

# Logging
status /var/log/openvpn/openvpn-status.log
log-append /var/log/openvpn/openvpn.log
verb 3
explicit-exit-notify 1

# Dvarpala 2-step authentication
auth-user-pass-verify /opt/dvarpala/bin/openvpn-auth via-env
script-security 3
username-as-common-name

# Client connect/disconnect scripts for dynamic routing
client-connect /opt/dvarpala/bin/client-connect
client-disconnect /opt/dvarpala/bin/client-disconnect

# Management interface for dynamic certificate management
management localhost 7505

# Certificate revocation list
crl-verify /etc/openvpn/easy-rsa/pki/crl.pem

# Session timeout (force reconnection for re-authentication)
reneg-sec 3600
EOF

    # Create client-connect script for dynamic routing
    cat > /opt/dvarpala/bin/client-connect << 'EOF'
#!/bin/bash
# Dvarpala Client Connect Script
# This script is called when a client connects to assign network access

CLIENT_IP="$ifconfig_pool_remote_ip"
COMMON_NAME="$common_name"
LOG_FILE="/var/log/openvpn/client-connect.log"

echo "$(date): Client $COMMON_NAME connected with IP $CLIENT_IP" >> "$LOG_FILE"

# Check if user is authenticated via Dvarpala API
AUTH_STATUS=$(curl -s "http://localhost:8080/api/internal/check-auth/$COMMON_NAME" || echo "unauthenticated")

if [ "$AUTH_STATUS" = "authenticated" ]; then
    # Grant full access - push all internet routes
    echo "push \"redirect-gateway def1 bypass-dhcp\"" > "$1"
    echo "push \"dhcp-option DNS 8.8.8.8\"" >> "$1"
    echo "push \"dhcp-option DNS 8.8.4.4\"" >> "$1"
    echo "$(date): Full access granted to $COMMON_NAME" >> "$LOG_FILE"
else
    # Captive portal mode - only allow access to Dvarpala dashboard
    echo "push \"route 192.168.100.1 255.255.255.255\"" > "$1"
    echo "push \"dhcp-option DNS 192.168.100.1\"" >> "$1"
    echo "$(date): Captive portal access for $COMMON_NAME" >> "$LOG_FILE"
fi
EOF

    # Create client-disconnect script
    cat > /opt/dvarpala/bin/client-disconnect << 'EOF'
#!/bin/bash
# Dvarpala Client Disconnect Script
# This script is called when a client disconnects to clean up sessions

COMMON_NAME="$common_name"
LOG_FILE="/var/log/openvpn/client-disconnect.log"

echo "$(date): Client $COMMON_NAME disconnected" >> "$LOG_FILE"

# Revoke temporary certificate if it exists
if [ -f "/etc/openvpn/temp-certs/$COMMON_NAME.crt" ]; then
    # Add to CRL and regenerate
    cd /etc/openvpn/easy-rsa
    ./easyrsa revoke "$COMMON_NAME" || true
    ./easyrsa gen-crl
    cp pki/crl.pem /etc/openvpn/easy-rsa/pki/crl.pem
    
    # Remove temporary certificate files
    rm -f "/etc/openvpn/temp-certs/$COMMON_NAME"*
    rm -f "/etc/openvpn/client-configs/$COMMON_NAME.ovpn"
    
    echo "$(date): Temporary certificate revoked for $COMMON_NAME" >> "$LOG_FILE"
fi

# Clear authentication status in Dvarpala
curl -s -X DELETE "http://localhost:8080/api/internal/clear-auth/$COMMON_NAME" || true
EOF

    # Make scripts executable
    chmod +x /opt/dvarpala/bin/client-connect
    chmod +x /opt/dvarpala/bin/client-disconnect
    
    # Create initial empty CRL
    cd /etc/openvpn/easy-rsa
    ./easyrsa gen-crl
    
    # Create log directory
    mkdir -p /var/log/openvpn
    
    # Enable IP forwarding
    echo 'net.ipv4.ip_forward=1' >> /etc/sysctl.conf
    sysctl -p
    
    # Configure firewall for 2-step access
    configure_firewall_2step
    
    log_success "OpenVPN server configured with 2-step access"
}

# Configure firewall for 2-step access
configure_firewall_2step() {
    log_info "Configuring firewall for 2-step VPN access..."
    
    case $OS in
        ubuntu|debian)
            # UFW configuration
            ufw --force reset
            ufw default deny incoming
            ufw default allow outgoing
            
            # Allow SSH
            ufw allow ssh
            
            # Allow OpenVPN
            ufw allow $OPENVPN_PORT/udp
            
            # Allow Dvarpala web interface from VPN networks
            ufw allow in on tun0 to any port $DVARPALA_WEB_PORT
            
            # Allow established connections
            ufw allow out on tun0 from any to any
            ufw allow in on tun0 from any to any
            
            # Create advanced NAT and filtering rules
            cat > /etc/ufw/before.rules << 'EOF'
# NAT rules for OpenVPN 2-step access
*nat
:PREROUTING ACCEPT [0:0]
:INPUT ACCEPT [0:0]
:OUTPUT ACCEPT [0:0]
:POSTROUTING ACCEPT [0:0]

# Masquerade for both networks
-A POSTROUTING -s 192.168.100.0/24 -o eth0 -j MASQUERADE
-A POSTROUTING -s 10.8.0.0/24 -o eth0 -j MASQUERADE

COMMIT

*filter
:ufw-before-input - [0:0]
:ufw-before-output - [0:0]
:ufw-before-forward - [0:0]
:ufw-not-local - [0:0]

# Captive portal network restrictions (192.168.100.0/24)
# Only allow access to Dvarpala dashboard
-A ufw-before-forward -s 192.168.100.0/24 -d 192.168.100.1 -p tcp --dport 8080 -j ACCEPT
-A ufw-before-forward -s 192.168.100.0/24 -d 192.168.100.1 -p udp --dport 53 -j ACCEPT
-A ufw-before-forward -s 192.168.100.0/24 -j DROP

# Full access network (10.8.0.0/24) - allow everything
-A ufw-before-forward -s 10.8.0.0/24 -j ACCEPT

# Allow VPN traffic
-A ufw-before-input -i tun0 -j ACCEPT
-A ufw-before-output -o tun0 -j ACCEPT

EOF
            
            ufw --force enable
            ;;
        centos|rhel|rocky|almalinux)
            # Firewalld configuration
            systemctl start firewalld
            systemctl enable firewalld
            
            firewall-cmd --permanent --add-service=ssh
            firewall-cmd --permanent --add-port=$OPENVPN_PORT/udp
            firewall-cmd --permanent --add-masquerade
            
            # Create custom zones for different access levels
            firewall-cmd --permanent --new-zone=captive-portal
            firewall-cmd --permanent --new-zone=full-access
            
            # Captive portal zone - restricted access
            firewall-cmd --permanent --zone=captive-portal --add-source=192.168.100.0/24
            firewall-cmd --permanent --zone=captive-portal --add-port=$DVARPALA_WEB_PORT/tcp
            firewall-cmd --permanent --zone=captive-portal --add-service=dns
            firewall-cmd --permanent --zone=captive-portal --set-target=DROP
            
            # Full access zone - unrestricted
            firewall-cmd --permanent --zone=full-access --add-source=10.8.0.0/24
            firewall-cmd --permanent --zone=full-access --set-target=ACCEPT
            
            # NAT rules for both networks
            firewall-cmd --permanent --direct --passthrough ipv4 -t nat -A POSTROUTING -s 192.168.100.0/24 -o eth0 -j MASQUERADE
            firewall-cmd --permanent --direct --passthrough ipv4 -t nat -A POSTROUTING -s 10.8.0.0/24 -o eth0 -j MASQUERADE
            
            firewall-cmd --reload
            ;;
    esac
    
    log_success "Firewall configured for 2-step access"
}

# Create temporary certificate generation script
create_temp_cert_scripts() {
    log_info "Creating temporary certificate management scripts..."
    
    # Create certificate generation script
    cat > /opt/dvarpala/bin/generate-temp-cert << 'EOF'
#!/bin/bash
# Generate temporary OpenVPN certificate for user access

if [ "$#" -ne 1 ]; then
    echo "Usage: $0 <username>"
    exit 1
fi

USERNAME="$1"
CERT_DIR="/etc/openvpn/temp-certs"
CONFIG_DIR="/etc/openvpn/client-configs"
EASYRSA_DIR="/etc/openvpn/easy-rsa"

# Create directories if they don't exist
mkdir -p "$CERT_DIR"
mkdir -p "$CONFIG_DIR"

cd "$EASYRSA_DIR"

# Generate certificate request and sign it
./easyrsa gen-req "$USERNAME" nopass 2>/dev/null
./easyrsa sign-req client "$USERNAME" 2>/dev/null

if [ $? -eq 0 ]; then
    # Copy certificates to temp directory
    cp "pki/issued/$USERNAME.crt" "$CERT_DIR/"
    cp "pki/private/$USERNAME.key" "$CERT_DIR/"
    
    # Generate client configuration
    cat > "$CONFIG_DIR/$USERNAME.ovpn" << EOL
client
dev tun
proto udp
remote $(curl -s http://ipv4.icanhazip.com) 1194
resolv-retry infinite
nobind
user nobody
group nogroup
persist-key
persist-tun
cipher AES-256-GCM
auth SHA256
key-direction 1
remote-cert-tls server
verb 3

<ca>
$(cat pki/ca.crt)
</ca>

<cert>
$(cat pki/issued/$USERNAME.crt)
</cert>

<key>
$(cat pki/private/$USERNAME.key)
</key>

<tls-auth>
$(cat pki/ta.key)
</tls-auth>
EOL

    echo "Temporary certificate generated for $USERNAME"
    echo "Configuration file: $CONFIG_DIR/$USERNAME.ovpn"
else
    echo "Failed to generate certificate for $USERNAME"
    exit 1
fi
EOF

    # Create certificate revocation script
    cat > /opt/dvarpala/bin/revoke-temp-cert << 'EOF'
#!/bin/bash
# Revoke temporary OpenVPN certificate

if [ "$#" -ne 1 ]; then
    echo "Usage: $0 <username>"
    exit 1
fi

USERNAME="$1"
CERT_DIR="/etc/openvpn/temp-certs"
CONFIG_DIR="/etc/openvpn/client-configs"
EASYRSA_DIR="/etc/openvpn/easy-rsa"

cd "$EASYRSA_DIR"

# Revoke certificate
./easyrsa revoke "$USERNAME" 2>/dev/null

# Regenerate CRL
./easyrsa gen-crl

# Copy updated CRL
cp pki/crl.pem /etc/openvpn/easy-rsa/pki/crl.pem

# Remove certificate files
rm -f "$CERT_DIR/$USERNAME.crt"
rm -f "$CERT_DIR/$USERNAME.key"
rm -f "$CONFIG_DIR/$USERNAME.ovpn"

# Signal OpenVPN to reload CRL
if pgrep openvpn > /dev/null; then
    kill -USR1 $(pgrep openvpn)
fi

echo "Certificate revoked for $USERNAME"
EOF

    # Create OpenVPN authentication script
    cat > /opt/dvarpala/bin/openvpn-auth << 'EOF'
#!/bin/bash
# OpenVPN authentication script for Dvarpala 2-step access

# This script validates username/password for temporary access
# For 2-step access, we allow any connection to reach captive portal

USERNAME="$username"
PASSWORD="$password"
LOG_FILE="/var/log/openvpn/auth.log"

echo "$(date): Authentication attempt for user: $USERNAME" >> "$LOG_FILE"

# For temporary access, we can use a simple system or validate against Dvarpala API
# For now, allow all connections to reach captive portal
if [ -n "$USERNAME" ] && [ -n "$PASSWORD" ]; then
    echo "$(date): Temporary access granted to $USERNAME" >> "$LOG_FILE"
    exit 0
else
    echo "$(date): Authentication failed for $USERNAME" >> "$LOG_FILE"
    exit 1
fi
EOF

    # Make scripts executable
    chmod +x /opt/dvarpala/bin/generate-temp-cert
    chmod +x /opt/dvarpala/bin/revoke-temp-cert
    chmod +x /opt/dvarpala/bin/openvpn-auth
    
    # Set proper ownership
    chown "$DVARPALA_USER:$DVARPALA_USER" /opt/dvarpala/bin/generate-temp-cert
    chown "$DVARPALA_USER:$DVARPALA_USER" /opt/dvarpala/bin/revoke-temp-cert
    chown "$DVARPALA_USER:$DVARPALA_USER" /opt/dvarpala/bin/openvpn-auth
    
    log_success "Temporary certificate management scripts created"
}

# Download and install Dvarpala application
install_dvarpala_app() {
    log_info "Installing Dvarpala 2-Step VPN application..."
    
    # Clone repository (in production, this would download release binaries)
    cd /tmp
    
    # For now, we'll create placeholder binaries
    # In production: wget https://github.com/yourcompany/dvarpala/releases/latest/download/dvarpala-linux-amd64.tar.gz
    
    # Create placeholder binaries (these would be replaced by actual builds)
    mkdir -p dvarpala-release/bin
    
    # Create basic Go module for building
    cat > dvarpala-release/go.mod << 'EOF'
module dvarpala
go 1.21

require (
    github.com/gin-gonic/gin v1.9.1
    github.com/golang-jwt/jwt/v5 v5.0.0
)
EOF

    # Create comprehensive main.go for 2-step authentication server
    cat > dvarpala-release/main.go << 'EOF'
package main

import (
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "os"
    "sync"
    "time"
    
    "github.com/gin-gonic/gin"
    "github.com/golang-jwt/jwt/v5"
)

// In-memory storage for authenticated users (in production, use Redis/database)
var (
    authenticatedUsers = make(map[string]time.Time)
    userMutex         = sync.RWMutex{}
    jwtSecret         = []byte(os.Getenv("AUTH_JWT_SECRET"))
)

type AuthStatus struct {
    Authenticated bool   `json:"authenticated"`
    User         string `json:"user,omitempty"`
    ExpiresAt    string `json:"expires_at,omitempty"`
}

type OAuthCallback struct {
    Provider string `json:"provider"`
    Code     string `json:"code"`
    State    string `json:"state"`
}

func main() {
    port := os.Getenv("SERVER_PORT")
    if port == "" {
        port = "8080"
    }
    
    // Set Gin to release mode in production
    if os.Getenv("SERVER_MODE") == "production" {
        gin.SetMode(gin.ReleaseMode)
    }
    
    r := gin.Default()
    
    // Serve static files for captive portal
    r.Static("/static", "./static")
    r.LoadHTMLGlob("templates/*")
    
    // Main captive portal page
    r.GET("/", func(c *gin.Context) {
        c.HTML(http.StatusOK, "captive-portal.html", gin.H{
            "title": "Dvarpala VPN - Authentication Required",
        })
    })
    
    // OAuth authentication endpoints
    r.GET("/auth/google", handleOAuthRedirect("google"))
    r.GET("/auth/microsoft", handleOAuthRedirect("microsoft"))
    r.GET("/auth/github", handleOAuthRedirect("github"))
    r.GET("/auth/gitlab", handleOAuthRedirect("gitlab"))
    
    // OAuth callback handlers
    r.GET("/auth/google/callback", handleOAuthCallback("google"))
    r.GET("/auth/microsoft/callback", handleOAuthCallback("microsoft"))
    r.GET("/auth/github/callback", handleOAuthCallback("github"))
    r.GET("/auth/gitlab/callback", handleOAuthCallback("gitlab"))
    
    // Internal API for OpenVPN scripts
    r.GET("/api/internal/check-auth/:username", func(c *gin.Context) {
        username := c.Param("username")
        
        userMutex.RLock()
        expiresAt, exists := authenticatedUsers[username]
        userMutex.RUnlock()
        
        if exists && time.Now().Before(expiresAt) {
            c.String(http.StatusOK, "authenticated")
        } else {
            // Clean up expired session
            if exists {
                userMutex.Lock()
                delete(authenticatedUsers, username)
                userMutex.Unlock()
            }
            c.String(http.StatusOK, "unauthenticated")
        }
    })
    
    r.DELETE("/api/internal/clear-auth/:username", func(c *gin.Context) {
        username := c.Param("username")
        
        userMutex.Lock()
        delete(authenticatedUsers, username)
        userMutex.Unlock()
        
        c.JSON(http.StatusOK, gin.H{"status": "cleared"})
    })
    
    // Dashboard (full access only)
    r.GET("/dashboard", func(c *gin.Context) {
        // Check if user has full access
        clientIP := c.ClientIP()
        if !isFullAccessNetwork(clientIP) {
            c.Redirect(http.StatusFound, "/")
            return
        }
        
        c.HTML(http.StatusOK, "dashboard.html", gin.H{
            "title": "Dvarpala VPN Dashboard",
        })
    })
    
    // Create basic HTML templates
    createTemplates()
    
    log.Printf("Dvarpala 2-Step VPN server starting on port %s", port)
    log.Fatal(http.ListenAndServe(":"+port, r))
}

func handleOAuthRedirect(provider string) gin.HandlerFunc {
    return func(c *gin.Context) {
        // In production, implement proper OAuth redirect
        // For now, simulate OAuth process
        c.HTML(http.StatusOK, "oauth-success.html", gin.H{
            "provider": provider,
            "username": "user@example.com", // Would come from OAuth
        })
    }
}

func handleOAuthCallback(provider string) gin.HandlerFunc {
    return func(c *gin.Context) {
        // Simulate OAuth validation
        username := "user@example.com" // Would come from OAuth validation
        
        // Check if user exists in database
        if validateUser(username) {
            // Grant authentication for 1 hour
            expiresAt := time.Now().Add(1 * time.Hour)
            
            userMutex.Lock()
            authenticatedUsers[username] = expiresAt
            userMutex.Unlock()
            
            log.Printf("User %s authenticated via %s", username, provider)
            
            c.HTML(http.StatusOK, "auth-success.html", gin.H{
                "username": username,
                "provider": provider,
            })
        } else {
            c.HTML(http.StatusForbidden, "auth-denied.html", gin.H{
                "reason": "User not authorized for VPN access",
            })
        }
    }
}

func validateUser(username string) bool {
    // In production, check against database
    // For now, allow all users
    return true
}

func isFullAccessNetwork(ip string) bool {
    // Check if IP is in full access network (10.8.0.0/24)
    // For now, simple check
    return ip != "192.168.100.1" // Simplified for demo
}

func createTemplates() {
    // Create templates directory
    os.MkdirAll("templates", 0755)
    
    // Captive portal template
    captivePortalHTML := `<!DOCTYPE html>
<html>
<head>
    <title>{{ .title }}</title>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <style>
        body { font-family: Arial, sans-serif; background: #f5f5f5; margin: 0; padding: 20px; }
        .container { max-width: 500px; margin: 0 auto; background: white; padding: 30px; border-radius: 10px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
        h1 { color: #333; text-align: center; }
        .auth-buttons { margin: 20px 0; }
        .auth-btn { display: block; width: 100%; padding: 12px; margin: 10px 0; background: #007bff; color: white; text-decoration: none; border-radius: 5px; text-align: center; }
        .auth-btn:hover { background: #0056b3; }
    </style>
</head>
<body>
    <div class="container">
        <h1>🔐 VPN Authentication Required</h1>
        <p>Welcome to Dvarpala VPN. Please authenticate to gain full internet access.</p>
        <div class="auth-buttons">
            <a href="/auth/google" class="auth-btn">Sign in with Google</a>
            <a href="/auth/microsoft" class="auth-btn">Sign in with Microsoft</a>
            <a href="/auth/github" class="auth-btn">Sign in with GitHub</a>
            <a href="/auth/gitlab" class="auth-btn">Sign in with GitLab</a>
        </div>
        <p><small>After authentication, you will receive full VPN access until you disconnect.</small></p>
    </div>
</body>
</html>`

    // Write template files
    os.WriteFile("templates/captive-portal.html", []byte(captivePortalHTML), 0644)
    
    // Other template files would be created similarly...
    authSuccessHTML := `<!DOCTYPE html>
<html>
<head><title>Authentication Successful</title></head>
<body>
    <h1>✅ Authentication Successful</h1>
    <p>Welcome {{ .username }}! You now have full VPN access.</p>
    <p>You can now access the internet normally.</p>
</body>
</html>`
    os.WriteFile("templates/auth-success.html", []byte(authSuccessHTML), 0644)
}
EOF

    # Build the application (this will fail without dependencies in real scenario)
    cd dvarpala-release
    source /etc/profile.d/go.sh
    
    # Create template-based Go application
    cat > template-main.go << 'EOF'
package main

import (
    "encoding/json"
    "fmt"
    "html/template"
    "log"
    "net/http"
    "os"
    "path/filepath"
    "strings"
    "sync"
    "time"
)

var (
    authenticatedUsers = make(map[string]time.Time)
    userMutex         = sync.RWMutex{}
    templates         *template.Template
    config            *PortalConfig
)

type PortalConfig struct {
    Branding struct {
        CompanyName    string `json:"company_name"`
        LogoEmoji      string `json:"logo_emoji"`
        PrimaryColor   string `json:"primary_color"`
        SecondaryColor string `json:"secondary_color"`
        AccentColor    string `json:"accent_color"`
    } `json:"branding"`
    Portal struct {
        Title          string `json:"title"`
        Subtitle       string `json:"subtitle"`
        WelcomeMessage string `json:"welcome_message"`
        SessionInfo    string `json:"session_info"`
    } `json:"portal"`
    Support struct {
        Email           string `json:"email"`
        Phone           string `json:"phone"`
        Slack           string `json:"slack"`
        ShowContactInfo bool   `json:"show_contact_info"`
    } `json:"support"`
}

type PageData struct {
    Config    *PortalConfig
    Username  string
    Provider  string
    Timestamp string
    Reason    string
    ErrorCode string
}

func loadConfig() {
    configPath := "/opt/dvarpala/web/config/portal-config.json"
    
    // Default config if file doesn't exist
    config = &PortalConfig{}
    config.Branding.CompanyName = "Your Organization"
    config.Branding.LogoEmoji = "🛡️"
    config.Portal.Title = "Dvarpala VPN - Authentication Required"
    config.Portal.Subtitle = "Zero Trust VPN Gateway"
    config.Portal.WelcomeMessage = "Please authenticate to gain full internet access."
    config.Support.Email = "support@yourorganization.com"
    
    // Try to load from file
    if data, err := os.ReadFile(configPath); err == nil {
        if err := json.Unmarshal(data, config); err != nil {
            log.Printf("Warning: Failed to parse config file: %v", err)
        } else {
            log.Printf("Loaded portal configuration from %s", configPath)
        }
    } else {
        log.Printf("Using default configuration (config file not found: %s)", configPath)
    }
}

func loadTemplates() {
    templatesPath := "/opt/dvarpala/web/templates"
    
    // Check if templates directory exists
    if _, err := os.Stat(templatesPath); os.IsNotExist(err) {
        log.Printf("Templates directory not found at %s, using embedded templates", templatesPath)
        return
    }
    
    // Load templates
    var err error
    templates, err = template.ParseGlob(filepath.Join(templatesPath, "*.html"))
    if err != nil {
        log.Printf("Warning: Failed to load templates: %v", err)
        templates = nil
        return
    }
    
    log.Printf("Loaded templates from %s", templatesPath)
}

func serveTemplate(w http.ResponseWriter, templateName string, data PageData) {
    if templates == nil {
        // Fallback to embedded HTML
        serveFallbackHTML(w, templateName, data)
        return
    }
    
    w.Header().Set("Content-Type", "text/html")
    if err := templates.ExecuteTemplate(w, templateName, data); err != nil {
        log.Printf("Template execution error: %v", err)
        serveFallbackHTML(w, templateName, data)
    }
}

func serveFallbackHTML(w http.ResponseWriter, templateName string, data PageData) {
    w.Header().Set("Content-Type", "text/html")
    
    switch templateName {
    case "captive-portal.html":
        html := `<!DOCTYPE html>
<html>
<head>
    <title>Dvarpala VPN - Authentication Required</title>
    <style>
        body { font-family: Arial, sans-serif; background: #f5f5f5; margin: 0; padding: 20px; }
        .container { max-width: 500px; margin: 0 auto; background: white; padding: 30px; border-radius: 10px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
        h1 { color: #333; text-align: center; }
        .auth-btn { display: block; width: 100%%; padding: 12px; margin: 10px 0; background: #007bff; color: white; text-decoration: none; border-radius: 5px; text-align: center; }
        .auth-btn:hover { background: #0056b3; }
    </style>
</head>
<body>
    <div class="container">
        <h1>🔐 Dvarpala VPN Authentication</h1>
        <p>Please authenticate to gain full internet access.</p>
        <a href="/auth/google" class="auth-btn">Sign in with Google</a>
        <a href="/auth/microsoft" class="auth-btn">Sign in with Microsoft</a>
        <a href="/auth/github" class="auth-btn">Sign in with GitHub</a>
        <a href="/auth/gitlab" class="auth-btn">Sign in with GitLab</a>
        <p><small>After authentication, you will receive full VPN access until disconnect.</small></p>
    </div>
</body>
</html>`
        fmt.Fprint(w, html)
        
    case "auth-success.html":
        html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><title>Authentication Successful</title></head>
<body>
    <div style="text-align: center; padding: 50px;">
        <h1>✅ Authentication Successful!</h1>
        <p>Welcome %s! You now have full VPN access.</p>
        <p>You can access the internet normally.</p>
        <p><small>Authenticated via %s at %s</small></p>
    </div>
</body>
</html>`, data.Username, data.Provider, data.Timestamp)
        fmt.Fprint(w, html)
        
    case "auth-error.html":
        html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><title>Authentication Failed</title></head>
<body>
    <div style="text-align: center; padding: 50px;">
        <h1>❌ Authentication Failed</h1>
        <p>%s</p>
        <p><a href="/">Try Again</a></p>
    </div>
</body>
</html>`, data.Reason)
        fmt.Fprint(w, html)
    }
}

func main() {
    port := os.Getenv("SERVER_PORT")
    if port == "" {
        port = "8080"
    }
    
    // Load configuration and templates
    loadConfig()
    loadTemplates()
    
    // Serve static files
    http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("/opt/dvarpala/web/static/"))))
    
    // Main captive portal
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        data := PageData{
            Config:    config,
            Timestamp: time.Now().Format("2006-01-02 15:04:05"),
        }
        serveTemplate(w, "captive-portal.html", data)
    })
    
    // OAuth authentication endpoints
    http.HandleFunc("/auth/google", func(w http.ResponseWriter, r *http.Request) {
        handleOAuthCallback(w, r, "google")
    })
    
    http.HandleFunc("/auth/microsoft", func(w http.ResponseWriter, r *http.Request) {
        handleOAuthCallback(w, r, "microsoft")
    })
    
    http.HandleFunc("/auth/github", func(w http.ResponseWriter, r *http.Request) {
        handleOAuthCallback(w, r, "github")
    })
    
    http.HandleFunc("/auth/gitlab", func(w http.ResponseWriter, r *http.Request) {
        handleOAuthCallback(w, r, "gitlab")
    })
    
    // Success page
    http.HandleFunc("/auth/success", func(w http.ResponseWriter, r *http.Request) {
        data := PageData{
            Config:    config,
            Username:  "demo-user",
            Provider:  "Demo Provider",
            Timestamp: time.Now().Format("2006-01-02 15:04:05"),
        }
        serveTemplate(w, "auth-success.html", data)
    })
    
    // Error page
    http.HandleFunc("/auth/error", func(w http.ResponseWriter, r *http.Request) {
        reason := r.URL.Query().Get("reason")
        if reason == "" {
            reason = "Authentication failed. Please try again."
        }
        
        data := PageData{
            Config:    config,
            Reason:    reason,
            ErrorCode: "AUTH_FAILED",
            Timestamp: time.Now().Format("2006-01-02 15:04:05"),
        }
        serveTemplate(w, "auth-error.html", data)
    })
    
    // Internal APIs
    http.HandleFunc("/api/internal/check-auth/", func(w http.ResponseWriter, r *http.Request) {
        username := strings.TrimPrefix(r.URL.Path, "/api/internal/check-auth/")
        
        userMutex.RLock()
        expiresAt, exists := authenticatedUsers[username]
        userMutex.RUnlock()
        
        if exists && time.Now().Before(expiresAt) {
            fmt.Fprint(w, "authenticated")
        } else {
            if exists {
                userMutex.Lock()
                delete(authenticatedUsers, username)
                userMutex.Unlock()
            }
            fmt.Fprint(w, "unauthenticated")
        }
    })
    
    http.HandleFunc("/api/internal/clear-auth/", func(w http.ResponseWriter, r *http.Request) {
        if r.Method == "DELETE" {
            username := strings.TrimPrefix(r.URL.Path, "/api/internal/clear-auth/")
            userMutex.Lock()
            delete(authenticatedUsers, username)
            userMutex.Unlock()
        }
        fmt.Fprint(w, "OK")
    })
    
    http.HandleFunc("/api/internal/auth-status", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        
        // Demo: simulate authentication check
        response := map[string]interface{}{
            "authenticated": false,
            "user":          "",
            "expires_at":    "",
        }
        
        json.NewEncoder(w).Encode(response)
    })
    
    log.Printf("🔐 Dvarpala 2-Step VPN server starting on port %s", port)
    log.Printf("📁 Templates: /opt/dvarpala/web/templates/")
    log.Printf("📁 Static files: /opt/dvarpala/web/static/")
    log.Printf("🔧 Config: /opt/dvarpala/web/config/portal-config.json")
    log.Fatal(http.ListenAndServe(":"+port, nil))
}

func handleOAuthCallback(w http.ResponseWriter, r *http.Request, provider string) {
    // Simulate OAuth authentication
    username := fmt.Sprintf("demo-user@%s.com", provider)
    expiresAt := time.Now().Add(1 * time.Hour)
    
    userMutex.Lock()
    authenticatedUsers[username] = expiresAt
    userMutex.Unlock()
    
    log.Printf("✅ User %s authenticated via %s", username, provider)
    
    data := PageData{
        Config:    config,
        Username:  username,
        Provider:  provider,
        Timestamp: time.Now().Format("2006-01-02 15:04:05"),
    }
    
    serveTemplate(w, "auth-success.html", data)
}
EOF

    go build -o bin/dvarpala-server template-main.go
    
    # Install binaries
    cp -r bin/* "$DVARPALA_HOME/bin/"
    chown -R "$DVARPALA_USER:$DVARPALA_USER" "$DVARPALA_HOME/bin"
    chmod +x "$DVARPALA_HOME/bin"/*
    
    # Create web directories and copy template files
    mkdir -p "$DVARPALA_HOME/web/templates"
    mkdir -p "$DVARPALA_HOME/web/static/js"
    mkdir -p "$DVARPALA_HOME/web/static/css"
    mkdir -p "$DVARPALA_HOME/web/static/images"
    mkdir -p "$DVARPALA_HOME/web/config"
    
    # Copy templates if they exist in the project
    if [ -d "/tmp/dvarpala-templates" ]; then
        cp -r /tmp/dvarpala-templates/* "$DVARPALA_HOME/web/templates/"
    fi
    
    # Set proper ownership
    chown -R "$DVARPALA_USER:$DVARPALA_USER" "$DVARPALA_HOME/web"
    chmod -R 755 "$DVARPALA_HOME/web"
    
    log_success "Dvarpala 2-Step VPN application with templates installed"
}

# Configure OAuth providers
configure_oauth_providers() {
    log_info "Configuring OAuth authentication providers..."
    echo
    echo "=================================================================="
    echo -e "${BLUE}🔐 OAuth Provider Configuration${NC}"
    echo "=================================================================="
    echo
    echo "Dvarpala supports multiple OAuth providers for user authentication."
    echo "You can enable up to 2 providers for user login."
    echo
    echo "Available providers:"
    echo "1. Google/Gmail"
    echo "2. Microsoft"
    echo "3. GitHub"
    echo "4. GitLab"
    echo
    
    # Array to store selected providers
    declare -a SELECTED_PROVIDERS
    declare -A OAUTH_CONFIGS
    
    # Provider selection loop
    while true; do
        if [ ${#SELECTED_PROVIDERS[@]} -eq 0 ]; then
            echo "Please select your first OAuth provider:"
        elif [ ${#SELECTED_PROVIDERS[@]} -eq 1 ]; then
            echo "You can select one more provider (optional):"
            echo "5. Skip - Use only ${SELECTED_PROVIDERS[0]}"
        else
            break
        fi
        
        echo
        read -p "Enter your choice (1-5): " provider_choice
        
        case $provider_choice in
            1)
                if [[ ! " ${SELECTED_PROVIDERS[@]} " =~ " google " ]]; then
                    configure_google_oauth
                    if [ $? -eq 0 ]; then
                        SELECTED_PROVIDERS+=("google")
                        echo -e "${GREEN}✅${NC} Google OAuth configured"
                    fi
                else
                    echo -e "${YELLOW}⚠️${NC} Google OAuth already configured"
                fi
                ;;
            2)
                if [[ ! " ${SELECTED_PROVIDERS[@]} " =~ " microsoft " ]]; then
                    configure_microsoft_oauth
                    if [ $? -eq 0 ]; then
                        SELECTED_PROVIDERS+=("microsoft")
                        echo -e "${GREEN}✅${NC} Microsoft OAuth configured"
                    fi
                else
                    echo -e "${YELLOW}⚠️${NC} Microsoft OAuth already configured"
                fi
                ;;
            3)
                if [[ ! " ${SELECTED_PROVIDERS[@]} " =~ " github " ]]; then
                    configure_github_oauth
                    if [ $? -eq 0 ]; then
                        SELECTED_PROVIDERS+=("github")
                        echo -e "${GREEN}✅${NC} GitHub OAuth configured"
                    fi
                else
                    echo -e "${YELLOW}⚠️${NC} GitHub OAuth already configured"
                fi
                ;;
            4)
                if [[ ! " ${SELECTED_PROVIDERS[@]} " =~ " gitlab " ]]; then
                    configure_gitlab_oauth
                    if [ $? -eq 0 ]; then
                        SELECTED_PROVIDERS+=("gitlab")
                        echo -e "${GREEN}✅${NC} GitLab OAuth configured"
                    fi
                else
                    echo -e "${YELLOW}⚠️${NC} GitLab OAuth already configured"
                fi
                ;;
            5)
                if [ ${#SELECTED_PROVIDERS[@]} -eq 1 ]; then
                    echo -e "${BLUE}ℹ️${NC} Proceeding with ${SELECTED_PROVIDERS[0]} OAuth only"
                    break
                else
                    echo -e "${RED}❌${NC} Please select at least one OAuth provider first"
                fi
                ;;
            *)
                echo -e "${RED}❌${NC} Invalid choice. Please select 1-5."
                ;;
        esac
        
        echo
        if [ ${#SELECTED_PROVIDERS[@]} -eq 2 ]; then
            echo -e "${GREEN}✅${NC} Maximum providers configured: ${SELECTED_PROVIDERS[0]}, ${SELECTED_PROVIDERS[1]}"
            break
        fi
    done
    
    # Store OAuth configuration summary
    echo "# OAuth Providers Configured: ${SELECTED_PROVIDERS[*]}" > "$DVARPALA_HOME/config/.oauth_summary"
    echo "# Configured on: $(date)" >> "$DVARPALA_HOME/config/.oauth_summary"
    echo "ENABLED_PROVIDERS=\"${SELECTED_PROVIDERS[*]}\"" >> "$DVARPALA_HOME/config/.oauth_summary"
    
    chown "$DVARPALA_USER:$DVARPALA_USER" "$DVARPALA_HOME/config/.oauth_summary"
    chmod 600 "$DVARPALA_HOME/config/.oauth_summary"
    
    log_success "OAuth providers configured: ${SELECTED_PROVIDERS[*]}"
}

# Create OAuth providers database seeding script
create_oauth_seeding_script() {
    log_info "Creating OAuth provider database seeding script..."
    
    # Create a script to seed OAuth providers into database
    cat > "$DVARPALA_HOME/config/seed_oauth_providers.sql" << 'EOF'
-- Seed OAuth providers based on installation configuration
-- This script will be executed after database initialization

-- Clear existing OAuth providers (for clean setup)
DELETE FROM oauth_providers;

EOF

    # Add configured OAuth providers to the SQL script
    if [[ -f "$DVARPALA_HOME/config/.oauth_summary" ]]; then
        source "$DVARPALA_HOME/config/.oauth_summary"
        
        for provider in $ENABLED_PROVIDERS; do
            case $provider in
                "google")
                    cat >> "$DVARPALA_HOME/config/seed_oauth_providers.sql" << EOF
-- Google OAuth Provider
INSERT INTO oauth_providers (name, display_name, client_id, client_secret, auth_url, token_url, user_info_url, redirect_url, scopes, is_enabled, display_order, created_at, updated_at) 
VALUES (
    'google',
    'Google',
    '',  -- Will be populated from environment variables
    '',  -- Will be populated from environment variables
    'https://accounts.google.com/o/oauth2/auth',
    'https://oauth2.googleapis.com/token',
    'https://www.googleapis.com/oauth2/v2/userinfo',
    'http://$SERVER_IP:8080/auth/google/callback',
    'openid email profile',
    true,
    1,
    NOW(),
    NOW()
);

EOF
                    ;;
                "microsoft")
                    cat >> "$DVARPALA_HOME/config/seed_oauth_providers.sql" << EOF
-- Microsoft OAuth Provider
INSERT INTO oauth_providers (name, display_name, client_id, client_secret, auth_url, token_url, user_info_url, redirect_url, scopes, is_enabled, display_order, created_at, updated_at) 
VALUES (
    'microsoft',
    'Microsoft',
    '',  -- Will be populated from environment variables
    '',  -- Will be populated from environment variables
    'https://login.microsoftonline.com/common/oauth2/v2.0/authorize',
    'https://login.microsoftonline.com/common/oauth2/v2.0/token',
    'https://graph.microsoft.com/v1.0/me',
    'http://$SERVER_IP:8080/auth/microsoft/callback',
    'openid email profile',
    true,
    2,
    NOW(),
    NOW()
);

EOF
                    ;;
                "github")
                    cat >> "$DVARPALA_HOME/config/seed_oauth_providers.sql" << EOF
-- GitHub OAuth Provider
INSERT INTO oauth_providers (name, display_name, client_id, client_secret, auth_url, token_url, user_info_url, redirect_url, scopes, is_enabled, display_order, created_at, updated_at) 
VALUES (
    'github',
    'GitHub',
    '',  -- Will be populated from environment variables
    '',  -- Will be populated from environment variables
    'https://github.com/login/oauth/authorize',
    'https://github.com/login/oauth/access_token',
    'https://api.github.com/user',
    'http://$SERVER_IP:8080/auth/github/callback',
    'user:email',
    true,
    3,
    NOW(),
    NOW()
);

EOF
                    ;;
                "gitlab")
                    # Get GitLab URL from OAuth credentials
                    GITLAB_URL="https://gitlab.com"
                    if [[ -f "$DVARPALA_HOME/config/.oauth_credentials" ]]; then
                        source "$DVARPALA_HOME/config/.oauth_credentials"
                        if [[ -n "$OAUTH_GITLAB_URL" ]]; then
                            GITLAB_URL="$OAUTH_GITLAB_URL"
                        fi
                    fi
                    
                    cat >> "$DVARPALA_HOME/config/seed_oauth_providers.sql" << EOF
-- GitLab OAuth Provider
INSERT INTO oauth_providers (name, display_name, client_id, client_secret, auth_url, token_url, user_info_url, redirect_url, scopes, is_enabled, display_order, created_at, updated_at) 
VALUES (
    'gitlab',
    'GitLab',
    '',  -- Will be populated from environment variables
    '',  -- Will be populated from environment variables
    '$GITLAB_URL/oauth/authorize',
    '$GITLAB_URL/oauth/token',
    '$GITLAB_URL/api/v4/user',
    'http://$SERVER_IP:8080/auth/gitlab/callback',
    'read_user',
    true,
    4,
    NOW(),
    NOW()
);

EOF
                    ;;
            esac
        done
    fi
    
    chown "$DVARPALA_USER:$DVARPALA_USER" "$DVARPALA_HOME/config/seed_oauth_providers.sql"
    chmod 600 "$DVARPALA_HOME/config/seed_oauth_providers.sql"
    
    log_success "OAuth seeding script created"
}

# Create admin user database seeding script
create_admin_user_script() {
    log_info "Creating admin user database seeding script..."
    
    # Create SQL script to create admin user and assign to system_admins group
    cat > "$DVARPALA_HOME/config/seed_admin_user.sql" << EOF
-- Create admin user and assign to system_admins group
-- This script creates the first administrative user

-- Create admin user
INSERT INTO users (email, full_name, status, oauth_provider, created_at, updated_at)
VALUES (
    '$ADMIN_EMAIL',
    '$ADMIN_FULL_NAME',
    'active',
    '',  -- Will be set when user first logs in via OAuth
    NOW(),
    NOW()
) ON CONFLICT (email) DO NOTHING;

-- Get the admin user ID
WITH admin_user AS (
    SELECT id FROM users WHERE email = '$ADMIN_EMAIL'
),
system_admin_group AS (
    SELECT id FROM groups WHERE name = 'system_admins'
)
-- Add admin user to system_admins group
INSERT INTO user_groups (user_id, group_id, created_at, updated_at)
SELECT admin_user.id, system_admin_group.id, NOW(), NOW()
FROM admin_user, system_admin_group
ON CONFLICT (user_id, group_id) DO NOTHING;

-- Log the admin user creation
INSERT INTO audit_logs (user_id, action, resource_type, resource_id, ip_address, details, created_at)
SELECT 
    admin_user.id,
    'admin_user_created',
    'user',
    admin_user.id,
    '127.0.0.1',
    '{"created_during": "installation", "admin_email": "$ADMIN_EMAIL"}',
    NOW()
FROM (SELECT id FROM users WHERE email = '$ADMIN_EMAIL') admin_user;
EOF

    chown "$DVARPALA_USER:$DVARPALA_USER" "$DVARPALA_HOME/config/seed_admin_user.sql"
    chmod 600 "$DVARPALA_HOME/config/seed_admin_user.sql"
    
    log_success "Admin user seeding script created"
}

# Configure Google OAuth
configure_google_oauth() {
    echo -e "${BLUE}🔧 Google/Gmail OAuth Configuration${NC}"
    echo "To set up Google OAuth:"
    echo "1. Go to https://console.cloud.google.com/"
    echo "2. Create a new project or select existing"
    echo "3. Enable Google+ API or Google Identity API"
    echo "4. Go to Credentials > Create Credentials > OAuth 2.0 Client IDs"
    echo "5. Set Application type to 'Web application'"
    echo "6. Add Authorized redirect URI: http://$SERVER_IP:8080/auth/google/callback"
    echo
    
    read -p "Do you have Google OAuth credentials ready? (y/n): " ready
    if [[ ! $ready =~ ^[Yy]$ ]]; then
        echo "Please set up Google OAuth first and run the configuration again."
        return 1
    fi
    
    echo
    read -p "Google Client ID: " GOOGLE_CLIENT_ID
    read -p "Google Client Secret: " GOOGLE_CLIENT_SECRET
    
    if [[ -z "$GOOGLE_CLIENT_ID" || -z "$GOOGLE_CLIENT_SECRET" ]]; then
        echo -e "${RED}❌${NC} Client ID and Secret are required"
        return 1
    fi
    
    # Store Google OAuth config
    cat >> "$DVARPALA_HOME/config/.oauth_credentials" << EOF
# Google OAuth Configuration
OAUTH_GOOGLE_CLIENT_ID=$GOOGLE_CLIENT_ID
OAUTH_GOOGLE_CLIENT_SECRET=$GOOGLE_CLIENT_SECRET
OAUTH_GOOGLE_REDIRECT_URL=http://$SERVER_IP:8080/auth/google/callback

EOF
    
    return 0
}

# Configure Microsoft OAuth
configure_microsoft_oauth() {
    echo -e "${BLUE}🔧 Microsoft OAuth Configuration${NC}"
    echo "To set up Microsoft OAuth:"
    echo "1. Go to https://portal.azure.com/"
    echo "2. Navigate to Azure Active Directory > App registrations"
    echo "3. Click 'New registration'"
    echo "4. Set Redirect URI: http://$SERVER_IP:8080/auth/microsoft/callback"
    echo "5. Go to Certificates & secrets > New client secret"
    echo
    
    read -p "Do you have Microsoft OAuth credentials ready? (y/n): " ready
    if [[ ! $ready =~ ^[Yy]$ ]]; then
        echo "Please set up Microsoft OAuth first and run the configuration again."
        return 1
    fi
    
    echo
    read -p "Microsoft Application (client) ID: " MICROSOFT_CLIENT_ID
    read -p "Microsoft Client Secret: " MICROSOFT_CLIENT_SECRET
    
    if [[ -z "$MICROSOFT_CLIENT_ID" || -z "$MICROSOFT_CLIENT_SECRET" ]]; then
        echo -e "${RED}❌${NC} Client ID and Secret are required"
        return 1
    fi
    
    # Store Microsoft OAuth config
    cat >> "$DVARPALA_HOME/config/.oauth_credentials" << EOF
# Microsoft OAuth Configuration
OAUTH_MICROSOFT_CLIENT_ID=$MICROSOFT_CLIENT_ID
OAUTH_MICROSOFT_CLIENT_SECRET=$MICROSOFT_CLIENT_SECRET
OAUTH_MICROSOFT_REDIRECT_URL=http://$SERVER_IP:8080/auth/microsoft/callback

EOF
    
    return 0
}

# Configure GitHub OAuth
configure_github_oauth() {
    echo -e "${BLUE}🔧 GitHub OAuth Configuration${NC}"
    echo "To set up GitHub OAuth:"
    echo "1. Go to https://github.com/settings/applications/new"
    echo "2. Set Application name: 'Dvarpala VPN'"
    echo "3. Set Authorization callback URL: http://$SERVER_IP:8080/auth/github/callback"
    echo "4. Create the application to get Client ID and Secret"
    echo
    
    read -p "Do you have GitHub OAuth credentials ready? (y/n): " ready
    if [[ ! $ready =~ ^[Yy]$ ]]; then
        echo "Please set up GitHub OAuth first and run the configuration again."
        return 1
    fi
    
    echo
    read -p "GitHub Client ID: " GITHUB_CLIENT_ID
    read -p "GitHub Client Secret: " GITHUB_CLIENT_SECRET
    
    if [[ -z "$GITHUB_CLIENT_ID" || -z "$GITHUB_CLIENT_SECRET" ]]; then
        echo -e "${RED}❌${NC} Client ID and Secret are required"
        return 1
    fi
    
    # Store GitHub OAuth config
    cat >> "$DVARPALA_HOME/config/.oauth_credentials" << EOF
# GitHub OAuth Configuration
OAUTH_GITHUB_CLIENT_ID=$GITHUB_CLIENT_ID
OAUTH_GITHUB_CLIENT_SECRET=$GITHUB_CLIENT_SECRET
OAUTH_GITHUB_REDIRECT_URL=http://$SERVER_IP:8080/auth/github/callback

EOF
    
    return 0
}

# Configure GitLab OAuth
configure_gitlab_oauth() {
    echo -e "${BLUE}🔧 GitLab OAuth Configuration${NC}"
    echo "To set up GitLab OAuth:"
    echo "1. Go to https://gitlab.com/-/profile/applications (or your GitLab instance)"
    echo "2. Set Name: 'Dvarpala VPN'"
    echo "3. Set Redirect URI: http://$SERVER_IP:8080/auth/gitlab/callback"
    echo "4. Select scopes: 'read_user' (minimum required)"
    echo "5. Create application to get Application ID and Secret"
    echo
    
    read -p "Do you have GitLab OAuth credentials ready? (y/n): " ready
    if [[ ! $ready =~ ^[Yy]$ ]]; then
        echo "Please set up GitLab OAuth first and run the configuration again."
        return 1
    fi
    
    echo
    read -p "GitLab Application ID: " GITLAB_CLIENT_ID
    read -p "GitLab Secret: " GITLAB_CLIENT_SECRET
    read -p "GitLab URL (default: https://gitlab.com): " GITLAB_URL
    
    # Set default GitLab URL if not provided
    if [[ -z "$GITLAB_URL" ]]; then
        GITLAB_URL="https://gitlab.com"
    fi
    
    if [[ -z "$GITLAB_CLIENT_ID" || -z "$GITLAB_CLIENT_SECRET" ]]; then
        echo -e "${RED}❌${NC} Application ID and Secret are required"
        return 1
    fi
    
    # Store GitLab OAuth config
    cat >> "$DVARPALA_HOME/config/.oauth_credentials" << EOF
# GitLab OAuth Configuration
OAUTH_GITLAB_CLIENT_ID=$GITLAB_CLIENT_ID
OAUTH_GITLAB_CLIENT_SECRET=$GITLAB_CLIENT_SECRET
OAUTH_GITLAB_URL=$GITLAB_URL
OAUTH_GITLAB_REDIRECT_URL=http://$SERVER_IP:8080/auth/gitlab/callback

EOF
    
    return 0
}

# Create environment configuration
create_environment_config() {
    log_info "Creating environment configuration..."
    
    # Source database credentials
    source "$DVARPALA_HOME/config/.db_credentials"
    
    # Generate JWT secret
    JWT_SECRET=$(openssl rand -base64 64)
    
    # Initialize OAuth credentials file
    touch "$DVARPALA_HOME/config/.oauth_credentials"
    chown "$DVARPALA_USER:$DVARPALA_USER" "$DVARPALA_HOME/config/.oauth_credentials"
    chmod 600 "$DVARPALA_HOME/config/.oauth_credentials"
    
    # Configure OAuth providers
    configure_oauth_providers
    
    # Create OAuth seeding script
    create_oauth_seeding_script
    
    # Create admin user seeding script
    create_admin_user_script
    
    # Create .env file
    cat > "$DVARPALA_HOME/config/.env" << EOF
# Dvarpala VPN Server Configuration

# Server Configuration
SERVER_PORT=$DVARPALA_WEB_PORT
SERVER_MODE=production
SERVER_READ_TIMEOUT=30s
SERVER_WRITE_TIMEOUT=30s

# Database Configuration
DB_HOST=$DB_HOST
DB_PORT=$DB_PORT
DB_NAME=$DB_NAME
DB_USER=$DB_USER
DB_PASSWORD=$DB_PASSWORD
DB_SSL_MODE=disable
DB_MAX_OPEN_CONNS=25
DB_MAX_IDLE_CONNS=5

# Redis Configuration
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=
REDIS_DB=0
REDIS_POOL_SIZE=10

# Authentication Configuration
AUTH_SESSION_DURATION=28800
AUTH_CAPTIVE_PORTAL_TIMEOUT=1800
AUTH_JWT_SECRET=$JWT_SECRET
AUTH_ALLOWED_DOMAINS=yourcompany.com

EOF

    # Append OAuth credentials from the collected configuration
    if [[ -f "$DVARPALA_HOME/config/.oauth_credentials" ]]; then
        echo "" >> "$DVARPALA_HOME/config/.env"
        echo "# OAuth Configuration" >> "$DVARPALA_HOME/config/.env"
        cat "$DVARPALA_HOME/config/.oauth_credentials" >> "$DVARPALA_HOME/config/.env"
    else
        # Add placeholder OAuth configuration if none configured
        cat >> "$DVARPALA_HOME/config/.env" << 'EOF'

# OAuth Configuration (Not configured during installation)
OAUTH_GOOGLE_CLIENT_ID=
OAUTH_GOOGLE_CLIENT_SECRET=
OAUTH_GOOGLE_REDIRECT_URL=

OAUTH_MICROSOFT_CLIENT_ID=
OAUTH_MICROSOFT_CLIENT_SECRET=
OAUTH_MICROSOFT_REDIRECT_URL=

OAUTH_GITHUB_CLIENT_ID=
OAUTH_GITHUB_CLIENT_SECRET=
OAUTH_GITHUB_REDIRECT_URL=

OAUTH_GITLAB_CLIENT_ID=
OAUTH_GITLAB_CLIENT_SECRET=
OAUTH_GITLAB_URL=
OAUTH_GITLAB_REDIRECT_URL=
EOF
    fi
    
    # Continue with the rest of the configuration
    cat >> "$DVARPALA_HOME/config/.env" << EOF

# OpenVPN Configuration
OPENVPN_MANAGEMENT_HOST=localhost
OPENVPN_MANAGEMENT_PORT=7505
OPENVPN_CAPTIVE_PORTAL_NETWORK=192.168.100.0/24
OPENVPN_FULL_ACCESS_NETWORK=10.8.0.0/24

# Security Configuration
SECURITY_FAILED_LOGIN_THRESHOLD=5
SECURITY_IP_BLOCK_DURATION=1800
SECURITY_BCRYPT_COST=12

# Logging Configuration
LOG_LEVEL=info
LOG_FORMAT=json
LOG_OUTPUT=stdout
EOF

    chown "$DVARPALA_USER:$DVARPALA_USER" "$DVARPALA_HOME/config/.env"
    chmod 600 "$DVARPALA_HOME/config/.env"
    
    log_success "Environment configuration created"
}

# Create systemd services
create_systemd_services() {
    log_info "Creating systemd services..."
    
    # OpenVPN service
    cat > /etc/systemd/system/openvpn-server.service << EOF
[Unit]
Description=OpenVPN Server for Dvarpala
After=network.target
Wants=network.target

[Service]
Type=notify
PrivateTmp=true
WorkingDirectory=/etc/openvpn
ExecStart=/usr/sbin/openvpn --status /run/openvpn/server.status 10 --cd /etc/openvpn --config server.conf --writepid /run/openvpn/server.pid
ExecReload=/bin/kill -HUP \$MAINPID
CapabilityBoundingSet=CAP_IPC_LOCK CAP_NET_ADMIN CAP_NET_BIND_SERVICE CAP_NET_RAW CAP_SETGID CAP_SETUID CAP_SYS_CHROOT CAP_DAC_OVERRIDE CAP_AUDIT_WRITE
LimitNPROC=10
DeviceAllow=/dev/null rw
DeviceAllow=/dev/net/tun rw
ProtectSystem=true
ProtectHome=true
KillMode=process

[Install]
WantedBy=multi-user.target
EOF

    # Dvarpala service
    cat > /etc/systemd/system/dvarpala.service << EOF
[Unit]
Description=Dvarpala VPN Management Server
After=network.target postgresql.service redis.service
Wants=network.target
Requires=postgresql.service redis.service

[Service]
Type=simple
User=$DVARPALA_USER
Group=$DVARPALA_USER
WorkingDirectory=$DVARPALA_HOME
ExecStart=$DVARPALA_HOME/bin/dvarpala-server
EnvironmentFile=$DVARPALA_HOME/config/.env
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal
SyslogIdentifier=dvarpala

[Install]
WantedBy=multi-user.target
EOF

    # Reload systemd and enable services
    systemctl daemon-reload
    systemctl enable postgresql redis openvpn-server dvarpala
    
    log_success "Systemd services created and enabled"
}

# Generate admin OVPN file
generate_admin_ovpn() {
    log_info "Generating admin .ovpn file..."
    
    # Create admin client configuration
    cat > "$DVARPALA_HOME/certs/admin.ovpn" << EOF
client
dev tun
proto udp
remote $SERVER_IP $OPENVPN_PORT
resolv-retry infinite
nobind
user nobody
group nogroup
persist-key
persist-tun
cipher AES-256-GCM
auth SHA256
verb 3

# Embedded certificates and keys
<ca>
$(cat /etc/openvpn/easy-rsa/pki/ca.crt)
</ca>

<cert>
$(cat /etc/openvpn/easy-rsa/pki/issued/admin.crt)
</cert>

<key>
$(cat /etc/openvpn/easy-rsa/pki/private/admin.key)
</key>

<tls-auth>
$(cat /etc/openvpn/easy-rsa/pki/ta.key)
</tls-auth>
key-direction 1

# Authentication
auth-user-pass
EOF

    chown "$DVARPALA_USER:$DVARPALA_USER" "$DVARPALA_HOME/certs/admin.ovpn"
    chmod 600 "$DVARPALA_HOME/certs/admin.ovpn"
    
    log_success "Admin .ovpn file generated: $DVARPALA_HOME/certs/admin.ovpn"
}

# Display admin credentials and VPN config
display_admin_credentials() {
    echo
    echo "=================================================================="
    echo -e "${YELLOW}🔐 IMPORTANT: SAVE THESE CREDENTIALS NOW!${NC}"
    echo "=================================================================="
    echo
    echo -e "${RED}⚠️  CRITICAL: Copy these credentials immediately!${NC}"
    echo -e "${RED}⚠️  SSH access will be disabled after you continue!${NC}"
    echo
    
    echo -e "${BLUE}📧 Database Credentials:${NC}"
    echo "Database: dvarpala"
    echo "Username: dvarpala"
    echo "Password: $POSTGRES_PASSWORD"
    echo
    
    echo -e "${BLUE}🔑 VPN Temporary Credentials:${NC}"
    echo "Username: temp_user"
    echo "Password: temp_portal_access"
    echo
    
    echo -e "${BLUE}📁 Admin OpenVPN Configuration:${NC}"
    echo "Copy the entire content below and save it as 'dvarpala-admin.ovpn':"
    echo
    echo "=================================================================="
    echo -e "${GREEN}"
    cat "$DVARPALA_HOME/certs/admin.ovpn"
    echo -e "${NC}"
    echo "=================================================================="
    echo
    
    echo -e "${YELLOW}📋 REQUIRED ACTIONS:${NC}"
    echo "1. Copy the .ovpn configuration above to your local machine"
    echo "2. Save it as 'dvarpala-admin.ovpn'"
    echo "3. Import it into your OpenVPN client"
    echo "4. Connect using the temporary credentials shown above"
    echo "5. Complete OAuth setup via web interface: http://$SERVER_IP:8080"
    echo
    
    # Create a backup copy that can be retrieved later if needed
    cp "$DVARPALA_HOME/certs/admin.ovpn" "$DVARPALA_HOME/certs/admin-backup.ovpn"
    chmod 600 "$DVARPALA_HOME/certs/admin-backup.ovpn"
    
    echo -e "${BLUE}💾 Backup saved at: $DVARPALA_HOME/certs/admin-backup.ovpn${NC}"
    echo
}

# Wait for user confirmation before securing the server
wait_for_user_confirmation() {
    echo -e "${RED}⚠️  WARNING: SECURITY LOCKDOWN${NC}"
    echo "=================================================================="
    echo
    echo "The next step will:"
    echo "• Block SSH access from the internet"
    echo "• Only allow SSH via VPN connection"
    echo "• Configure firewall for VPN-only access"
    echo
    echo -e "${YELLOW}Make sure you have:${NC}"
    echo "✓ Copied the .ovpn configuration above"
    echo "✓ Saved the database credentials"
    echo "✓ Tested VPN connection (recommended)"
    echo
    echo -e "${RED}After this step, you can ONLY access this server via VPN!${NC}"
    echo
    
    while true; do
        read -p "Have you copied all credentials and are ready to secure the server? (yes/no): " -r
        case $REPLY in
            [Yy][Ee][Ss])
                log_info "Proceeding with server security lockdown..."
                break
                ;;
            [Nn][Oo])
                echo
                echo "Please copy the credentials above and type 'yes' when ready."
                echo "The .ovpn configuration is also saved at: $DVARPALA_HOME/certs/admin.ovpn"
                echo
                ;;
            *)
                echo "Please answer 'yes' or 'no'"
                ;;
        esac
    done
}

# Secure the server by blocking external SSH
secure_server_access() {
    log_info "Securing server access - blocking external SSH..."
    
    # Backup current SSH config
    cp /etc/ssh/sshd_config /etc/ssh/sshd_config.backup
    
    # Create new SSH config that only allows VPN access
    cat >> /etc/ssh/sshd_config << EOF

# Dvarpala VPN Security Configuration
# Only allow SSH from VPN network
Match Address 10.8.0.0/24
    AllowUsers root dvarpala

# Block all other SSH access
Match Address *,!10.8.0.0/24
    DenyUsers *
EOF

    # Update firewall to block SSH from internet
    case $OS in
        ubuntu|debian)
            # Remove general SSH rule and add VPN-only SSH rule
            ufw --force delete allow ssh 2>/dev/null || true
            ufw allow from 10.8.0.0/24 to any port 22
            ufw deny 22
            ;;
        centos|rhel|rocky|almalinux)
            # Remove SSH service and add specific rule
            firewall-cmd --permanent --remove-service=ssh 2>/dev/null || true
            firewall-cmd --permanent --add-rich-rule="rule family='ipv4' source address='10.8.0.0/24' port protocol='tcp' port='22' accept"
            firewall-cmd --permanent --add-rich-rule="rule family='ipv4' port protocol='tcp' port='22' reject"
            firewall-cmd --reload
            ;;
    esac
    
    # Restart SSH service
    systemctl restart sshd
    
    log_success "Server secured - SSH now only accessible via VPN (10.8.0.0/24)"
    log_warning "External SSH access has been blocked!"
}

# Initialize database with schema and seed data
initialize_database() {
    log_info "Initializing Dvarpala database..."
    
    # Source database credentials
    source "$DVARPALA_HOME/config/.db_credentials"
    
    # Wait for PostgreSQL to be ready
    until sudo -u postgres psql -c '\q' 2>/dev/null; do
        log_info "Waiting for PostgreSQL to be ready..."
        sleep 2
    done
    
    # Create database schema (for now, we'll create basic tables)
    # In production, this would run GORM auto-migration or SQL schema files
    log_info "Creating database schema..."
    
    sudo -u postgres psql -d dvarpala << 'EOF'
-- Users table
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    full_name VARCHAR(255),
    status VARCHAR(50) DEFAULT 'pending',
    oauth_provider VARCHAR(50),
    oauth_id VARCHAR(255),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Groups table
CREATE TABLE IF NOT EXISTS groups (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) UNIQUE NOT NULL,
    description TEXT,
    group_type VARCHAR(50) DEFAULT 'user',
    metadata JSONB,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- User groups junction table
CREATE TABLE IF NOT EXISTS user_groups (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    group_id INTEGER REFERENCES groups(id) ON DELETE CASCADE,
    role VARCHAR(50) DEFAULT 'member',
    metadata JSONB,
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(user_id, group_id)
);

-- OAuth providers table
CREATE TABLE IF NOT EXISTS oauth_providers (
    id SERIAL PRIMARY KEY,
    name VARCHAR(50) UNIQUE NOT NULL,
    display_name VARCHAR(100),
    client_id VARCHAR(255),
    client_secret VARCHAR(500),
    auth_url VARCHAR(500),
    token_url VARCHAR(500),
    user_info_url VARCHAR(500),
    redirect_url VARCHAR(500),
    scopes VARCHAR(255),
    is_enabled BOOLEAN DEFAULT true,
    display_order INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Create system_admins group
INSERT INTO groups (name, description, group_type, metadata) 
VALUES (
    'system_admins',
    'System administrators with full access to Dvarpala',
    'system',
    '{"permissions": ["admin", "user_management", "system_config"]}'
) ON CONFLICT (name) DO NOTHING;

EOF

    if [ $? -eq 0 ]; then
        log_success "Database schema created"
    else
        log_error "Failed to create database schema"
        return 1
    fi
    
    # Execute OAuth provider seeding
    if [ -f "$DVARPALA_HOME/config/seed_oauth_providers.sql" ]; then
        log_info "Seeding OAuth providers..."
        sudo -u postgres psql -d dvarpala -f "$DVARPALA_HOME/config/seed_oauth_providers.sql"
        if [ $? -eq 0 ]; then
            log_success "OAuth providers seeded"
        else
            log_warning "Failed to seed OAuth providers"
        fi
    fi
    
    # Execute admin user seeding
    if [ -f "$DVARPALA_HOME/config/seed_admin_user.sql" ]; then
        log_info "Creating admin user..."
        sudo -u postgres psql -d dvarpala -f "$DVARPALA_HOME/config/seed_admin_user.sql"
        if [ $? -eq 0 ]; then
            log_success "Admin user created: $ADMIN_EMAIL"
        else
            log_warning "Failed to create admin user"
        fi
    fi
    
    log_success "Database initialization completed"
}

# Start all services
start_services() {
    log_info "Starting all services..."
    
    systemctl start postgresql
    systemctl start redis
    systemctl start openvpn-server
    systemctl start dvarpala
    
    # Wait a moment for services to start
    sleep 5
    
    # Check service status
    if systemctl is-active --quiet postgresql; then
        log_success "PostgreSQL is running"
    else
        log_error "PostgreSQL failed to start"
    fi
    
    if systemctl is-active --quiet redis; then
        log_success "Redis is running"
    else
        log_error "Redis failed to start"
    fi
    
    if systemctl is-active --quiet openvpn-server; then
        log_success "OpenVPN server is running"
    else
        log_error "OpenVPN server failed to start"
    fi
    
    if systemctl is-active --quiet dvarpala; then
        log_success "Dvarpala application is running"
    else
        log_warning "Dvarpala application may need configuration"
    fi
}

# Print final information
print_final_info() {
    echo
    echo "=================================================================="
    echo -e "${GREEN}✅ Dvarpala VPN Server Setup Complete & Secured!${NC}"
    echo "=================================================================="
    echo
    echo -e "${BLUE}🔒 Security Status:${NC}"
    echo "   ✅ Server is now secured"
    echo "   ✅ SSH only accessible via VPN (10.8.0.0/24)"
    echo "   ✅ External SSH access blocked"
    echo "   ✅ Firewall configured for VPN-only access"
    echo
    echo -e "${BLUE}🌐 Connection Information:${NC}"
    echo "   VPN Server: $SERVER_IP:$OPENVPN_PORT (UDP)"
    echo "   Web Interface: http://$SERVER_IP:$DVARPALA_WEB_PORT (via VPN only)"
    echo
    echo -e "${BLUE}🔑 Access Method:${NC}"
    echo "   1. Connect via OpenVPN using the admin.ovpn file you copied"
    echo "   2. Use temporary credentials: temp_user / temp_portal_access"
    echo "   3. Once connected, you can SSH to: ssh root@10.8.0.X"
    echo "   4. Access web interface: http://$SERVER_IP:$DVARPALA_WEB_PORT"
    echo
    echo -e "${BLUE}📋 Next Steps:${NC}"
    echo "   1. Test VPN connection with the .ovpn file you copied"
    echo "   2. SSH into server via VPN: ssh root@10.8.0.1"
    echo "   3. Configure OAuth providers: sudo /opt/dvarpala/scripts/configure-oauth.sh"
    echo "   4. Access web interface and complete setup"
    echo
    echo -e "${YELLOW}⚠️  IMPORTANT SECURITY NOTES:${NC}"
    echo "   • This server is now only accessible via VPN"
    echo "   • If you lose the .ovpn file, you'll need console access to recover"
    echo "   • Backup file available at: $DVARPALA_HOME/certs/admin-backup.ovpn"
    echo "   • SSH config backup at: /etc/ssh/sshd_config.backup"
    echo
    echo -e "${BLUE}🆘 Emergency Access:${NC}"
    echo "   If you lose VPN access, use your cloud provider's console to:"
    echo "   1. Access the server via console/serial connection"
    echo "   2. Restore SSH: sudo cp /etc/ssh/sshd_config.backup /etc/ssh/sshd_config"
    echo "   3. Restart SSH: sudo systemctl restart sshd"
    echo "   4. Re-retrieve the .ovpn file from $DVARPALA_HOME/certs/admin-backup.ovpn"
    echo
    echo "=================================================================="
    echo -e "${GREEN}🎉 Enjoy your secure Dvarpala VPN server!${NC}"
    echo "=================================================================="
}

# Main execution
main() {
    echo "=================================================================="
    echo "🚀 Dvarpala VPN Server Provisioning Script"
    echo "=================================================================="
    echo
    
    check_root
    collect_admin_information
    detect_os
    update_system
    install_essentials
    create_dvarpala_user
    install_postgresql
    configure_postgresql
    install_redis
    install_go
    install_openvpn
    configure_openvpn
    create_temp_cert_scripts
    install_dvarpala_app
    create_environment_config
    create_systemd_services
    generate_admin_ovpn
    start_services
    initialize_database
    
    # Security phase - display credentials and secure server
    display_admin_credentials
    wait_for_user_confirmation
    secure_server_access
    print_final_info
}

# Run main function
main "$@"