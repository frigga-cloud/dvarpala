#!/bin/bash

# Dvarpala Emergency Access Recovery Script
# Use this script via console/serial access if you lose VPN connectivity

set -euo pipefail

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo "=================================================================="
echo -e "${RED}🆘 Dvarpala Emergency Access Recovery${NC}"
echo "=================================================================="
echo

# Check if running as root
if [[ $EUID -ne 0 ]]; then
    echo -e "${RED}[ERROR]${NC} This script must be run as root"
    exit 1
fi

echo -e "${YELLOW}⚠️  Emergency Access Options:${NC}"
echo "1. Restore SSH access from internet"
echo "2. Display admin VPN configuration"
echo "3. Reset to temporary open access (5 minutes)"
echo "4. Check VPN server status"
echo "5. Exit"
echo

while true; do
    read -p "Select option (1-5): " choice
    case $choice in
        1)
            echo -e "${BLUE}[INFO]${NC} Restoring SSH access from internet..."
            
            # Restore SSH config
            if [[ -f /etc/ssh/sshd_config.backup ]]; then
                cp /etc/ssh/sshd_config.backup /etc/ssh/sshd_config
                systemctl restart sshd
                echo -e "${GREEN}✅${NC} SSH config restored"
            else
                echo -e "${RED}❌${NC} Backup SSH config not found"
            fi
            
            # Restore firewall rules
            if command -v ufw >/dev/null 2>&1; then
                ufw allow ssh
                echo -e "${GREEN}✅${NC} UFW SSH rule restored"
            elif command -v firewall-cmd >/dev/null 2>&1; then
                firewall-cmd --permanent --add-service=ssh
                firewall-cmd --reload
                echo -e "${GREEN}✅${NC} Firewalld SSH rule restored"
            fi
            
            echo -e "${GREEN}✅${NC} SSH access restored from internet"
            echo -e "${YELLOW}⚠️  Remember to secure the server again after fixing VPN issues"
            break
            ;;
        2)
            echo -e "${BLUE}[INFO]${NC} Displaying admin VPN configuration..."
            echo
            
            if [[ -f /opt/dvarpala/certs/admin-backup.ovpn ]]; then
                echo "=================================================================="
                echo -e "${GREEN}Admin OpenVPN Configuration:${NC}"
                echo "=================================================================="
                cat /opt/dvarpala/certs/admin-backup.ovpn
                echo "=================================================================="
                echo
                echo "Copy the configuration above and save as 'dvarpala-admin.ovpn'"
                echo "Temporary credentials: temp_user / temp_portal_access"
            else
                echo -e "${RED}❌${NC} Admin VPN configuration not found"
            fi
            ;;
        3)
            echo -e "${BLUE}[INFO]${NC} Setting temporary open access for 5 minutes..."
            
            # Temporarily allow SSH
            if command -v ufw >/dev/null 2>&1; then
                ufw allow ssh
            elif command -v firewall-cmd >/dev/null 2>&1; then
                firewall-cmd --add-service=ssh
            fi
            
            echo -e "${GREEN}✅${NC} Temporary SSH access enabled"
            echo -e "${YELLOW}⚠️  Access will be automatically revoked in 5 minutes"
            
            # Schedule removal
            (
                sleep 300  # 5 minutes
                if command -v ufw >/dev/null 2>&1; then
                    ufw deny 22
                elif command -v firewall-cmd >/dev/null 2>&1; then
                    firewall-cmd --remove-service=ssh
                fi
                echo "Temporary access revoked" | logger
            ) &
            
            echo "You have 5 minutes to fix the VPN configuration"
            ;;
        4)
            echo -e "${BLUE}[INFO]${NC} Checking VPN server status..."
            echo
            
            echo "OpenVPN Server Status:"
            systemctl status openvpn-server --no-pager || echo "OpenVPN server not running"
            echo
            
            echo "OpenVPN Log (last 10 lines):"
            if [[ -f /var/log/openvpn/openvpn.log ]]; then
                tail -n 10 /var/log/openvpn/openvpn.log
            else
                echo "No OpenVPN logs found"
            fi
            echo
            
            echo "Dvarpala Service Status:"
            systemctl status dvarpala --no-pager || echo "Dvarpala service not running"
            echo
            ;;
        5)
            echo "Exiting emergency recovery"
            exit 0
            ;;
        *)
            echo "Invalid option. Please select 1-5."
            ;;
    esac
    
    echo
    read -p "Press Enter to continue or Ctrl+C to exit..."
    echo
done