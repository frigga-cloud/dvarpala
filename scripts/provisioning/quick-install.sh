#!/bin/bash

# Dvarpala VPN Quick Install Script
# One-liner installation for Dvarpala VPN Server
# Usage: bash <(curl -fsSL https://raw.githubusercontent.com/yourcompany/dvarpala/main/scripts/provisioning/quick-install.sh)

set -euo pipefail

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo "=================================================================="
echo -e "${BLUE}🚀 Dvarpala VPN Server Quick Installer${NC}"
echo "=================================================================="
echo

# Check if running as root
if [[ $EUID -ne 0 ]]; then
    echo -e "${RED}[ERROR]${NC} This script must be run as root"
    echo "Please run: sudo bash <(curl -fsSL https://raw.githubusercontent.com/yourcompany/dvarpala/main/scripts/provisioning/quick-install.sh)"
    exit 1
fi

# Confirm installation
echo -e "${YELLOW}⚠️  This will install and configure:${NC}"
echo "   • PostgreSQL 15 database server"
echo "   • Redis cache server"
echo "   • OpenVPN server with certificates"
echo "   • Dvarpala VPN management application"
echo "   • Firewall rules and systemd services"
echo
read -p "Do you want to continue? (y/N): " -r
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Installation cancelled."
    exit 0
fi

echo
echo -e "${BLUE}[INFO]${NC} Downloading and running setup script..."

# Download and execute the main setup script
curl -fsSL https://raw.githubusercontent.com/yourcompany/dvarpala/main/scripts/provisioning/setup-server.sh | bash

echo
echo -e "${GREEN}✅ Quick installation completed!${NC}"
echo "Check the output above for connection details and next steps."