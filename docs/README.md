# Dvarpala VPN Documentation

This directory contains comprehensive documentation for the Dvarpala 2-Step VPN system.

## =Ú Documentation Index

### =€ Getting Started
- **[INSTALLATION.md](../INSTALLATION.md)** - Complete installation guide for production servers
- **[oauth-setup-guide.md](oauth-setup-guide.md)** - OAuth provider configuration guide

### = 2-Step VPN Architecture

Dvarpala implements a Zero Trust Network Access (ZTNA) architecture with 2-step authentication:

#### Step 1: Initial VPN Connection
- Users connect with **temporary certificates**
- Gain access to **captive portal network** (192.168.100.0/24)
- Can only access Dvarpala authentication portal at `http://192.168.100.1:8080`

#### Step 2: OAuth Authentication
- Users authenticate via configured OAuth providers:
  - **Google/Gmail**
  - **Microsoft**
  - **GitHub** 
  - **GitLab**
- System validates user exists in database
- Upon successful authentication, user is promoted to **full access network** (10.8.0.0/24)

#### Session Management
- **No permanent access** - certificates revoked on disconnect
- **Re-authentication required** for each VPN session
- **Admin bypass** - permanent certificates for administrators

### =á Security Features

#### Zero Trust Principles
- **Never trust, always verify** - Every connection requires authentication
- **Least privilege access** - Users start with minimal network access
- **Dynamic access control** - Network promotion based on real-time authentication

#### Certificate Management
- **Temporary certificates** with automatic expiration and revocation
- **Dynamic generation** per user request
- **Certificate Revocation List (CRL)** for immediate access termination
- **Secure certificate distribution** via administrator

#### Network Segmentation
- **Captive Portal Network**: 192.168.100.0/24
  - Only allows access to authentication portal
  - DNS resolution limited to portal server
  - No internet access until authenticated
- **Full Access Network**: 10.8.0.0/24
  - Complete internet access after authentication
  - Full DNS resolution
  - All ports and protocols available

### =' Administrative Guide

#### User Provisioning Process
1. **Add user to database** (via admin interface or direct database)
2. **Generate temporary certificate**:
   ```bash
   sudo /opt/dvarpala/bin/generate-temp-cert user@example.com
   ```
3. **Distribute .ovpn file** to user securely
4. **User follows connection process** (connect ’ authenticate ’ access)

#### Certificate Lifecycle
```bash
# Generate certificate
sudo /opt/dvarpala/bin/generate-temp-cert user@example.com

# Certificate automatically revoked on disconnect via:
# - client-disconnect script
# - Certificate Revocation List update
# - Session cleanup

# Manual revocation (if needed)
sudo /opt/dvarpala/bin/revoke-temp-cert user@example.com
```

#### Monitoring and Logging
- **OpenVPN logs**: `/var/log/openvpn/`
- **Application logs**: `journalctl -u dvarpala`
- **Authentication logs**: `/var/log/openvpn/auth.log`
- **Connection tracking**: `/var/log/openvpn/client-connect.log`

### < Network Architecture

#### Firewall Configuration
- **Captive Portal Restrictions**:
  - Only HTTP/HTTPS to 192.168.100.1:8080
  - DNS queries to 192.168.100.1
  - All other traffic blocked
- **Full Access Promotion**:
  - Unrestricted internet access
  - All ports and protocols
  - Standard DNS resolution

#### OpenVPN Configuration
- **Port**: 1194 (UDP)
- **Encryption**: AES-256-GCM
- **Authentication**: SHA-256
- **TLS Version**: 1.2+
- **Certificate Authority**: RSA 2048-bit
- **Session Timeout**: 1 hour (configurable)

### = OAuth Integration

#### Supported Providers
1. **Google/Gmail**
   - OAuth 2.0 with OpenID Connect
   - Requires Google Cloud Console setup
   - Scopes: `openid`, `email`, `profile`

2. **Microsoft**
   - Azure AD OAuth 2.0
   - Supports both personal and organizational accounts
   - Requires Azure App Registration

3. **GitHub**
   - OAuth 2.0
   - Requires GitHub App creation
   - Scopes: `user:email`

4. **GitLab**
   - OAuth 2.0
   - Supports both GitLab.com and self-hosted instances
   - Scopes: `read_user`

#### OAuth Flow
1. User clicks provider button on captive portal
2. Redirect to provider authorization endpoint
3. User authenticates with provider
4. Provider redirects back with authorization code
5. Dvarpala exchanges code for access token
6. Retrieves user information from provider
7. Validates user exists in Dvarpala database
8. Grants network access if authorized

### =¨ Troubleshooting

#### Common Issues

**Users can't access authentication portal**
```bash
# Check captive portal network configuration
grep -A 10 "192.168.100" /etc/openvpn/server.conf

# Verify firewall rules
sudo iptables -L -n | grep 192.168.100

# Check client-connect script
tail -f /var/log/openvpn/client-connect.log
```

**OAuth authentication fails**
```bash
# Check OAuth provider configuration
sudo cat /opt/dvarpala/config/.oauth_credentials

# Check application logs
journalctl -u dvarpala -f

# Verify database connectivity
sudo -u postgres psql dvarpala -c "SELECT * FROM oauth_providers;"
```

**Users don't get promoted to full access**
```bash
# Check authentication API
curl "http://localhost:8080/api/internal/check-auth/username"

# Verify user exists in database
sudo -u postgres psql dvarpala -c "SELECT * FROM users WHERE email='user@example.com';"

# Check client-connect script execution
tail -f /var/log/openvpn/client-connect.log
```

#### Log Locations
- **Main application**: `journalctl -u dvarpala`
- **OpenVPN server**: `/var/log/openvpn/openvpn.log`
- **Client connections**: `/var/log/openvpn/client-connect.log`
- **Client disconnections**: `/var/log/openvpn/client-disconnect.log`
- **Authentication attempts**: `/var/log/openvpn/auth.log`

### =Ê Monitoring and Metrics

#### Key Metrics to Monitor
- **Active VPN connections** by network (captive vs full access)
- **Authentication success/failure rates** by OAuth provider
- **Certificate generation and revocation** frequency
- **Session duration** and patterns
- **Failed authentication attempts** for security monitoring

#### Health Checks
```bash
# System status check
sudo /opt/dvarpala/scripts/check-status.sh

# Service status
sudo systemctl status dvarpala openvpn-server postgresql redis

# Network connectivity
ping -c 1 192.168.100.1  # Captive portal
ping -c 1 10.8.0.1       # Full access gateway
```

### = Maintenance and Updates

#### Regular Maintenance Tasks
- **Certificate Revocation List** updates (automatic)
- **Database cleanup** of expired sessions
- **Log rotation** for OpenVPN and application logs
- **OAuth token refresh** (automatic)

#### Backup Procedures
```bash
# Database backup
sudo -u postgres pg_dump dvarpala > dvarpala-backup-$(date +%Y%m%d).sql

# Certificate authority backup
sudo tar -czf ca-backup-$(date +%Y%m%d).tar.gz /etc/openvpn/easy-rsa/pki/

# Configuration backup
sudo tar -czf config-backup-$(date +%Y%m%d).tar.gz /opt/dvarpala/config/
```

### <˜ Emergency Procedures

#### Lost Admin Access
```bash
# Via cloud provider console/serial access
sudo bash /opt/dvarpala/scripts/emergency-access.sh

# Options:
# 1. Restore SSH access from internet
# 2. Display admin VPN configuration
# 3. Generate new admin certificate
```

#### System Recovery
```bash
# Restore from backup
sudo systemctl stop dvarpala openvpn-server
sudo -u postgres psql dvarpala < dvarpala-backup.sql
sudo tar -xzf config-backup.tar.gz -C /
sudo systemctl start postgresql redis openvpn-server dvarpala
```

---

## =Þ Support and Community

- **GitHub Issues**: [Report bugs and feature requests](https://github.com/yourcompany/dvarpala/issues)
- **Installation Guide**: [INSTALLATION.md](../INSTALLATION.md)
- **OAuth Setup**: [oauth-setup-guide.md](oauth-setup-guide.md)
- **Provisioning Scripts**: [scripts/provisioning/README.md](../scripts/provisioning/README.md)

---

*Dvarpala (&M5>0*>2) - Your trusted gatekeeper for Zero Trust VPN access*