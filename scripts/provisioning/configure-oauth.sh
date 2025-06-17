#!/bin/bash

# OAuth Configuration Script for Dvarpala VPN
# This script helps configure OAuth providers after installation

set -euo pipefail

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

DVARPALA_HOME="/opt/dvarpala"
ENV_FILE="$DVARPALA_HOME/config/.env"

# Check if running as root or dvarpala user
if [[ $EUID -ne 0 ]] && [[ $(whoami) != "dvarpala" ]]; then
    echo -e "${RED}[ERROR]${NC} This script must be run as root or dvarpala user"
    exit 1
fi

echo "=================================================================="
echo -e "${BLUE}🔧 Dvarpala OAuth Configuration${NC}"
echo "=================================================================="
echo

# Check if env file exists
if [[ ! -f "$ENV_FILE" ]]; then
    echo -e "${RED}[ERROR]${NC} Environment file not found: $ENV_FILE"
    echo "Please run the setup script first."
    exit 1
fi

# Function to update env variable
update_env_var() {
    local var_name="$1"
    local var_value="$2"
    
    if grep -q "^${var_name}=" "$ENV_FILE"; then
        sed -i "s|^${var_name}=.*|${var_name}=${var_value}|" "$ENV_FILE"
    else
        echo "${var_name}=${var_value}" >> "$ENV_FILE"
    fi
}

# Get server IP
SERVER_IP=$(curl -s http://ipv4.icanhazip.com || curl -s http://checkip.amazonaws.com || echo "localhost")
WEB_PORT=$(grep "^SERVER_PORT=" "$ENV_FILE" | cut -d'=' -f2 || echo "8080")

echo "Server IP detected: $SERVER_IP"
echo "Web interface: http://$SERVER_IP:$WEB_PORT"
echo

# Configure allowed domains
echo -e "${YELLOW}📧 Email Domain Configuration${NC}"
echo "Enter the email domains that should be allowed to access the VPN"
echo "Example: company.com,subsidiary.com"
read -p "Allowed domains: " ALLOWED_DOMAINS

if [[ -n "$ALLOWED_DOMAINS" ]]; then
    update_env_var "AUTH_ALLOWED_DOMAINS" "$ALLOWED_DOMAINS"
    echo -e "${GREEN}✅${NC} Allowed domains updated"
fi

echo

# Google OAuth Configuration
echo -e "${YELLOW}🔗 Google OAuth Configuration${NC}"
echo "To configure Google OAuth:"
echo "1. Go to https://console.cloud.google.com/"
echo "2. Create a new project or select existing one"
echo "3. Enable Google+ API"
echo "4. Create OAuth 2.0 credentials"
echo "5. Add redirect URI: http://$SERVER_IP:$WEB_PORT/auth/google/callback"
echo

read -p "Do you want to configure Google OAuth? (y/N): " -r
if [[ $REPLY =~ ^[Yy]$ ]]; then
    read -p "Google Client ID: " GOOGLE_CLIENT_ID
    read -p "Google Client Secret: " GOOGLE_CLIENT_SECRET
    
    if [[ -n "$GOOGLE_CLIENT_ID" && -n "$GOOGLE_CLIENT_SECRET" ]]; then
        update_env_var "OAUTH_GOOGLE_CLIENT_ID" "$GOOGLE_CLIENT_ID"
        update_env_var "OAUTH_GOOGLE_CLIENT_SECRET" "$GOOGLE_CLIENT_SECRET"
        update_env_var "OAUTH_GOOGLE_REDIRECT_URL" "http://$SERVER_IP:$WEB_PORT/auth/google/callback"
        echo -e "${GREEN}✅${NC} Google OAuth configured"
    fi
fi

echo

# Microsoft OAuth Configuration
echo -e "${YELLOW}🔗 Microsoft OAuth Configuration${NC}"
echo "To configure Microsoft OAuth:"
echo "1. Go to https://portal.azure.com/"
echo "2. Go to Azure Active Directory > App registrations"
echo "3. Create a new registration"
echo "4. Add redirect URI: http://$SERVER_IP:$WEB_PORT/auth/microsoft/callback"
echo "5. Create a client secret"
echo

read -p "Do you want to configure Microsoft OAuth? (y/N): " -r
if [[ $REPLY =~ ^[Yy]$ ]]; then
    read -p "Microsoft Client ID: " MICROSOFT_CLIENT_ID
    read -p "Microsoft Client Secret: " MICROSOFT_CLIENT_SECRET
    
    if [[ -n "$MICROSOFT_CLIENT_ID" && -n "$MICROSOFT_CLIENT_SECRET" ]]; then
        update_env_var "OAUTH_MICROSOFT_CLIENT_ID" "$MICROSOFT_CLIENT_ID"
        update_env_var "OAUTH_MICROSOFT_CLIENT_SECRET" "$MICROSOFT_CLIENT_SECRET"
        update_env_var "OAUTH_MICROSOFT_REDIRECT_URL" "http://$SERVER_IP:$WEB_PORT/auth/microsoft/callback"
        echo -e "${GREEN}✅${NC} Microsoft OAuth configured"
    fi
fi

echo

# GitHub OAuth Configuration
echo -e "${YELLOW}🔗 GitHub OAuth Configuration${NC}"
echo "To configure GitHub OAuth:"
echo "1. Go to https://github.com/settings/applications/new"
echo "2. Create a new OAuth App"
echo "3. Set Authorization callback URL: http://$SERVER_IP:$WEB_PORT/auth/github/callback"
echo

read -p "Do you want to configure GitHub OAuth? (y/N): " -r
if [[ $REPLY =~ ^[Yy]$ ]]; then
    read -p "GitHub Client ID: " GITHUB_CLIENT_ID
    read -p "GitHub Client Secret: " GITHUB_CLIENT_SECRET
    
    if [[ -n "$GITHUB_CLIENT_ID" && -n "$GITHUB_CLIENT_SECRET" ]]; then
        update_env_var "OAUTH_GITHUB_CLIENT_ID" "$GITHUB_CLIENT_ID"
        update_env_var "OAUTH_GITHUB_CLIENT_SECRET" "$GITHUB_CLIENT_SECRET"
        update_env_var "OAUTH_GITHUB_REDIRECT_URL" "http://$SERVER_IP:$WEB_PORT/auth/github/callback"
        echo -e "${GREEN}✅${NC} GitHub OAuth configured"
    fi
fi

echo

# Fix file permissions
chown dvarpala:dvarpala "$ENV_FILE"
chmod 600 "$ENV_FILE"

# Restart Dvarpala service
echo -e "${BLUE}[INFO]${NC} Restarting Dvarpala service..."
systemctl restart dvarpala

if systemctl is-active --quiet dvarpala; then
    echo -e "${GREEN}✅${NC} Dvarpala service restarted successfully"
else
    echo -e "${RED}❌${NC} Failed to restart Dvarpala service"
    echo "Check logs: journalctl -u dvarpala -f"
fi

echo
echo "=================================================================="
echo -e "${GREEN}✅ OAuth Configuration Complete!${NC}"
echo "=================================================================="
echo
echo "🌐 Web Interface: http://$SERVER_IP:$WEB_PORT"
echo "📁 Configuration file: $ENV_FILE"
echo
echo "You can now test OAuth authentication by visiting the web interface."