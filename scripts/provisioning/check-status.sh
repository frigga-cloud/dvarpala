#!/bin/bash

# Dvarpala VPN Server Status Check Script
# This script checks the status of all Dvarpala components

set -euo pipefail

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo "=================================================================="
echo -e "${BLUE}📊 Dvarpala VPN Server Status Check${NC}"
echo "=================================================================="
echo

# Function to check service status
check_service() {
    local service_name="$1"
    local display_name="$2"
    
    if systemctl is-active --quiet "$service_name"; then
        echo -e "${GREEN}✅${NC} $display_name: Running"
        return 0
    else
        echo -e "${RED}❌${NC} $display_name: Not running"
        return 1
    fi
}

# Function to check port
check_port() {
    local port="$1"
    local service_name="$2"
    
    if netstat -tuln | grep -q ":$port "; then
        echo -e "${GREEN}✅${NC} Port $port ($service_name): Open"
        return 0
    else
        echo -e "${RED}❌${NC} Port $port ($service_name): Closed"
        return 1
    fi
}

# Function to check file exists
check_file() {
    local file_path="$1"
    local description="$2"
    
    if [[ -f "$file_path" ]]; then
        echo -e "${GREEN}✅${NC} $description: Found"
        return 0
    else
        echo -e "${RED}❌${NC} $description: Missing"
        return 1
    fi
}

# System Information
echo -e "${BLUE}🖥️  System Information${NC}"
echo "Hostname: $(hostname)"
echo "OS: $(lsb_release -d 2>/dev/null | cut -f2 || cat /etc/os-release | grep PRETTY_NAME | cut -d'=' -f2 | tr -d '\"')"
echo "Kernel: $(uname -r)"
echo "Uptime: $(uptime -p 2>/dev/null || uptime)"
echo

# Get server IP
SERVER_IP=$(curl -s http://ipv4.icanhazip.com 2>/dev/null || curl -s http://checkip.amazonaws.com 2>/dev/null || echo "Unable to detect")
echo "Public IP: $SERVER_IP"
echo

# Service Status
echo -e "${BLUE}🔧 Service Status${NC}"
check_service "postgresql" "PostgreSQL Database"
check_service "redis" "Redis Cache"
check_service "openvpn-server" "OpenVPN Server"
check_service "dvarpala" "Dvarpala Application"
echo

# Port Status
echo -e "${BLUE}🌐 Network Ports${NC}"
check_port "5432" "PostgreSQL"
check_port "6379" "Redis"
check_port "1194" "OpenVPN"
check_port "8080" "Dvarpala Web"
check_port "7505" "OpenVPN Management"
echo

# File Status
echo -e "${BLUE}📁 Important Files${NC}"
check_file "/opt/dvarpala/config/.env" "Environment Configuration"
check_file "/opt/dvarpala/config/.db_credentials" "Database Credentials"
check_file "/opt/dvarpala/certs/admin.ovpn" "Admin OpenVPN Profile"
check_file "/etc/openvpn/server.conf" "OpenVPN Server Config"
check_file "/opt/dvarpala/bin/dvarpala-server" "Dvarpala Binary"
echo

# OpenVPN Certificate Status
echo -e "${BLUE}🔐 OpenVPN Certificates${NC}"
if [[ -f "/etc/openvpn/easy-rsa/pki/ca.crt" ]]; then
    echo -e "${GREEN}✅${NC} CA Certificate: Valid"
    echo "   Subject: $(openssl x509 -in /etc/openvpn/easy-rsa/pki/ca.crt -noout -subject 2>/dev/null | cut -d'=' -f2-)"
    echo "   Expires: $(openssl x509 -in /etc/openvpn/easy-rsa/pki/ca.crt -noout -enddate 2>/dev/null | cut -d'=' -f2)"
else
    echo -e "${RED}❌${NC} CA Certificate: Missing"
fi

if [[ -f "/etc/openvpn/easy-rsa/pki/issued/server.crt" ]]; then
    echo -e "${GREEN}✅${NC} Server Certificate: Valid"
    echo "   Expires: $(openssl x509 -in /etc/openvpn/easy-rsa/pki/issued/server.crt -noout -enddate 2>/dev/null | cut -d'=' -f2)"
else
    echo -e "${RED}❌${NC} Server Certificate: Missing"
fi

if [[ -f "/etc/openvpn/easy-rsa/pki/issued/admin.crt" ]]; then
    echo -e "${GREEN}✅${NC} Admin Certificate: Valid"
    echo "   Expires: $(openssl x509 -in /etc/openvpn/easy-rsa/pki/issued/admin.crt -noout -enddate 2>/dev/null | cut -d'=' -f2)"
else
    echo -e "${RED}❌${NC} Admin Certificate: Missing"
fi
echo

# Database Status
echo -e "${BLUE}🗄️  Database Status${NC}"
if systemctl is-active --quiet postgresql; then
    # Check if dvarpala database exists
    if sudo -u postgres psql -lqt | cut -d \| -f 1 | grep -qw dvarpala; then
        echo -e "${GREEN}✅${NC} Dvarpala Database: Exists"
        
        # Check if dvarpala user exists
        if sudo -u postgres psql -tAc "SELECT 1 FROM pg_roles WHERE rolname='dvarpala'" | grep -q 1; then
            echo -e "${GREEN}✅${NC} Database User: Exists"
        else
            echo -e "${RED}❌${NC} Database User: Missing"
        fi
    else
        echo -e "${RED}❌${NC} Dvarpala Database: Missing"
    fi
else
    echo -e "${RED}❌${NC} Cannot check database - PostgreSQL not running"
fi
echo

# Recent Logs
echo -e "${BLUE}📝 Recent Service Logs${NC}"
echo "Dvarpala Application (last 5 lines):"
journalctl -u dvarpala --no-pager -n 5 --output=cat 2>/dev/null || echo "No logs available"
echo

echo "OpenVPN Server (last 5 lines):"
if [[ -f "/var/log/openvpn/openvpn.log" ]]; then
    tail -n 5 /var/log/openvpn/openvpn.log 2>/dev/null || echo "No logs available"
else
    journalctl -u openvpn-server --no-pager -n 5 --output=cat 2>/dev/null || echo "No logs available"
fi
echo

# Connection Information
echo -e "${BLUE}🔗 Connection Information${NC}"
if [[ "$SERVER_IP" != "Unable to detect" ]]; then
    echo "Web Interface: http://$SERVER_IP:8080"
    echo "OpenVPN Server: $SERVER_IP:1194 (UDP)"
    echo
    echo "Temporary VPN Credentials:"
    echo "Username: temp_user"
    echo "Password: temp_portal_access"
else
    echo "Unable to determine connection information"
fi
echo

# Quick Commands
echo -e "${BLUE}⚙️  Quick Management Commands${NC}"
echo "View Dvarpala logs:     journalctl -u dvarpala -f"
echo "Restart Dvarpala:       sudo systemctl restart dvarpala"
echo "View OpenVPN logs:      tail -f /var/log/openvpn/openvpn.log"
echo "Restart OpenVPN:        sudo systemctl restart openvpn-server"
echo "Configure OAuth:        sudo /opt/dvarpala/scripts/configure-oauth.sh"
echo

echo "=================================================================="
echo -e "${GREEN}📊 Status Check Complete${NC}"
echo "=================================================================="