# Dvarpala VPN Server Installation

Dvarpala now supports two installation methods: **Cloud Installer** (recommended) and **Manual VM Installation**.

## 🔐 2-Step VPN Access Overview

Dvarpala implements a Zero Trust Network Access (ZTNA) architecture with 2-step authentication:

1. **Initial Connection**: Users connect to OpenVPN and receive temporary access to a captive portal network
2. **Authentication**: Users access the Dvarpala dashboard (only accessible page) and authenticate via OAuth
3. **Full Access**: After successful authentication, users are granted full VPN access with internet connectivity
4. **Session Management**: Users must re-authenticate each time they disconnect and reconnect

## 🎯 Installation Methods

### Method 1: Cloud Installer (Recommended)

The **Cloud Installer** creates infrastructure and installs dvarpala directly from your laptop to AWS, GCP, or Azure.

#### Prerequisites
- **Local Machine**: Go 1.19+, Git, Internet connection
- **Cloud Account**: AWS, GCP, or Azure with billing enabled
- **Cloud CLI**: Appropriate CLI tool installed (aws, gcloud, or az)

#### Quick Start
```bash
# Clone repository
git clone https://github.com/yourcompany/dvarpala.git
cd dvarpala/scripts/installation

# Interactive installation
go run cloud-installer.go
```

The installer will:
1. **Authenticate** with your chosen cloud provider (AWS/GCP/Azure)
2. **Create VPC** named "frigga-labs" with minimal configuration (4-5 server limit)
3. **Launch VM** and install dvarpala automatically
4. **Setup object storage** for configuration backups
5. **Provide connection details** including admin.ovpn file

#### Configuration Options

**AWS Example:**
```bash
go run cloud-installer.go -provider=aws -region=us-east-1 -interactive=false
```

**Using Config File:**
```bash
cp examples/config-aws.json my-config.json
# Edit my-config.json with your credentials
go run cloud-installer.go -config=my-config.json -interactive=false
```

**What Gets Created:**
- **VPC**: `frigga-labs` (shared across Frigga Labs tools)
- **VM**: Ubuntu 22.04 with dvarpala pre-installed
- **Object Storage**: S3/GCS/Azure Storage for backups
- **Security Groups**: Minimal required ports (22, 1194, 8080, 443)
- **Admin Certificate**: Ready-to-use .ovpn file

**Output Files:**
```
dvarpala-deployment/
├── installation-config.json    # Full configuration
├── connection-info.txt         # Server details
├── admin.ovpn                  # VPN configuration
└── ssh-key.pem                 # SSH access key
```

See [Cloud Installer Documentation](scripts/installation/README.md) for detailed configuration options.

---

### Method 2: Manual VM Installation

Install dvarpala on an existing virtual machine.

#### Prerequisites
- **Fresh VM** with root access (Ubuntu 20.04+, Debian 11+, CentOS 8+, RHEL 8+, Rocky Linux 8+, or AlmaLinux 8+)
- **Minimum 2 GB RAM** (4 GB recommended)
- **20 GB disk space** (50 GB recommended for logs and data)
- **1 CPU core** (2+ cores recommended)
- **Public IP address** with internet connectivity

#### Network Requirements
- **Outbound internet access** for downloading packages
- **Inbound ports** that will be opened:
  - `22` - SSH access (will be restricted to VPN after installation)
  - `1194` - OpenVPN server (UDP) - 2-step access
  - `8080` - Dvarpala web interface (TCP) - captive portal + full access

#### Client Requirements
- **OpenVPN client** installed on your local machine
- **SSH client** for initial server access
- **Web browser** for accessing the management interface

## 🚀 Manual Installation Commands

### One-Line Installation (Recommended)

```bash
sudo bash <(curl -fsSL https://raw.githubusercontent.com/yourcompany/dvarpala/main/scripts/provisioning/quick-install.sh)
```

### Manual Installation

```bash
# Download the installation script
curl -fsSL https://raw.githubusercontent.com/yourcompany/dvarpala/main/scripts/provisioning/setup-server.sh -o setup-server.sh

# Review the script (recommended)
less setup-server.sh

# Run the installation
sudo bash setup-server.sh
```

## 🛠️ What Gets Installed

### Core Applications
- **PostgreSQL 15** - Primary database with `dvarpala` database and user
- **Redis 7** - Cache and session storage
- **OpenVPN Server** - VPN server with Easy-RSA certificate authority
- **Dvarpala Application** - VPN management web interface and API
- **Go 1.21** - Runtime environment for the Dvarpala application

### System Components
- **Firewall Configuration** - UFW (Ubuntu/Debian) or Firewalld (CentOS/RHEL)
- **IP Forwarding** - Kernel configuration for VPN traffic routing
- **NAT/Masquerading** - Network address translation for internet access
- **SSL Certificates** - 2048-bit RSA certificates for OpenVPN (10-year validity)
- **Systemd Services** - Auto-starting services for all components

### Security Features
- **Hardened SSH** - Restricted to VPN network access only (after installation)
- **Encrypted VPN** - AES-256-GCM encryption with SHA-256 authentication
- **Random Passwords** - Automatically generated secure database passwords
- **User Isolation** - Dedicated `dvarpala` system user for application processes

## ⚠️ Important Installation Notes

### Installation Process
The installation follows a comprehensive setup process:

1. **Admin Configuration**: Collects administrator email for first user account
2. **System Setup**: Installs and configures all components
3. **OAuth Configuration**: Interactive setup for authentication providers
4. **Database Initialization**: Creates schema and seeds initial data
5. **Security Phase**: Displays credentials and secures the server

#### Administrator Setup
During installation, you'll be prompted to provide:
- **Administrator Email**: Email address for the first administrative user
- **Full Name**: Optional display name (defaults to email username if not provided)

#### OAuth Provider Selection
During installation, you'll be prompted to configure OAuth providers:
- **Supported Providers**: Google/Gmail, Microsoft, GitHub, GitLab
- **Maximum Providers**: Up to 2 providers can be enabled
- **Required Information**: Client ID, Client Secret, and redirect URLs
- **Provider Setup**: You'll need to create OAuth applications beforehand

**CRITICAL**: During the security phase, you must:
- Copy the database password shown on screen
- Copy the complete `.ovpn` file configuration
- Confirm you have saved all credentials before proceeding

After confirmation, **SSH access will be blocked from the internet** and only accessible via VPN.

### Access Credentials
After installation, you'll receive:

- **Database Password**: Randomly generated secure password for PostgreSQL
- **VPN Temporary Credentials**: `temp_user` / `temp_portal_access`
- **Admin .ovpn File**: Complete OpenVPN client configuration with embedded certificates

## 🔧 Post-Installation Steps

### 1. Understanding 2-Step Access

**For End Users (Regular VPN Access):**
Users will need to:
1. Request a temporary OpenVPN configuration from the administrator
2. Connect to VPN using their temporary certificate 
3. Open browser and navigate to `http://192.168.100.1:8080`
4. Authenticate via OAuth (Google, Microsoft, GitHub, or GitLab)
5. Receive full internet access after successful authentication

**For Administrators:**
Use the permanent admin certificate for direct access.

### 2. Generate User Certificates
```bash
# SSH into the server via VPN (admin access)
ssh root@192.168.100.1

# Generate temporary certificate for a user
sudo /opt/dvarpala/bin/generate-temp-cert username@example.com

# The .ovpn file will be created in /etc/openvpn/client-configs/
# Send this file to the user for temporary access
```

### 3. Configure OAuth Providers
```bash
# Configure OAuth authentication
sudo /opt/dvarpala/scripts/configure-oauth.sh
```

### 4. Verify Installation
```bash
# Check system status
sudo /opt/dvarpala/scripts/check-status.sh
```

### 5. User Access Flow
1. **Admin provides** user with temporary `.ovpn` file
2. **User connects** to VPN using the `.ovpn` file
3. **User browses** to `http://192.168.100.1:8080` (only accessible page)
4. **User authenticates** via OAuth provider
5. **System grants** full internet access automatically
6. **Certificate revoked** automatically when user disconnects

## 🌐 Default Configuration

### Network Settings
- **Captive Portal Network**: `192.168.100.0/24` (initial temporary access)
- **Full Access Network**: `10.8.0.0/24` (after authentication)
- **OpenVPN Port**: `1194` (UDP)
- **Web Interface Port**: `8080` (TCP)
- **Management Port**: `7505` (localhost only)

### File Locations
```
/opt/dvarpala/                    # Main application directory
├── bin/dvarpala-server          # Main application binary
├── config/.env                  # Environment configuration
├── config/.db_credentials       # Database credentials
└── certs/admin.ovpn            # Admin VPN profile

/etc/openvpn/                    # OpenVPN configuration
├── server.conf                  # OpenVPN server configuration
└── easy-rsa/pki/               # Certificate authority and certificates
```

### Service Names
- `postgresql` - Database server
- `redis` - Cache server  
- `openvpn-server` - VPN server
- `dvarpala` - VPN management application

## 🆘 Emergency Recovery

If you lose VPN access:

1. **Access via console** (cloud provider's console/serial access)
2. **Run emergency script**:
   ```bash
   sudo bash /opt/dvarpala/scripts/emergency-access.sh
   ```
3. **Choose recovery option**:
   - Restore SSH access from internet
   - Display admin VPN configuration
   - Temporary 5-minute access window

## 📞 Support

For detailed configuration, troubleshooting, and management instructions, see:
- [Provisioning Documentation](scripts/provisioning/README.md)
- [GitHub Issues](https://github.com/yourcompany/dvarpala/issues)

---

**Installation Time**: Typically 5-10 minutes on a standard VM
**Difficulty**: Beginner (fully automated)
**Support**: Community support available via GitHub Issues