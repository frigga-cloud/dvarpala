#!/bin/bash

# Enhanced Dvarpala Server Setup Script for Cloud Deployment
# This script is called automatically by the cloud installer
# Usage: curl -fsSL https://raw.githubusercontent.com/frigga-cloud/dvarpala/main/scripts/installation/cloud-setup-server.sh | bash

set -euo pipefail

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Configuration
DVARPALA_USER="dvarpala"
DVARPALA_DIR="/opt/dvarpala"
CONFIG_DIR="$DVARPALA_DIR/config"
LOG_FILE="/var/log/dvarpala-installation.log"

echo "=================================================================="
echo -e "${BLUE}🚀 Dvarpala VPN Server Cloud Setup${NC}"
echo "=================================================================="
echo

# Logging function
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" | tee -a "$LOG_FILE"
}

# Check if running as root
if [[ $EUID -ne 0 ]]; then
    echo -e "${RED}[ERROR]${NC} This script must be run as root"
    exit 1
fi

# Create dvarpala user
create_dvarpala_user() {
    log "Creating dvarpala system user..."
    if ! id "$DVARPALA_USER" &>/dev/null; then
        useradd --system --home-dir "$DVARPALA_DIR" --shell /bin/bash "$DVARPALA_USER"
        log "Created dvarpala user"
    else
        log "Dvarpala user already exists"
    fi
}

# Create directory structure
create_directories() {
    log "Creating directory structure..."
    mkdir -p "$DVARPALA_DIR"/{bin,config,logs,certs,scripts}
    mkdir -p "$CONFIG_DIR"
    mkdir -p /etc/openvpn/{server,client-configs}
    mkdir -p /var/log/dvarpala
    
    chown -R "$DVARPALA_USER:$DVARPALA_USER" "$DVARPALA_DIR"
    chown -R "$DVARPALA_USER:$DVARPALA_USER" /var/log/dvarpala
}

# Detect OS and install dependencies
install_dependencies() {
    log "Detecting OS and installing dependencies..."
    
    if [[ -f /etc/debian_version ]]; then
        # Debian/Ubuntu
        export DEBIAN_FRONTEND=noninteractive
        apt-get update -y
        apt-get upgrade -y
        
        apt-get install -y \
            curl wget unzip git \
            postgresql postgresql-contrib \
            redis-server \
            openvpn easy-rsa \
            ufw \
            nginx \
            certbot python3-certbot-nginx \
            fail2ban \
            htop iotop \
            jq
            
    elif [[ -f /etc/redhat-release ]]; then
        # RHEL/CentOS/Rocky/AlmaLinux
        yum update -y
        yum install -y epel-release
        
        yum install -y \
            curl wget unzip git \
            postgresql-server postgresql-contrib \
            redis \
            openvpn easy-rsa \
            firewalld \
            nginx \
            certbot python3-certbot-nginx \
            fail2ban \
            htop iotop \
            jq
            
        # Initialize PostgreSQL
        postgresql-setup initdb
        
    else
        log "Unsupported operating system"
        exit 1
    fi
    
    log "Dependencies installed successfully"
}

# Install Go
install_go() {
    log "Installing Go..."
    
    GO_VERSION="1.21.5"
    GO_ARCH="amd64"
    
    if command -v go &> /dev/null; then
        CURRENT_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
        if [[ "$CURRENT_VERSION" == "$GO_VERSION" ]]; then
            log "Go $GO_VERSION already installed"
            return
        fi
    fi
    
    cd /tmp
    wget -q "https://golang.org/dl/go${GO_VERSION}.linux-${GO_ARCH}.tar.gz"
    rm -rf /usr/local/go
    tar -C /usr/local -xzf "go${GO_VERSION}.linux-${GO_ARCH}.tar.gz"
    
    # Add Go to PATH for all users
    echo 'export PATH=/usr/local/go/bin:$PATH' > /etc/profile.d/go.sh
    chmod +x /etc/profile.d/go.sh
    
    # Set PATH for current session
    export PATH=/usr/local/go/bin:$PATH
    
    log "Go $GO_VERSION installed successfully"
}

# Download and build dvarpala
build_dvarpala() {
    log "Downloading and building dvarpala..."
    
    cd /tmp
    if [[ ! -d "dvarpala" ]]; then
        git clone https://github.com/frigga-cloud/dvarpala.git
    fi
    
    cd dvarpala
    export PATH=/usr/local/go/bin:$PATH
    
    # Build binaries
    go mod download
    go build -o "$DVARPALA_DIR/bin/dvarpala-server" ./cmd/dvarpala-server
    go build -o "$DVARPALA_DIR/bin/dvarpala-cli" ./cmd/dvarpala-cli
    go build -o "$DVARPALA_DIR/bin/dvarpala-worker" ./cmd/dvarpala-worker
    go build -o "$DVARPALA_DIR/bin/openvpn-auth" ./cmd/openvpn-auth
    
    # Copy configuration files
    cp -r configs/* "$CONFIG_DIR/"
    cp -r scripts/* "$DVARPALA_DIR/scripts/"
    
    # Set permissions
    chmod +x "$DVARPALA_DIR/bin/"*
    chmod +x "$DVARPALA_DIR/scripts/"*.sh
    chown -R "$DVARPALA_USER:$DVARPALA_USER" "$DVARPALA_DIR"
    
    log "Dvarpala built and installed successfully"
}

# Configure PostgreSQL
configure_postgresql() {
    log "Configuring PostgreSQL..."
    
    # Generate secure password
    DB_PASSWORD=$(openssl rand -base64 32)
    echo "$DB_PASSWORD" > "$CONFIG_DIR/.db_password"
    chmod 600 "$CONFIG_DIR/.db_password"
    
    # Start PostgreSQL
    systemctl enable postgresql
    systemctl start postgresql
    
    # Create database and user
    sudo -u postgres psql <<EOF
CREATE USER dvarpala WITH PASSWORD '$DB_PASSWORD';
CREATE DATABASE dvarpala OWNER dvarpala;
GRANT ALL PRIVILEGES ON DATABASE dvarpala TO dvarpala;
\q
EOF

    # Update PostgreSQL configuration for local connections
    PG_VERSION=$(sudo -u postgres psql -t -c "SELECT version();" | grep -o '[0-9]\+\.[0-9]\+' | head -1)
    PG_CONFIG_DIR="/etc/postgresql/$PG_VERSION/main"
    
    if [[ -d "$PG_CONFIG_DIR" ]]; then
        # Ubuntu/Debian path
        sed -i "s/#listen_addresses = 'localhost'/listen_addresses = 'localhost'/" "$PG_CONFIG_DIR/postgresql.conf"
        echo "host    dvarpala        dvarpala        127.0.0.1/32            md5" >> "$PG_CONFIG_DIR/pg_hba.conf"
    else
        # RHEL/CentOS path
        PG_DATA_DIR="/var/lib/pgsql/data"
        sed -i "s/#listen_addresses = 'localhost'/listen_addresses = 'localhost'/" "$PG_DATA_DIR/postgresql.conf"
        echo "host    dvarpala        dvarpala        127.0.0.1/32            md5" >> "$PG_DATA_DIR/pg_hba.conf"
    fi
    
    systemctl restart postgresql
    
    log "PostgreSQL configured successfully"
}

# Configure Redis
configure_redis() {
    log "Configuring Redis..."
    
    systemctl enable redis
    systemctl start redis
    
    # Basic Redis security
    redis-cli CONFIG SET requirepass "$(openssl rand -base64 32)"
    
    log "Redis configured successfully"
}

# Configure OpenVPN
configure_openvpn() {
    log "Configuring OpenVPN..."
    
    # Initialize Easy-RSA
    cd /etc/openvpn
    make-cadir easy-rsa
    cd easy-rsa
    
    # Set up CA variables
    cat > vars <<EOF
export KEY_COUNTRY="US"
export KEY_PROVINCE="CA"
export KEY_CITY="SanFrancisco"
export KEY_ORG="Frigga-Labs"
export KEY_EMAIL="admin@frigga-labs.com"
export KEY_OU="VPN"
export KEY_NAME="server"
EOF

    source ./vars
    ./clean-all
    
    # Build CA
    echo -e "\n\n\n\n\n\n\n" | ./build-ca
    
    # Build server certificate
    echo -e "\n\n\n\n\n\n\n\ny\ny\n" | ./build-key-server server
    
    # Build Diffie-Hellman parameters
    ./build-dh
    
    # Generate TLS auth key
    openvpn --genkey --secret keys/ta.key
    
    # Copy certificates to OpenVPN directory
    cp keys/{ca.crt,server.crt,server.key,dh2048.pem,ta.key} /etc/openvpn/server/
    
    # Create OpenVPN server configuration
    cat > /etc/openvpn/server/server.conf <<EOF
port 1194
proto udp
dev tun

ca ca.crt
cert server.crt
key server.key
dh dh2048.pem

# Network configuration
server 172.30.100.0 255.255.255.0
ifconfig-pool-persist ipp.txt

# Full access network (after authentication)
push "route 172.30.8.0 255.255.248.0"

# Client configuration
client-config-dir /etc/openvpn/ccd
keepalive 10 120
tls-auth ta.key 0
cipher AES-256-GCM
user nobody
group nogroup
persist-key
persist-tun

# Logging
status /var/log/openvpn/openvpn-status.log
log-append /var/log/openvpn/openvpn.log
verb 3

# Authentication
script-security 3
auth-user-pass-verify /opt/dvarpala/bin/openvpn-auth via-env
client-connect /opt/dvarpala/scripts/client-connect.sh
client-disconnect /opt/dvarpala/scripts/client-disconnect.sh
EOF

    mkdir -p /etc/openvpn/ccd
    mkdir -p /var/log/openvpn
    
    # Enable IP forwarding
    echo 'net.ipv4.ip_forward=1' >> /etc/sysctl.conf
    sysctl -p
    
    systemctl enable openvpn-server@server
    
    log "OpenVPN configured successfully"
}

# Configure firewall
configure_firewall() {
    log "Configuring firewall..."
    
    if command -v ufw &> /dev/null; then
        # Ubuntu/Debian - UFW
        ufw --force reset
        ufw default deny incoming
        ufw default allow outgoing
        
        # Allow SSH (will be restricted later)
        ufw allow 22/tcp
        
        # Allow OpenVPN
        ufw allow 1194/udp
        
        # Allow Dvarpala web interface
        ufw allow 8080/tcp
        
        # Allow HTTPS for Let's Encrypt
        ufw allow 443/tcp
        
        # Allow HTTP for Let's Encrypt challenges
        ufw allow 80/tcp
        
        # Enable UFW
        ufw --force enable
        
    elif command -v firewall-cmd &> /dev/null; then
        # RHEL/CentOS - Firewalld
        systemctl enable firewalld
        systemctl start firewalld
        
        firewall-cmd --permanent --add-service=ssh
        firewall-cmd --permanent --add-port=1194/udp
        firewall-cmd --permanent --add-port=8080/tcp
        firewall-cmd --permanent --add-port=443/tcp
        firewall-cmd --permanent --add-port=80/tcp
        
        firewall-cmd --reload
    fi
    
    # Configure NAT for VPN traffic
    iptables -t nat -A POSTROUTING -s 172.30.100.0/24 -o $(ip route | grep default | awk '{print $5}') -j MASQUERADE
    iptables-save > /etc/iptables/rules.v4 2>/dev/null || iptables-save > /etc/sysconfig/iptables 2>/dev/null || true
    
    log "Firewall configured successfully"
}

# Read admin configuration from cloud installer
read_admin_config() {
    ADMIN_EMAIL=""
    ADMIN_NAME=""
    
    if [[ -f "$CONFIG_DIR/admin-email.txt" ]]; then
        ADMIN_EMAIL=$(cat "$CONFIG_DIR/admin-email.txt")
    fi
    
    if [[ -f "$CONFIG_DIR/admin-name.txt" ]]; then
        ADMIN_NAME=$(cat "$CONFIG_DIR/admin-name.txt")
    fi
    
    # Fallback to prompting
    if [[ -z "$ADMIN_EMAIL" ]]; then
        echo -n "Administrator email: "
        read ADMIN_EMAIL
    fi
    
    if [[ -z "$ADMIN_NAME" ]]; then
        echo -n "Administrator name: "
        read ADMIN_NAME
    fi
    
    echo "$ADMIN_EMAIL" > "$CONFIG_DIR/admin-email.txt"
    echo "$ADMIN_NAME" > "$CONFIG_DIR/admin-name.txt"
}

# Create dvarpala configuration
create_dvarpala_config() {
    log "Creating dvarpala configuration..."
    
    # Generate JWT secret
    JWT_SECRET=$(openssl rand -base64 64)
    
    # Read database password
    DB_PASSWORD=$(cat "$CONFIG_DIR/.db_password")
    
    # Create environment configuration
    cat > "$CONFIG_DIR/.env" <<EOF
# Database Configuration
DB_HOST=localhost
DB_PORT=5432
DB_NAME=dvarpala
DB_USER=dvarpala
DB_PASSWORD=$DB_PASSWORD

# Redis Configuration
REDIS_HOST=localhost
REDIS_PORT=6379

# JWT Configuration
JWT_SECRET=$JWT_SECRET

# Server Configuration
SERVER_HOST=0.0.0.0
SERVER_PORT=8080
LOG_LEVEL=info

# OpenVPN Configuration
OPENVPN_CONFIG=/etc/openvpn/server/server.conf
OPENVPN_STATUS=/var/log/openvpn/openvpn-status.log

# Admin Configuration
ADMIN_EMAIL=$ADMIN_EMAIL
ADMIN_NAME=$ADMIN_NAME

# Captive Portal Configuration
CAPTIVE_PORTAL_NETWORK=172.30.100.0/24
FULL_ACCESS_NETWORK=172.30.8.0/21
EOF

    chmod 600 "$CONFIG_DIR/.env"
    chown "$DVARPALA_USER:$DVARPALA_USER" "$CONFIG_DIR/.env"
    
    log "Dvarpala configuration created"
}

# Initialize database
initialize_database() {
    log "Initializing database..."
    
    cd "$DVARPALA_DIR"
    sudo -u "$DVARPALA_USER" ./bin/dvarpala-server --config "$CONFIG_DIR/.env" --migrate
    
    log "Database initialized successfully"
}

# Generate admin VPN certificate
generate_admin_cert() {
    log "Generating admin VPN certificate..."
    
    cd /etc/openvpn/easy-rsa
    source ./vars
    
    # Build admin certificate
    echo -e "\n\n\n\n\n\n\n\ny\ny\n" | ./build-key admin
    
    # Create admin OpenVPN configuration
    cat > "$DVARPALA_DIR/certs/admin.ovpn" <<EOF
client
dev tun
proto udp
remote $(curl -s http://checkip.amazonaws.com) 1194
resolv-retry infinite
nobind
persist-key
persist-tun
remote-cert-tls server
cipher AES-256-GCM
verb 3

<ca>
$(cat keys/ca.crt)
</ca>

<cert>
$(cat keys/admin.crt)
</cert>

<key>
$(cat keys/admin.key)
</key>

<tls-auth>
$(cat keys/ta.key)
</tls-auth>
key-direction 1
EOF

    chmod 600 "$DVARPALA_DIR/certs/admin.ovpn"
    chown "$DVARPALA_USER:$DVARPALA_USER" "$DVARPALA_DIR/certs/admin.ovpn"
    
    log "Admin VPN certificate generated"
}

# Create systemd services
create_systemd_services() {
    log "Creating systemd services..."
    
    # Dvarpala server service
    cat > /etc/systemd/system/dvarpala.service <<EOF
[Unit]
Description=Dvarpala VPN Management Server
After=network.target postgresql.service redis.service

[Service]
Type=simple
User=$DVARPALA_USER
WorkingDirectory=$DVARPALA_DIR
ExecStart=$DVARPALA_DIR/bin/dvarpala-server --config $CONFIG_DIR/.env
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF

    # Dvarpala worker service
    cat > /etc/systemd/system/dvarpala-worker.service <<EOF
[Unit]
Description=Dvarpala Background Worker
After=network.target postgresql.service redis.service

[Service]
Type=simple
User=$DVARPALA_USER
WorkingDirectory=$DVARPALA_DIR
ExecStart=$DVARPALA_DIR/bin/dvarpala-worker --config $CONFIG_DIR/.env
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    systemctl enable dvarpala
    systemctl enable dvarpala-worker
    systemctl enable openvpn-server@server
    
    log "Systemd services created"
}

# Start services
start_services() {
    log "Starting services..."
    
    # Start OpenVPN first
    systemctl start openvpn-server@server
    
    # Wait a moment for OpenVPN to initialize
    sleep 5
    
    # Start dvarpala services
    systemctl start dvarpala
    systemctl start dvarpala-worker
    
    log "Services started successfully"
}

# Display final information
display_final_info() {
    echo
    echo -e "${GREEN}🎉 Dvarpala Installation Complete!${NC}"
    echo "============================================="
    echo
    echo -e "${BLUE}Server Information:${NC}"
    echo "  Public IP: $(curl -s http://checkip.amazonaws.com)"
    echo "  VPN Network: 172.30.100.0/24 (captive portal)"
    echo "  Admin VPN Config: $DVARPALA_DIR/certs/admin.ovpn"
    echo
    echo -e "${BLUE}Admin Access:${NC}"
    echo "  Email: $ADMIN_EMAIL"
    echo "  VPN Config: Download admin.ovpn for VPN access"
    echo "  Web Interface: http://172.30.100.1:8080 (via VPN)"
    echo
    echo -e "${BLUE}Database Credentials:${NC}"
    echo "  Username: dvarpala"
    echo "  Password: $(cat "$CONFIG_DIR/.db_password")"
    echo "  Database: dvarpala"
    echo
    echo -e "${BLUE}Next Steps:${NC}"
    echo "  1. Download $DVARPALA_DIR/certs/admin.ovpn"
    echo "  2. Connect to VPN using admin certificate"
    echo "  3. Access http://172.30.100.1:8080 in browser"
    echo "  4. Configure OAuth providers"
    echo "  5. Generate user certificates as needed"
    echo
    echo -e "${YELLOW}⚠️  Important:${NC}"
    echo "  - SSH access will be restricted to VPN network"
    echo "  - Save the database password shown above"
    echo "  - Keep your admin.ovpn file secure"
    echo
    
    # Save information to file
    cat > "$DVARPALA_DIR/installation-summary.txt" <<EOF
Dvarpala Installation Summary
============================

Installed: $(date)
Public IP: $(curl -s http://checkip.amazonaws.com)
Admin Email: $ADMIN_EMAIL

Files:
- Admin VPN: $DVARPALA_DIR/certs/admin.ovpn
- Config: $CONFIG_DIR/.env
- Database Password: $CONFIG_DIR/.db_password

Services:
- dvarpala.service
- dvarpala-worker.service  
- openvpn-server@server.service
- postgresql.service
- redis.service

Access:
- Web Interface: http://172.30.100.1:8080 (via VPN)
- SSH: ssh dvarpala@172.30.100.1 (via VPN)
EOF

    chmod 644 "$DVARPALA_DIR/installation-summary.txt"
}

# Main installation flow
main() {
    log "Starting dvarpala installation..."
    
    create_dvarpala_user
    create_directories
    install_dependencies
    install_go
    build_dvarpala
    configure_postgresql
    configure_redis
    configure_openvpn
    configure_firewall
    read_admin_config
    create_dvarpala_config
    initialize_database
    generate_admin_cert
    create_systemd_services
    start_services
    display_final_info
    
    log "Installation completed successfully!"
}

# Run main function
main "$@"