#!/bin/bash
# Dvarpala Captive Portal - OAuth Provider Firewall Rules
# This script configures iptables to allow access to OAuth providers
# while blocking general internet access for unauthenticated users

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}Configuring OAuth provider access rules...${NC}"

# Create custom chains for better organization
iptables -t nat -N DVARPALA_CAPTIVE 2>/dev/null || true
iptables -t filter -N DVARPALA_AUTH 2>/dev/null || true
iptables -t filter -N DVARPALA_OAUTH 2>/dev/null || true

# Mark packets from unauthenticated users (those in 172.30.100.0/24 subnet)
iptables -t mangle -A PREROUTING -s 172.30.100.0/24 -j MARK --set-mark 0x100

# === CAPTIVE PORTAL ACCESS ===
echo -e "${YELLOW}Setting up captive portal access...${NC}"
# Allow access to captive portal
iptables -A DVARPALA_AUTH -d 172.30.100.1 -p tcp --dport 8080 -j ACCEPT
iptables -A DVARPALA_AUTH -d 172.30.100.1 -p tcp --dport 443 -j ACCEPT

# === DNS RESOLUTION ===
echo -e "${YELLOW}Allowing DNS resolution...${NC}"
# Allow DNS queries (essential for OAuth domain resolution)
iptables -A DVARPALA_AUTH -p udp --dport 53 -j ACCEPT
iptables -A DVARPALA_AUTH -p tcp --dport 53 -j ACCEPT

# === MICROSOFT 365 / AZURE AD OAUTH ===
echo -e "${YELLOW}Configuring Microsoft 365 OAuth access...${NC}"
# Microsoft login endpoints
iptables -A DVARPALA_OAUTH -d login.microsoftonline.com -j ACCEPT
iptables -A DVARPALA_OAUTH -d login.microsoft.com -j ACCEPT
iptables -A DVARPALA_OAUTH -d login.live.com -j ACCEPT
iptables -A DVARPALA_OAUTH -d login.windows.net -j ACCEPT
iptables -A DVARPALA_OAUTH -d graph.microsoft.com -j ACCEPT
iptables -A DVARPALA_OAUTH -d account.microsoft.com -j ACCEPT

# Azure AD endpoints
iptables -A DVARPALA_OAUTH -d aadcdn.msauth.net -j ACCEPT
iptables -A DVARPALA_OAUTH -d aadcdn.msftauth.net -j ACCEPT
iptables -A DVARPALA_OAUTH -d msauth.net -j ACCEPT
iptables -A DVARPALA_OAUTH -d msftauth.net -j ACCEPT

# Microsoft IP ranges (primary authentication servers)
iptables -A DVARPALA_OAUTH -d 20.190.128.0/18 -j ACCEPT  # Azure AD
iptables -A DVARPALA_OAUTH -d 40.126.0.0/18 -j ACCEPT    # Microsoft Login
iptables -A DVARPALA_OAUTH -d 13.107.6.0/24 -j ACCEPT    # Microsoft services
iptables -A DVARPALA_OAUTH -d 13.107.9.0/24 -j ACCEPT    # Microsoft services

# === GOOGLE / GSUITE OAUTH ===
echo -e "${YELLOW}Configuring Google OAuth access...${NC}"
# Google OAuth endpoints
iptables -A DVARPALA_OAUTH -d accounts.google.com -j ACCEPT
iptables -A DVARPALA_OAUTH -d oauth2.googleapis.com -j ACCEPT
iptables -A DVARPALA_OAUTH -d www.googleapis.com -j ACCEPT
iptables -A DVARPALA_OAUTH -d ssl.gstatic.com -j ACCEPT
iptables -A DVARPALA_OAUTH -d apis.google.com -j ACCEPT
iptables -A DVARPALA_OAUTH -d myaccount.google.com -j ACCEPT

# Google authentication support domains
iptables -A DVARPALA_OAUTH -d accounts.youtube.com -j ACCEPT
iptables -A DVARPALA_OAUTH -d accounts.google.co.in -j ACCEPT
iptables -A DVARPALA_OAUTH -d accounts.google.co.uk -j ACCEPT

# Google IP ranges (primary blocks)
iptables -A DVARPALA_OAUTH -d 172.217.0.0/16 -j ACCEPT   # Google primary
iptables -A DVARPALA_OAUTH -d 172.253.0.0/16 -j ACCEPT   # Google secondary
iptables -A DVARPALA_OAUTH -d 142.250.0.0/15 -j ACCEPT   # Google services
iptables -A DVARPALA_OAUTH -d 74.125.0.0/16 -j ACCEPT    # Google services

# === GITHUB OAUTH ===
echo -e "${YELLOW}Configuring GitHub OAuth access...${NC}"
# GitHub OAuth endpoints
iptables -A DVARPALA_OAUTH -d github.com -j ACCEPT
iptables -A DVARPALA_OAUTH -d api.github.com -j ACCEPT
iptables -A DVARPALA_OAUTH -d gist.github.com -j ACCEPT
iptables -A DVARPALA_OAUTH -d github.githubassets.com -j ACCEPT
iptables -A DVARPALA_OAUTH -d avatars.githubusercontent.com -j ACCEPT
iptables -A DVARPALA_OAUTH -d camo.githubusercontent.com -j ACCEPT

# GitHub IP ranges
iptables -A DVARPALA_OAUTH -d 140.82.112.0/20 -j ACCEPT  # GitHub primary
iptables -A DVARPALA_OAUTH -d 192.30.252.0/22 -j ACCEPT  # GitHub legacy
iptables -A DVARPALA_OAUTH -d 185.199.108.0/22 -j ACCEPT # GitHub Pages

# === GITLAB OAUTH ===
echo -e "${YELLOW}Configuring GitLab OAuth access...${NC}"
# GitLab.com OAuth endpoints
iptables -A DVARPALA_OAUTH -d gitlab.com -j ACCEPT
iptables -A DVARPALA_OAUTH -d about.gitlab.com -j ACCEPT
iptables -A DVARPALA_OAUTH -d gitlab-static.net -j ACCEPT

# GitLab IP ranges (GitLab.com)
iptables -A DVARPALA_OAUTH -d 35.231.145.151/32 -j ACCEPT  # GitLab.com primary
iptables -A DVARPALA_OAUTH -d 34.74.90.64/28 -j ACCEPT     # GitLab.com range
iptables -A DVARPALA_OAUTH -d 34.74.226.0/24 -j ACCEPT    # GitLab.com range

# === CDN AND SUPPORT SERVICES ===
echo -e "${YELLOW}Configuring CDN and support services...${NC}"
# Common CDNs used by OAuth providers
iptables -A DVARPALA_OAUTH -d ajax.googleapis.com -j ACCEPT
iptables -A DVARPALA_OAUTH -d fonts.googleapis.com -j ACCEPT
iptables -A DVARPALA_OAUTH -d fonts.gstatic.com -j ACCEPT

# Certificate validation (OCSP)
iptables -A DVARPALA_OAUTH -d ocsp.pki.goog -j ACCEPT
iptables -A DVARPALA_OAUTH -d ocsp.digicert.com -j ACCEPT
iptables -A DVARPALA_OAUTH -d ocsp.msocsp.com -j ACCEPT

# === APPLY RULES TO FORWARD CHAIN ===
echo -e "${YELLOW}Applying rules to main chains...${NC}"
# Insert rules for marked packets (unauthenticated users)
iptables -A FORWARD -m mark --mark 0x100 -j DVARPALA_AUTH
iptables -A FORWARD -m mark --mark 0x100 -j DVARPALA_OAUTH

# Log blocked attempts (optional - for debugging)
iptables -A FORWARD -m mark --mark 0x100 -m limit --limit 1/min -j LOG --log-prefix "[DVARPALA-BLOCKED] "

# Drop all other traffic from unauthenticated users
iptables -A FORWARD -m mark --mark 0x100 -j DROP

# === NAT RULES FOR CAPTIVE PORTAL REDIRECT ===
echo -e "${YELLOW}Setting up captive portal redirects...${NC}"
# Redirect HTTP traffic to captive portal (except for allowed domains)
iptables -t nat -A PREROUTING -m mark --mark 0x100 -p tcp --dport 80 \
    -j DNAT --to-destination 172.30.100.1:8080

# Redirect HTTPS to captive portal (this will show cert warning)
iptables -t nat -A PREROUTING -m mark --mark 0x100 -p tcp --dport 443 \
    -j DNAT --to-destination 172.30.100.1:8080

# === SAVE RULES ===
echo -e "${YELLOW}Saving iptables rules...${NC}"
# Save rules (varies by distribution)
if command -v netfilter-persistent &> /dev/null; then
    netfilter-persistent save
elif command -v iptables-save &> /dev/null; then
    iptables-save > /etc/iptables/rules.v4
else
    echo -e "${RED}Warning: Could not save iptables rules automatically${NC}"
fi

echo -e "${GREEN}OAuth firewall rules configured successfully!${NC}"
echo -e "${GREEN}Authenticated users will have full internet access.${NC}"
echo -e "${GREEN}Unauthenticated users can only access:${NC}"
echo -e "  - Captive portal (172.30.100.1:8080)"
echo -e "  - Microsoft 365 / Azure AD OAuth"
echo -e "  - Google / GSuite OAuth"
echo -e "  - GitHub OAuth"
echo -e "  - GitLab OAuth"
echo -e "  - Required CDNs and certificate services"

# === HELPER FUNCTIONS ===
# Function to add authenticated user (call after successful OAuth)
cat << 'EOF' > /usr/local/bin/dvarpala-auth-user
#!/bin/bash
# Usage: dvarpala-auth-user <client-ip>
CLIENT_IP=$1
if [ -z "$CLIENT_IP" ]; then
    echo "Usage: $0 <client-ip>"
    exit 1
fi

# Remove the unauthenticated mark for this specific IP
iptables -t mangle -I PREROUTING -s $CLIENT_IP -j MARK --set-mark 0x0
echo "User $CLIENT_IP authenticated - full internet access granted"
EOF

chmod +x /usr/local/bin/dvarpala-auth-user

echo -e "\n${GREEN}Helper script created: /usr/local/bin/dvarpala-auth-user${NC}"
echo -e "Use it to grant full access after OAuth: dvarpala-auth-user <client-ip>"