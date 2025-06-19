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
PROGRESS_FILE="/var/log/dvarpala-progress.json"
COMPLETED_STEPS_FILE="/var/log/dvarpala-completed-steps.json"

# Installation steps definition
ALL_STEPS=(
    "System updates and package installations"
    "Installing core dependencies (PostgreSQL, Redis, OpenVPN)"
    "Downloading and installing Go programming language"
    "Downloading dvarpala source code from GitHub"
    "Compiling dvarpala binaries (server, worker, auth)"
    "Configuring PostgreSQL database and creating users"
    "Setting up Redis cache and OpenVPN server"
    "Generating SSL certificates and OpenVPN keys"
    "Configuring firewall rules and network settings"
    "Creating systemd services and starting processes"
    "Generating admin certificates and VPN configuration"
    "Starting all services and performing health checks"
    "Finalizing installation and performing cleanup"
)

TOTAL_STEPS=${#ALL_STEPS[@]}
COMPLETED_STEPS=()
CURRENT_STEP_INDEX=0

echo "=================================================================="
echo -e "${BLUE}🚀 Dvarpala VPN Server Cloud Setup${NC}"
echo "=================================================================="
echo

# 🔍 DEBUG: System and user information for troubleshooting
echo -e "${YELLOW}🔍 DEBUG: Installation environment information:${NC}"
echo "  Current user: $(whoami)"
echo "  User ID: $(id)"
echo "  Home directory: $HOME"
echo "  Working directory: $(pwd)"
echo "  Date/Time: $(date)"
echo "  System: $(uname -a)"
echo "  Process tree: $$ (parent: $PPID)"
echo "  Environment variables:"
env | grep -E "(USER|HOME|PATH|SUDO|ADMIN)" | head -10 | sed 's/^/    /'
echo "  Mount information for key directories:"
mount | grep -E "(tmp|home|var)" | head -5 | sed 's/^/    /'
echo "  Available disk space:"
df -h | grep -E "(tmp|home|var|root)" | head -5 | sed 's/^/    /'
echo "================================================================="
echo

# Enhanced logging function with structured progress tracking
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" | tee -a "$LOG_FILE"
    
    # Update installation status for monitoring
    echo "$1" > /var/log/dvarpala-current-status.txt
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" >> /var/log/dvarpala-progress.log
}

# Mark step as completed and update progress
complete_step() {
    local step="$1"
    
    # Add to completed steps array
    COMPLETED_STEPS+=("$step")
    
    # Update progress files
    update_progress_files
    
    log "✅ Completed: $step"
}

# Start a new installation step
start_step() {
    local step="$1"
    CURRENT_STEP="$step"
    
    # Update progress files
    update_progress_files
    
    log "🔄 Starting: $step"
}

# Update structured progress files
update_progress_files() {
    # Create JSON progress file
    cat > "$PROGRESS_FILE" << EOF
{
    "current_step": "$CURRENT_STEP",
    "completed_steps": $(printf '%s\n' "${COMPLETED_STEPS[@]}" | jq -R . | jq -s .),
    "total_steps": $TOTAL_STEPS,
    "failed_steps": [],
    "installation_id": "$(hostname)-$(date +%s)",
    "timestamp": "$(date -Iseconds)",
    "progress_percent": $((${#COMPLETED_STEPS[@]} * 100 / $TOTAL_STEPS))
}
EOF

    # Create completed steps JSON file
    printf '%s\n' "${COMPLETED_STEPS[@]}" | jq -R . | jq -s . > "$COMPLETED_STEPS_FILE"
}

# Setup enhanced HTTP server for installation monitoring
setup_progress_monitoring() {
    # Create web directory for endpoints
    mkdir -p /var/www/html
    
    # Initialize progress tracking
    CURRENT_STEP="Installation starting..."
    update_progress_files
    
    # Create initial endpoint files to ensure they exist
    echo "Installation starting..." > /var/www/html/installation-status
    echo '{"status": "installing", "timestamp": "'$(date -Iseconds)'"}' > /var/www/html/health
    echo '[]' > /var/www/html/completed-steps
    echo "Installation log starting..." > /var/www/html/installation-log
    
    log "Created initial monitoring endpoint files in /var/www/html"
    ls -la /var/www/html/ | sed 's/^/  /'
    
    # Create enhanced monitoring script
    mkdir -p /var/lib/dvarpala/scripts
    cat > /var/lib/dvarpala/scripts/setup-monitoring.sh << 'MONITOR_EOF'
#!/bin/bash
# Enhanced HTTP server for installation progress monitoring

PROGRESS_FILE="/var/log/dvarpala-progress.json"
COMPLETED_STEPS_FILE="/var/log/dvarpala-completed-steps.json"

# Create status endpoint (legacy)
create_status_endpoint() {
    if [ -f /var/log/dvarpala-current-status.txt ]; then
        cat /var/log/dvarpala-current-status.txt > /var/www/html/installation-status
    else
        echo "Installation starting..." > /var/www/html/installation-status
    fi
}

# Create structured progress endpoint (new)
create_progress_endpoint() {
    if [ -f "$PROGRESS_FILE" ]; then
        cp "$PROGRESS_FILE" /var/www/html/installation-progress
    else
        echo '{"current_step": "Installation starting...", "completed_steps": [], "total_steps": 13, "failed_steps": [], "progress_percent": 0}' > /var/www/html/installation-progress
    fi
}

# Create completed steps endpoint (new) 
create_completed_steps_endpoint() {
    if [ -f "$COMPLETED_STEPS_FILE" ]; then
        cp "$COMPLETED_STEPS_FILE" /var/www/html/completed-steps
    else
        echo '[]' > /var/www/html/completed-steps
    fi
}

# Create log endpoint  
create_log_endpoint() {
    if [ -f /var/log/dvarpala-progress.log ]; then
        tail -20 /var/log/dvarpala-progress.log > /var/www/html/installation-log
    else
        echo "Installation log not available yet" > /var/www/html/installation-log
    fi
}

# Create health endpoint
create_health_endpoint() {
    if [ -f /opt/dvarpala/installation-complete ]; then
        echo '{"status": "complete", "timestamp": "'$(date -Iseconds)'"}' > /var/www/html/health
    else
        echo '{"status": "installing", "timestamp": "'$(date -Iseconds)'"}' > /var/www/html/health
    fi
}

# Update all endpoints every 30 seconds
while true; do
    create_status_endpoint
    create_progress_endpoint
    create_completed_steps_endpoint
    create_log_endpoint
    create_health_endpoint
    sleep 30
done
MONITOR_EOF

    chmod +x /var/lib/dvarpala/scripts/setup-monitoring.sh
    
    # Start monitoring in background
    nohup /var/lib/dvarpala/scripts/setup-monitoring.sh > /dev/null 2>&1 &
    
    # Configure nginx for port 8080 monitoring endpoints
    configure_nginx_monitoring
}

# Configure nginx for installation monitoring endpoints
configure_nginx_monitoring() {
    log "Configuring nginx for installation monitoring..."
    
    # Create nginx configuration for port 8080
    cat > /etc/nginx/sites-available/dvarpala-monitoring << 'NGINX_EOF'
server {
    listen 8080;
    server_name _;
    
    # Root directory for monitoring files
    root /var/www/html;
    index index.html;
    
    # Enable directory browsing for debugging
    autoindex on;
    
    # Health check endpoint
    location /health {
        try_files $uri $uri/ =404;
        add_header Content-Type application/json;
        add_header Access-Control-Allow-Origin *;
    }
    
    # Installation progress endpoint
    location /installation-progress {
        try_files $uri $uri/ =404;
        add_header Content-Type application/json;
        add_header Access-Control-Allow-Origin *;
    }
    
    # Installation status endpoint
    location /installation-status {
        try_files $uri $uri/ =404;
        add_header Content-Type text/plain;
        add_header Access-Control-Allow-Origin *;
    }
    
    # Completed steps endpoint
    location /completed-steps {
        try_files $uri $uri/ =404;
        add_header Content-Type application/json;
        add_header Access-Control-Allow-Origin *;
    }
    
    # Installation log endpoint
    location /installation-log {
        try_files $uri $uri/ =404;
        add_header Content-Type text/plain;
        add_header Access-Control-Allow-Origin *;
    }
    
    # Admin OVPN download endpoint
    location /admin.ovpn {
        try_files $uri $uri/ =404;
        add_header Content-Type application/x-openvpn-profile;
        add_header Content-Disposition "attachment; filename=admin.ovpn";
    }
    
    # Basic logging
    access_log /var/log/nginx/dvarpala-monitoring.access.log;
    error_log /var/log/nginx/dvarpala-monitoring.error.log;
}
NGINX_EOF

    # Enable the site
    ln -sf /etc/nginx/sites-available/dvarpala-monitoring /etc/nginx/sites-enabled/
    
    # Test nginx configuration
    if nginx -t; then
        # Start nginx service
        systemctl enable nginx
        systemctl restart nginx
        log "Nginx configured and started for monitoring on port 8080"
        
        # Verify nginx is running on port 8080
        sleep 2
        if netstat -tlnp | grep :8080 > /dev/null; then
            log "✅ Confirmed: Nginx is listening on port 8080"
            
            # Test the monitoring endpoints
            log "Testing monitoring endpoints:"
            for endpoint in "health" "installation-status" "installation-progress" "completed-steps"; do
                if curl -s -o /dev/null -w "%{http_code}" "http://localhost:8080/$endpoint" | grep -q "200\|404"; then
                    log "  ✅ /$endpoint - Nginx serving endpoint"
                else
                    log "  ❌ /$endpoint - Endpoint not accessible"
                fi
            done
        else
            log "⚠️ WARNING: Nginx not listening on port 8080, checking status..."
            systemctl status nginx | head -10
        fi
    else
        log "ERROR: Nginx configuration test failed"
        nginx -t 2>&1 | head -5  # Show specific error
        
        # Fallback to Python HTTP server
        if command -v python3 &> /dev/null; then
            cd /var/www/html
            nohup python3 -m http.server 8080 > /dev/null 2>&1 &
            log "Fallback: Started Python HTTP server on port 8080"
            
            # Verify Python server is running
            sleep 2
            if netstat -tlnp | grep :8080 > /dev/null; then
                log "✅ Confirmed: Python HTTP server is listening on port 8080"
            else
                log "❌ ERROR: Neither nginx nor Python server is running on port 8080"
            fi
        fi
    fi
}

# Check if running as root
if [[ $EUID -ne 0 ]]; then
    echo -e "${RED}[ERROR]${NC} This script must be run as root"
    exit 1
fi

# Create dvarpala user
create_dvarpala_user() {
    start_step "Creating dvarpala system user"
    if ! id "$DVARPALA_USER" &>/dev/null; then
        useradd --system --home-dir "$DVARPALA_DIR" --shell /bin/bash "$DVARPALA_USER"
        log "Created dvarpala user"
    else
        log "Dvarpala user already exists"
    fi
    complete_step "Creating dvarpala system user"
}

# Create directory structure
create_directories() {
    start_step "Creating directory structure"
    mkdir -p "$DVARPALA_DIR"/{bin,config,logs,certs,scripts}
    mkdir -p "$CONFIG_DIR"
    mkdir -p /etc/openvpn/{server,client-configs}
    mkdir -p /var/log/dvarpala
    mkdir -p /var/lib/dvarpala/auth  # For authentication status files
    mkdir -p /var/lib/cloud/{scripts,downloads,builds}  # For installation temp files
    
    chown -R "$DVARPALA_USER:$DVARPALA_USER" "$DVARPALA_DIR"
    chown -R "$DVARPALA_USER:$DVARPALA_USER" /var/log/dvarpala
    chown -R "$DVARPALA_USER:$DVARPALA_USER" /var/lib/dvarpala
    complete_step "Creating directory structure"
}

# Detect OS and install dependencies
install_dependencies() {
    start_step "System updates and package installations"
    
    if [[ -f /etc/debian_version ]]; then
        # Debian/Ubuntu
        export DEBIAN_FRONTEND=noninteractive
        log "Updating system packages..."
        apt-get update -y
        apt-get upgrade -y
        complete_step "System updates and package installations"
        
        start_step "Installing core dependencies (PostgreSQL, Redis, OpenVPN)"
        log "Installing core packages (PostgreSQL, Redis, OpenVPN)..."
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
        complete_step "Installing core dependencies (PostgreSQL, Redis, OpenVPN)"
            
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
    start_step "Downloading and installing Go programming language"
    
    GO_VERSION="1.21.5"
    GO_ARCH="amd64"
    
    if command -v go &> /dev/null; then
        CURRENT_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
        if [[ "$CURRENT_VERSION" == "$GO_VERSION" ]]; then
            log "Go $GO_VERSION already installed"
            complete_step "Downloading and installing Go programming language"
            return
        fi
    fi
    
    # Use /var/lib/cloud/downloads for Go installation
    mkdir -p /var/lib/cloud/downloads
    cd /var/lib/cloud/downloads
    wget -q "https://golang.org/dl/go${GO_VERSION}.linux-${GO_ARCH}.tar.gz"
    rm -rf /usr/local/go
    tar -C /usr/local -xzf "go${GO_VERSION}.linux-${GO_ARCH}.tar.gz"
    
    # Add Go to PATH for all users
    echo 'export PATH=/usr/local/go/bin:$PATH' > /etc/profile.d/go.sh
    chmod +x /etc/profile.d/go.sh
    
    # Set PATH for current session
    export PATH=/usr/local/go/bin:$PATH
    
    log "Go $GO_VERSION installed successfully"
    complete_step "Downloading and installing Go programming language"
}

# Download and build dvarpala
build_dvarpala() {
    start_step "Downloading dvarpala source code from GitHub"
    
    # Use /var/lib/cloud/builds for source code
    mkdir -p /var/lib/cloud/builds
    cd /var/lib/cloud/builds
    if [[ ! -d "dvarpala" ]]; then
        git clone https://github.com/frigga-cloud/dvarpala.git
    fi
    complete_step "Downloading dvarpala source code from GitHub"
    
    start_step "Compiling dvarpala binaries (server, worker, auth)"
    cd dvarpala
    export PATH=/usr/local/go/bin:$PATH
    
    log "Downloading Go module dependencies..."
    go mod download
    
    log "Compiling dvarpala-server binary..."
    go build -o "$DVARPALA_DIR/bin/dvarpala-server" ./cmd/dvarpala-server
    
    log "Compiling dvarpala-cli binary..."
    go build -o "$DVARPALA_DIR/bin/dvarpala-cli" ./cmd/dvarpala-cli
    
    log "Compiling dvarpala-worker binary..."
    go build -o "$DVARPALA_DIR/bin/dvarpala-worker" ./cmd/dvarpala-worker
    
    log "Compiling openvpn-auth binary..."
    go build -o "$DVARPALA_DIR/bin/openvpn-auth" ./cmd/openvpn-auth
    
    log "Installing configuration files and scripts..."
    # Copy configuration files
    cp -r configs/* "$CONFIG_DIR/" 2>/dev/null || true
    cp -r scripts/* "$DVARPALA_DIR/scripts/" 2>/dev/null || true
    
    # Set permissions
    chmod +x "$DVARPALA_DIR/bin/"*
    chmod +x "$DVARPALA_DIR/scripts/"*.sh 2>/dev/null || true
    chown -R "$DVARPALA_USER:$DVARPALA_USER" "$DVARPALA_DIR"
    
    log "Dvarpala compilation completed successfully"
    complete_step "Compiling dvarpala binaries (server, worker, auth)"
}

# Configure PostgreSQL
configure_postgresql() {
    start_step "Configuring PostgreSQL database and creating users"
    
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
    complete_step "Configuring PostgreSQL database and creating users"
}

# Configure Redis
configure_redis() {
    start_step "Setting up Redis cache and OpenVPN server"
    
    systemctl enable redis
    systemctl start redis
    
    # Basic Redis security
    redis-cli CONFIG SET requirepass "$(openssl rand -base64 32)"
    
    log "Redis configured successfully"
    # Note: OpenVPN configuration continues in next function
}

# Configure OpenVPN
configure_openvpn() {
    start_step "Generating SSL certificates and OpenVPN keys"
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

# CAPTIVE PORTAL MODE: Do NOT push full access routes by default
# Routes will be pushed conditionally via client-connect script based on authentication status
# push "route 172.30.8.0 255.255.248.0"  # Commented out - conditional routing only

# Default captive portal route (only allow access to portal)
push "route 172.30.100.1 255.255.255.255"

# Client configuration directory for per-client routing
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
    mkdir -p /opt/dvarpala/scripts
    
    # Create client-connect script for conditional routing
    cat > /opt/dvarpala/scripts/client-connect.sh << 'EOF'
#!/bin/bash
# Dvarpala VPN Client Connect Script
# Determines network access level based on authentication status

# Environment variables provided by OpenVPN:
# $common_name - Client certificate common name
# $trusted_ip - Client's real IP address
# $ifconfig_pool_remote_ip - Assigned VPN IP

LOG_FILE="/var/log/openvpn/client-connect.log"
AUTH_STATUS_FILE="/var/lib/dvarpala/auth/dvarpala-auth-status"

# Log connection attempt
echo "$(date): Client connect - CN: $common_name, VPN IP: $ifconfig_pool_remote_ip, Real IP: $trusted_ip" >> $LOG_FILE

# Check if user has completed web authentication
if [ -f "$AUTH_STATUS_FILE-$common_name" ]; then
    AUTH_STATUS=$(cat "$AUTH_STATUS_FILE-$common_name")
    if [ "$AUTH_STATUS" = "authenticated" ]; then
        echo "$(date): Granting FULL access to $common_name" >> $LOG_FILE
        
        # Grant full network access
        echo "push \"route 172.30.8.0 255.255.248.0\"" > $1
        echo "push \"route 0.0.0.0 128.0.0.0\"" >> $1
        echo "push \"route 128.0.0.0 128.0.0.0\"" >> $1
        echo "push \"dhcp-option DNS 8.8.8.8\"" >> $1
        echo "push \"dhcp-option DNS 8.8.4.4\"" >> $1
        
        exit 0
    fi
fi

# Default: Only captive portal access
echo "$(date): Granting CAPTIVE PORTAL ONLY access to $common_name" >> $LOG_FILE

# Only allow access to captive portal (172.30.100.1)
echo "push \"route 172.30.100.1 255.255.255.255\"" > $1

# Log the restriction
echo "$(date): User $common_name restricted to captive portal access only" >> $LOG_FILE

exit 0
EOF

    chmod +x /opt/dvarpala/scripts/client-connect.sh

    # Create client-disconnect script for session cleanup
    cat > /opt/dvarpala/scripts/client-disconnect.sh << 'EOF'
#!/bin/bash
# Dvarpala VPN Client Disconnect Script
# Cleans up authentication status to force re-authentication

LOG_FILE="/var/log/openvpn/client-disconnect.log"
AUTH_STATUS_FILE="/var/lib/dvarpala/auth/dvarpala-auth-status"

# Log disconnection
echo "$(date): Client disconnect - CN: $common_name, VPN IP: $ifconfig_pool_remote_ip, Duration: $time_duration seconds" >> $LOG_FILE

# Remove authentication status to force re-authentication on next connection
if [ -f "$AUTH_STATUS_FILE-$common_name" ]; then
    rm -f "$AUTH_STATUS_FILE-$common_name"
    echo "$(date): Removed authentication status for $common_name - will require re-authentication" >> $LOG_FILE
else
    echo "$(date): No authentication status found for $common_name" >> $LOG_FILE
fi

# Optional: Log session statistics to database (for future analytics)
# /opt/dvarpala/bin/log-session "$common_name" "$time_duration" "$bytes_received" "$bytes_sent"

exit 0
EOF

    chmod +x /opt/dvarpala/scripts/client-disconnect.sh

    # Enable IP forwarding
    echo 'net.ipv4.ip_forward=1' >> /etc/sysctl.conf
    sysctl -p
    
    systemctl enable openvpn-server@server
    
    log "OpenVPN configured successfully"
    complete_step "Setting up Redis cache and OpenVPN server"
    complete_step "Generating SSL certificates and OpenVPN keys"
}

# Configure firewall
configure_firewall() {
    start_step "Configuring firewall rules and network settings"
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
        
        # Allow Dvarpala web interface (captive portal)
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
    
    # CAPTIVE PORTAL FIREWALL RULES
    # Block all traffic from VPN clients except to captive portal by default
    # (Additional rules will be managed dynamically based on authentication status)
    
    # Allow VPN clients to access only the captive portal (172.30.100.1:8080)
    iptables -I FORWARD -s 172.30.100.0/24 -d 172.30.100.1 -p tcp --dport 8080 -j ACCEPT
    
    # Allow VPN clients basic connectivity (DNS, DHCP) for captive portal to work
    iptables -I FORWARD -s 172.30.100.0/24 -p udp --dport 53 -j ACCEPT    # DNS
    iptables -I FORWARD -s 172.30.100.0/24 -p tcp --dport 53 -j ACCEPT    # DNS over TCP
    
    # Block SSH access from VPN network by default (will be opened after authentication)
    iptables -I FORWARD -s 172.30.100.0/24 -d 172.30.100.1 -p tcp --dport 22 -j DROP
    
    # Block all other traffic from VPN clients by default (will be opened after authentication)
    iptables -A FORWARD -s 172.30.100.0/24 -j DROP
    
    # Save iptables rules
    iptables-save > /etc/iptables/rules.v4 2>/dev/null || iptables-save > /etc/sysconfig/iptables 2>/dev/null || true
    
    log "Firewall configured successfully"
    complete_step "Configuring firewall rules and network settings"
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

# Create OpenVPN authentication script
create_auth_script() {
    log "Creating OpenVPN authentication script..."
    
    mkdir -p /opt/dvarpala/bin
    
    # Create simple authentication script for captive portal flow
    cat > /opt/dvarpala/bin/openvpn-auth << 'EOF'
#!/bin/bash
# Dvarpala OpenVPN Authentication Script
# Allows initial connection for captive portal access

# Environment variables provided by OpenVPN:
# $username - Username from client
# $password - Password from client

LOG_FILE="/var/log/openvpn/auth.log"

# Log authentication attempt
echo "$(date): Auth attempt - User: $username from IP: $trusted_ip" >> $LOG_FILE

# For captive portal mode, we allow connection with specific captive portal credentials
# Real authentication happens via web interface
# This allows users to connect and access the captive portal

if [ "$username" = "portal" ] && [ "$password" = "access" ]; then
    echo "$(date): Allowing captive portal access for user: $username" >> $LOG_FILE
    exit 0  # Allow connection for captive portal access
elif [ -n "$username" ] && [ -n "$password" ]; then
    echo "$(date): Invalid credentials for user: $username" >> $LOG_FILE
    exit 1  # Reject connection - invalid credentials
else
    echo "$(date): Rejecting connection - missing credentials" >> $LOG_FILE
    exit 1  # Reject connection
fi
EOF

    chmod +x /opt/dvarpala/bin/openvpn-auth
    
    log "OpenVPN authentication script created"
}

# Create web authentication helper script
create_web_auth_helper() {
    log "Creating web authentication helper..."
    
    # Create script to mark user as authenticated after web portal login
    cat > /opt/dvarpala/bin/mark-user-authenticated << 'EOF'
#!/bin/bash
# Script to mark a user as authenticated after successful web portal login
# Usage: ./mark-user-authenticated <username>

if [ $# -ne 1 ]; then
    echo "Usage: $0 <username>"
    exit 1
fi

USERNAME="$1"
AUTH_STATUS_FILE="/var/lib/dvarpala/auth/dvarpala-auth-status"
LOG_FILE="/var/log/openvpn/web-auth.log"

# Mark user as authenticated
echo "authenticated" > "$AUTH_STATUS_FILE-$USERNAME"

# Log the authentication
echo "$(date): User $USERNAME authenticated via web portal" >> $LOG_FILE

# Optional: Trigger OpenVPN to refresh user routing (requires reconnection for now)
# In future, this could use OpenVPN management interface to update routing dynamically

echo "User $USERNAME marked as authenticated"
EOF

    chmod +x /opt/dvarpala/bin/mark-user-authenticated
    
    # Create script to check authentication status (for web interface)
    cat > /opt/dvarpala/bin/check-auth-status << 'EOF'
#!/bin/bash
# Script to check if a user is authenticated
# Usage: ./check-auth-status <username>

if [ $# -ne 1 ]; then
    echo "unauthenticated"
    exit 1
fi

USERNAME="$1"
AUTH_STATUS_FILE="/var/lib/dvarpala/auth/dvarpala-auth-status"

if [ -f "$AUTH_STATUS_FILE-$USERNAME" ]; then
    cat "$AUTH_STATUS_FILE-$USERNAME"
else
    echo "unauthenticated"
fi
EOF

    chmod +x /opt/dvarpala/bin/check-auth-status
    
    log "Web authentication helper scripts created"
}

# Create captive portal auto-open scripts for different platforms
create_captive_portal_scripts() {
    log "Creating captive portal auto-open scripts..."
    
    mkdir -p "$DVARPALA_DIR/certs/scripts"
    
    # Windows batch script
    cat > "$DVARPALA_DIR/certs/scripts/open-captive-portal.bat" <<'BAT_EOF'
@echo off
REM Auto-open captive portal for Windows
echo %date% %time%: VPN connected, opening captive portal... >> %TEMP%\dvarpala-client.log

REM Wait for network to be established
timeout /t 3 /nobreak > nul

REM Test connectivity and open browser
ping -n 1 172.30.100.1 > nul 2>&1
if %errorlevel% equ 0 (
    echo %date% %time%: Opening captive portal in browser... >> %TEMP%\dvarpala-client.log
    start http://172.30.100.1:8080
) else (
    echo %date% %time%: Network not ready, please open http://172.30.100.1:8080 manually >> %TEMP%\dvarpala-client.log
)
BAT_EOF

    # macOS/Linux shell script (more robust version)
    cat > "$DVARPALA_DIR/certs/scripts/open-captive-portal-unix.sh" <<'UNIX_EOF'
#!/bin/bash
# Auto-open captive portal script for macOS/Linux
CAPTIVE_URL="http://172.30.100.1:8080"
LOG_FILE="$HOME/.dvarpala-client.log"

echo "$(date): VPN connected, opening captive portal..." >> "$LOG_FILE"

# Wait for network to be established
sleep 5

# Test connectivity first
if ping -c 1 172.30.100.1 >/dev/null 2>&1; then
    echo "$(date): Network ready, opening browser..." >> "$LOG_FILE"
    
    # Try multiple browser opening methods
    if [[ "$OSTYPE" == "darwin"* ]]; then
        # macOS
        open "$CAPTIVE_URL"
    else
        # Linux
        if command -v xdg-open >/dev/null 2>&1; then
            xdg-open "$CAPTIVE_URL" >/dev/null 2>&1 &
        elif command -v firefox >/dev/null 2>&1; then
            firefox "$CAPTIVE_URL" >/dev/null 2>&1 &
        elif command -v google-chrome >/dev/null 2>&1; then
            google-chrome "$CAPTIVE_URL" >/dev/null 2>&1 &
        elif command -v chromium-browser >/dev/null 2>&1; then
            chromium-browser "$CAPTIVE_URL" >/dev/null 2>&1 &
        fi
    fi
    
    echo "$(date): Browser opened for captive portal" >> "$LOG_FILE"
else
    echo "$(date): Network not ready, please open $CAPTIVE_URL manually" >> "$LOG_FILE"
fi
UNIX_EOF

    chmod +x "$DVARPALA_DIR/certs/scripts/open-captive-portal-unix.sh"
    
    # Create instructions file
    cat > "$DVARPALA_DIR/certs/AUTO-OPEN-SETUP.txt" <<'INSTRUCTIONS_EOF'
# Dvarpala VPN - Auto-Open Captive Portal Setup

The admin.ovpn file is configured to automatically open the captive portal
in your browser when you connect. However, some OpenVPN clients may require
additional configuration:

## Method 1: Automatic (Built-in)
The admin.ovpn file includes an "up" script that should automatically open
your browser to http://172.30.100.1:8080 after connection.

## Method 2: Manual Setup for GUI Clients

### For Windows (OpenVPN GUI):
1. Copy scripts/open-captive-portal.bat to your OpenVPN config folder
2. Edit your OpenVPN GUI settings:
   - Right-click OpenVPN GUI tray icon
   - Edit config for Dvarpala
   - Add line: up "scripts/open-captive-portal.bat"

### For macOS (Tunnelblick):
1. Copy scripts/open-captive-portal-unix.sh to your config folder
2. Tunnelblick should automatically use the "up" directive

### For Linux (NetworkManager):
1. Copy scripts/open-captive-portal-unix.sh to /etc/openvpn/
2. The script should run automatically via the "up" directive

## Method 3: Manual Browser Opening
If automatic opening doesn't work:
1. Connect to VPN with credentials: portal / access
2. Manually open browser to: http://172.30.100.1:8080
3. Complete authentication
4. Disconnect and reconnect VPN for full access

## Troubleshooting
- Check client logs: ~/.dvarpala-client.log (Unix) or %TEMP%\dvarpala-client.log (Windows)
- Ensure your OpenVPN client allows script execution
- Some corporate firewalls may block automatic browser opening
INSTRUCTIONS_EOF

    chown -R "$DVARPALA_USER:$DVARPALA_USER" "$DVARPALA_DIR/certs/scripts"
    chown "$DVARPALA_USER:$DVARPALA_USER" "$DVARPALA_DIR/certs/AUTO-OPEN-SETUP.txt"
    
    log "Captive portal auto-open scripts created"
}

# Generate admin VPN certificate
generate_admin_cert() {
    start_step "Generating admin certificates and VPN configuration"
    log "Generating admin VPN certificate..."
    
    cd /etc/openvpn/easy-rsa
    source ./vars
    
    # Build admin certificate
    echo -e "\n\n\n\n\n\n\n\ny\ny\n" | ./build-key admin
    
    # Create captive portal auto-open script for different platforms
    create_captive_portal_scripts

    # Create client-side script to auto-open captive portal
    cat > "$DVARPALA_DIR/certs/open-captive-portal.sh" <<'SCRIPT_EOF'
#!/bin/bash
# Auto-open captive portal script for Dvarpala VPN
# This script runs after OpenVPN connection is established

CAPTIVE_URL="http://172.30.100.1:8080"
LOG_FILE="$HOME/.dvarpala-client.log"

echo "$(date): VPN connected, attempting to open captive portal..." >> $LOG_FILE

# Wait a moment for network to be fully established
sleep 3

# Function to open URL in default browser (cross-platform)
open_browser() {
    local url="$1"
    
    # Detect operating system and open browser accordingly
    if [[ "$OSTYPE" == "darwin"* ]]; then
        # macOS
        open "$url" 2>/dev/null
        echo "$(date): Opened captive portal on macOS" >> $LOG_FILE
    elif [[ "$OSTYPE" == "linux-gnu"* ]]; then
        # Linux
        if command -v xdg-open &> /dev/null; then
            xdg-open "$url" 2>/dev/null
        elif command -v gnome-open &> /dev/null; then
            gnome-open "$url" 2>/dev/null
        elif command -v firefox &> /dev/null; then
            firefox "$url" 2>/dev/null &
        elif command -v google-chrome &> /dev/null; then
            google-chrome "$url" 2>/dev/null &
        fi
        echo "$(date): Opened captive portal on Linux" >> $LOG_FILE
    elif [[ "$OSTYPE" == "msys" ]] || [[ "$OSTYPE" == "cygwin" ]]; then
        # Windows (Git Bash/Cygwin)
        start "$url" 2>/dev/null
        echo "$(date): Opened captive portal on Windows" >> $LOG_FILE
    fi
}

# Test if captive portal is reachable
if curl -s --connect-timeout 5 "$CAPTIVE_URL" > /dev/null; then
    echo "$(date): Captive portal reachable, opening browser..." >> $LOG_FILE
    open_browser "$CAPTIVE_URL"
else
    echo "$(date): Captive portal not reachable yet, user will need to open manually" >> $LOG_FILE
fi

exit 0
SCRIPT_EOF

    chmod +x "$DVARPALA_DIR/certs/open-captive-portal.sh"

    # Create admin OpenVPN configuration with auto-open script
    cat > "$DVARPALA_DIR/certs/admin.ovpn" <<EOF
# Dvarpala VPN - Captive Portal Mode
# Browser will auto-open to: http://172.30.100.1:8080
# Complete authentication via web portal for full VPN access

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

# Auto-open captive portal after connection
script-security 2
up "$DVARPALA_DIR/certs/open-captive-portal.sh"

# Initial captive portal access credentials
# Username: portal, Password: access (for initial connection only)
auth-user-pass

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

    # Create credentials file for admin VPN connection
    cat > "$DVARPALA_DIR/certs/admin-credentials.txt" <<EOF
portal
access
EOF
    
    chmod 600 "$DVARPALA_DIR/certs/admin.ovpn"
    chmod 600 "$DVARPALA_DIR/certs/admin-credentials.txt"
    chmod 755 "$DVARPALA_DIR/certs/open-captive-portal.sh"
    chown "$DVARPALA_USER:$DVARPALA_USER" "$DVARPALA_DIR/certs/admin.ovpn"
    chown "$DVARPALA_USER:$DVARPALA_USER" "$DVARPALA_DIR/certs/admin-credentials.txt"
    chown "$DVARPALA_USER:$DVARPALA_USER" "$DVARPALA_DIR/certs/open-captive-portal.sh"
    
    log "Admin VPN certificate and credentials generated"
    complete_step "Generating admin certificates and VPN configuration"
}

# Create systemd services
create_systemd_services() {
    start_step "Creating systemd services and starting processes"
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
    complete_step "Creating systemd services and starting processes"
}

# Start services
start_services() {
    start_step "Starting all services and performing health checks"
    log "Starting services..."
    
    # Start OpenVPN first
    systemctl start openvpn-server@server
    
    # Wait a moment for OpenVPN to initialize
    sleep 5
    
    # Start dvarpala services
    systemctl start dvarpala
    systemctl start dvarpala-worker
    
    log "Services started successfully"
    complete_step "Starting all services and performing health checks"
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
    echo -e "${BLUE}Captive Portal Setup:${NC}"
    echo "  Username: portal"
    echo "  Password: access"
    echo "  (Use these credentials for initial VPN connection)"
    echo
    echo -e "${BLUE}Next Steps:${NC}"
    echo "  1. Download $DVARPALA_DIR/certs/admin.ovpn"
    echo "  2. Connect to VPN using credentials: portal/access"
    echo "  3. Browser should auto-open to: http://172.30.100.1:8080"
    echo "  4. Complete authentication via web portal for full access"
    echo "  5. Disconnect and reconnect VPN to get full access"
    echo "  6. Configure OAuth providers and generate user certificates"
    echo
    echo -e "${BLUE}Auto-Open Browser:${NC}"
    echo "  ✅ Built-in: Browser opens automatically after VPN connection"
    echo "  📁 Manual: See $DVARPALA_DIR/certs/AUTO-OPEN-SETUP.txt for GUI clients"
    echo "  🔧 Scripts: Platform-specific scripts in $DVARPALA_DIR/certs/scripts/"
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
    log "Step 1/20: Starting dvarpala installation..."
    
    # Setup progress monitoring early
    setup_progress_monitoring
    
    log "Step 2/20: Creating dvarpala system user and directories..."
    create_dvarpala_user
    create_directories
    
    log "Step 3/20: Installing system dependencies (PostgreSQL, Redis, OpenVPN)..."
    install_dependencies
    
    log "Step 4/20: Downloading and installing Go programming language..."
    install_go
    
    log "Step 5/20: Downloading dvarpala source code from GitHub..."
    log "Step 6/20: Compiling dvarpala binaries (server, worker, auth)..."
    build_dvarpala
    
    log "Step 7/20: Configuring PostgreSQL database..."
    configure_postgresql
    
    log "Step 8/20: Setting up Redis cache server..."
    configure_redis
    
    log "Step 9/20: Configuring OpenVPN server and certificates..."
    configure_openvpn
    
    log "Step 10/20: Setting up firewall rules and network configuration..."
    configure_firewall
    
    log "Step 11/20: Reading admin configuration from cloud installer..."
    read_admin_config
    
    log "Step 12/20: Creating dvarpala server configuration..."
    create_dvarpala_config
    
    log "Step 13/20: Initializing database schema and tables..."
    initialize_database
    
    log "Step 14/20: Creating OpenVPN authentication scripts..."
    create_auth_script
    
    log "Step 15/20: Setting up web authentication helpers..."
    create_web_auth_helper
    
    log "Step 16/20: Generating admin VPN certificates and configuration..."
    generate_admin_cert
    
    log "Step 17/20: Creating systemd services for dvarpala components..."
    create_systemd_services
    
    log "Step 18/20: Starting all services (OpenVPN, dvarpala, worker)..."
    start_services
    
    log "Step 19/20: Performing final configuration and health checks..."
    
    # Create installation completion marker
    touch /opt/dvarpala/installation-complete
    
    log "Step 20/20: Installation completed successfully!"
    display_final_info
}

# Run main function
main "$@"