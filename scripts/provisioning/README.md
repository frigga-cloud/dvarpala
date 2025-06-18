# Dvarpala VPN Server Provisioning

This directory contains scripts for both **Cloud Installation** (recommended) and **Manual VM Provisioning** of Dvarpala 2-Step VPN servers.

## 🎯 Installation Methods

### 🚀 Method 1: Cloud Installer (Recommended)

**Deploy directly to AWS, GCP, or Azure from your laptop**

The Cloud Installer creates complete infrastructure and installs dvarpala automatically. This is the **preferred method** for new deployments.

#### Quick Start
```bash
# One-click installation
curl -fsSL https://raw.githubusercontent.com/yourcompany/dvarpala/main/scripts/installation/install-dvarpala.sh | bash
```

#### What It Does
1. **Infrastructure Creation**: Creates "frigga-labs" VPC, VM, security groups
2. **Automatic Installation**: Installs and configures dvarpala on new VM
3. **Object Storage**: Sets up S3/GCS/Azure Storage for backups
4. **Ready-to-Use**: Provides admin.ovpn and connection details

#### Output
```
dvarpala-deployment/
├── installation-config.json    # Full configuration
├── connection-info.txt         # Server details  
├── admin.ovpn                  # VPN configuration
└── ssh-key.pem                 # SSH access key
```

**See: [📖 Cloud Installer Documentation](../installation/README.md)**

---

### 🔧 Method 2: Manual VM Provisioning

**Install on existing virtual machine**

Use these scripts if you already have a VM and want to install dvarpala manually.

## 📖 Quick Start (Manual)

For complete installation guide with prerequisites and commands, see:
**[📋 INSTALLATION.md](../../INSTALLATION.md)** - Complete installation guide

## 🚀 Manual Installation Commands

### One-Line Installation (Recommended)

```bash
sudo bash <(curl -fsSL https://raw.githubusercontent.com/yourcompany/dvarpala/main/scripts/provisioning/quick-install.sh)
```

### Manual Installation

```bash
# Download and review the script first
curl -fsSL https://raw.githubusercontent.com/yourcompany/dvarpala/main/scripts/provisioning/setup-server.sh -o setup-server.sh
less setup-server.sh
sudo bash setup-server.sh
```

## 🔄 Complete Installation Flow

### **Phase 1: Customer Initiation**
1. Customer runs cloud installer from their laptop
2. Selects cloud provider (AWS/GCP/Azure)
3. Provides authentication credentials

### **Phase 2: Local Environment Setup**
1. **Platform Detection**: OS and architecture detection
2. **Dependency Installation**: Go runtime, cloud CLI tools
3. **Repository Download**: Clones dvarpala source
4. **Binary Compilation**: Builds cloud installer

### **Phase 3: Cloud Authentication**
1. **Credential Validation**: Tests cloud provider access
2. **Permission Check**: Verifies required permissions
3. **Region Selection**: Chooses deployment region

### **Phase 4: Infrastructure Provisioning**
1. **VPC Creation**: Creates/finds "frigga-labs" VPC
2. **Security Groups**: Configures firewall rules
3. **VM Launch**: Creates Ubuntu 22.04 instance
4. **Network Setup**: Public/private subnet configuration

### **Phase 5: Software Installation** (on VM)
1. **System Updates**: Updates packages and dependencies
2. **Database Setup**: PostgreSQL 15 with secure credentials
3. **Cache Setup**: Redis server configuration
4. **VPN Setup**: OpenVPN with 2-step authentication
5. **Application Build**: Compiles and installs dvarpala

### **Phase 6: Security Configuration**
1. **Certificate Generation**: Creates admin and CA certificates
2. **Firewall Rules**: Configures iptables and cloud security groups
3. **Service Hardening**: Enables systemd services
4. **SSH Restriction**: Limits SSH to VPN network

### **Phase 7: Object Storage Integration**
1. **Bucket Creation**: Creates S3/GCS/Azure storage
2. **Configuration Backup**: Uploads installation config
3. **Credential Storage**: Stores admin details securely

### **Phase 8: Ready-to-Use Output**
1. **File Generation**: admin.ovpn, connection details, SSH keys
2. **Summary Display**: Shows server IP, credentials, next steps
3. **Documentation**: Provides usage instructions

## ⚠️ IMPORTANT: 2-Step VPN Installation Process

The installation script follows these steps:
1. **Admin Configuration** - Collects administrator email for first user account
2. **System Installation** - Installs and configures all components with 2-step access
3. **OAuth Provider Setup** - Configures up to 2 OAuth providers (Google, Microsoft, GitHub, GitLab)
4. **Database Initialization** - Creates schema and seeds admin user + OAuth providers
5. **Security Phase** - Displays credentials and secures server
6. **SSH Lockdown** - Server only accessible via VPN after confirmation

## 📋 What Gets Installed

### Core Components
- **PostgreSQL 15** - Database server with dvarpala database and admin user
- **Redis 7** - Cache and session storage  
- **OpenVPN Server** - 2-step VPN server with dynamic certificate management
- **Dvarpala Application** - Captive portal web interface with OAuth integration
- **Go 1.21** - Runtime for Dvarpala application

### 2-Step VPN Security & Networking
- **Captive Portal Network** (192.168.100.0/24) - Initial limited access
- **Full Access Network** (10.8.0.0/24) - Post-authentication internet access
- **Firewall segmentation** - Captive portal restrictions and full access promotion
- **Dynamic certificate management** - Temporary certificates with automatic revocation
- **OAuth integration** - Multi-provider authentication (Google, Microsoft, GitHub, GitLab)
- **Session management** - Authentication tracking with disconnect cleanup

### System Services
All components are configured as systemd services for automatic startup:
- `postgresql.service`
- `redis.service`
- `openvpn-server.service`
- `dvarpala.service`

## 🔧 Post-Installation Usage

### 1. Admin Access (Immediate)

Use the admin certificate for direct VPN access:
```bash
# Admin .ovpn file location
/opt/dvarpala/certs/admin.ovpn

# Import this file into your OpenVPN client for permanent admin access
```

### 2. Generate User Certificates (For End Users)

Create temporary certificates for users:
```bash
# SSH into server via admin VPN
ssh root@192.168.100.1

# Generate temporary certificate for a user
sudo /opt/dvarpala/bin/generate-temp-cert user@example.com

# Send the generated .ovpn file to the user
# Location: /etc/openvpn/client-configs/user@example.com.ovpn
```

### 3. User Experience Flow

**For End Users:**
1. Receive temporary `.ovpn` file from administrator
2. Connect to VPN using OpenVPN client
3. Open browser and navigate to `http://192.168.100.1:8080`
4. Authenticate using configured OAuth provider
5. Gain full internet access automatically
6. Certificate revoked automatically on disconnect

### 4. System Status Verification

```bash
sudo bash /opt/dvarpala/scripts/check-status.sh
```

## 🌐 Default Configuration

### Network Configuration
- **OpenVPN Port**: 1194 (UDP) - 2-step access
- **Web Interface**: 8080 (TCP) - Captive portal and dashboard
- **Captive Portal Network**: 192.168.100.0/24 (initial limited access)
- **Full Access Network**: 10.8.0.0/24 (post-authentication internet)

### Access Methods
- **Admin Access**: Permanent certificates bypass 2-step process
- **User Access**: Temporary certificates → OAuth authentication → Full access
- **No Default Credentials**: Users authenticate via OAuth only

### File Locations
```
/opt/dvarpala/                    # Dvarpala home directory
├── bin/                          # Application binaries
│   ├── dvarpala-server           # Main 2-step VPN application
│   ├── generate-temp-cert        # Temporary certificate generator
│   ├── revoke-temp-cert          # Certificate revocation tool
│   ├── client-connect            # OpenVPN connect script
│   └── client-disconnect         # OpenVPN disconnect script
├── config/                       # Configuration files
│   ├── .env                      # Environment variables
│   ├── .db_credentials           # Database credentials
│   ├── .oauth_credentials        # OAuth provider settings
│   ├── seed_admin_user.sql       # Admin user seeding
│   └── seed_oauth_providers.sql  # OAuth provider seeding
├── certs/                        # Certificates and keys
│   └── admin.ovpn               # Admin VPN profile (permanent)
├── logs/                         # Application logs
└── data/                         # Application data

/etc/openvpn/                     # OpenVPN 2-step configuration
├── server.conf                   # 2-step server configuration
├── temp-certs/                   # Temporary user certificates
├── client-configs/               # Generated .ovpn files for users
└── easy-rsa/                     # Certificate authority with CRL
```

## 🛠️ Management Commands

### Service Management
```bash
# Check all service status
sudo systemctl status dvarpala openvpn-server postgresql redis

# Restart services
sudo systemctl restart dvarpala
sudo systemctl restart openvpn-server

# View logs
journalctl -u dvarpala -f
journalctl -u openvpn-server -f
tail -f /var/log/openvpn/openvpn.log
```

### Emergency Access
If you lose VPN access and need to recover:

```bash
# Via console/serial access (cloud provider console)
sudo bash /opt/dvarpala/scripts/emergency-access.sh
```

This script provides options to:
- Restore SSH access from internet
- Display admin VPN configuration
- Temporarily enable access for recovery

### 2-Step VPN Certificate Management
```bash
# Generate temporary certificate for user (recommended method)
sudo /opt/dvarpala/bin/generate-temp-cert user@example.com

# Manually revoke user certificate
sudo /opt/dvarpala/bin/revoke-temp-cert user@example.com

# View active temporary certificates
ls -la /etc/openvpn/temp-certs/

# View generated user configurations
ls -la /etc/openvpn/client-configs/

# Manual certificate management (advanced)
cd /etc/openvpn/easy-rsa
./easyrsa gen-req username nopass
./easyrsa sign-req client username
./easyrsa revoke username
./easyrsa gen-crl
```

### Database Management
```bash
# Connect to database
sudo -u postgres psql dvarpala

# View database credentials
sudo cat /opt/dvarpala/config/.db_credentials

# Backup database
sudo -u postgres pg_dump dvarpala > dvarpala-backup.sql
```

## 🔒 Security Considerations

### Firewall Ports
The following ports are opened by default:
- **22** - SSH access
- **1194** - OpenVPN server (UDP)
- **8080** - Dvarpala web interface (TCP)

### SSL/TLS Configuration
- OpenVPN uses AES-256-GCM encryption
- 2048-bit RSA certificates with SHA-256 signatures
- TLS 1.2+ minimum version
- Perfect Forward Secrecy enabled

### Database Security
- PostgreSQL configured for local access only
- Dedicated `dvarpala` database user with limited privileges
- Random password generation
- Credentials stored in protected file (600 permissions)

## 🐛 Troubleshooting

### Common Issues

#### Services Won't Start
```bash
# Check service status
sudo systemctl status dvarpala
sudo systemctl status openvpn-server

# Check logs for errors
journalctl -u dvarpala --no-pager
journalctl -u openvpn-server --no-pager
```

#### VPN Connection Issues
```bash
# Check OpenVPN server status
sudo systemctl status openvpn-server

# Check OpenVPN logs
tail -f /var/log/openvpn/openvpn.log

# Check firewall rules
sudo ufw status
sudo iptables -L -n
```

#### Database Connection Issues
```bash
# Test PostgreSQL connection
sudo -u postgres psql -c "SELECT version();"

# Check if dvarpala database exists
sudo -u postgres psql -l | grep dvarpala

# Test application database connection
sudo -u dvarpala psql -h localhost -d dvarpala -c "SELECT 1;"
```

#### Web Interface Issues
```bash
# Check if Dvarpala service is running
sudo systemctl status dvarpala

# Check application logs
journalctl -u dvarpala -f

# Test web server directly
curl -I http://localhost:8080
```

### Log Locations
- **Dvarpala Application**: `journalctl -u dvarpala`
- **OpenVPN Server**: `/var/log/openvpn/openvpn.log`
- **PostgreSQL**: `/var/log/postgresql/`
- **System**: `/var/log/syslog` or `journalctl`

## 🌍 Cloud Provider Integration

### Object Storage Backup Locations
- **AWS S3**: `s3://frigga-labs-{suffix}/dvarpala/`
- **Google Cloud Storage**: `gs://frigga-labs-{suffix}/dvarpala/`
- **Azure Blob Storage**: `https://{account}.blob.core.windows.net/dvarpala/`

### VPC Configuration
- **VPC Name**: `frigga-labs` (shared across Frigga Labs tools)
- **CIDR Range**: `10.0.0.0/16` (configurable)
- **Public Subnet**: `10.0.1.0/24` (dvarpala instances)
- **Private Subnet**: `10.0.2.0/24` (future use)

### Instance Types
- **AWS**: t3.small/medium/large
- **GCP**: e2-small/medium/standard-2
- **Azure**: Standard_B1ms/B2s/B2ms

## 📚 Additional Resources

### Documentation
- [📖 Cloud Installer Guide](../installation/README.md)
- [📋 Complete Installation Guide](../../INSTALLATION.md)
- [🔧 User Guide](../../docs/user-guide.md)
- [🔌 API Documentation](../../docs/api.md)
- [🗄️ Database Schema](../../database-readme.md)

### Support
- GitHub Issues: https://github.com/yourcompany/dvarpala/issues
- Documentation: https://dvarpala.readthedocs.io
- Community Forum: https://community.dvarpala.org

## 🔄 Updates and Maintenance

### Updating Dvarpala
```bash
# Download latest version (when available)
sudo systemctl stop dvarpala
# Replace binaries with updated versions
sudo systemctl start dvarpala
```

### Certificate Renewal
OpenVPN certificates are valid for 10 years by default. To renew:
```bash
cd /etc/openvpn/easy-rsa
./easyrsa renew server nopass
sudo systemctl restart openvpn-server
```

### Database Backup
```bash
# Create backup
sudo -u postgres pg_dump dvarpala | gzip > dvarpala-backup-$(date +%Y%m%d).sql.gz

# Restore backup
sudo -u postgres psql dvarpala < dvarpala-backup.sql
```

---

**🚀 For the fastest and most reliable installation, use the Cloud Installer method.**

For the latest version of these scripts and documentation, visit:
https://github.com/yourcompany/dvarpala/tree/main/scripts/provisioning