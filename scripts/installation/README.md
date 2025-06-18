# Dvarpala Cloud Installer

The Dvarpala Cloud Installer is a comprehensive tool that allows you to deploy dvarpala VPN servers to AWS, Google Cloud Platform, or Microsoft Azure directly from your local machine.

## Overview

Instead of manually setting up a VM and installing dvarpala, this installer:

1. **Authenticates** with your chosen cloud provider
2. **Creates infrastructure** including VPC, subnets, security groups, and VM
3. **Installs dvarpala** server automatically on the new VM
4. **Sets up object storage** for configuration backups
5. **Provides connection details** for immediate use

## Architecture

The installer features a **modular, object-oriented architecture** with clean separation of concerns:

- **`launcher.go`**: Primary entry point that coordinates all components
- **`cloud-installer.go`**: Main installer logic and interactive UI
- **`cloud_service.go`**: Service layer abstraction with unified cloud operations interface
- **`cloud_wrappers.go`**: Provider-specific wrapper implementations (AWS, GCP, Azure)
- **`providers/*.go`**: Low-level cloud provider implementations

This design eliminates repetitive switch statements and follows proper OOP principles with factory patterns and interface-based abstractions.

## 🌐 Frigga Brand IP Addressing

Dvarpala uses a **unique brand IP addressing scheme** designed specifically for Frigga Cloud Labs:

### **Brand IP: 172.30.x.x**

**Why 172.30.x.x?**
- **Brand Association**: "30" represents **Frigga** (F=6th letter × 5 = 30)
- **Uniqueness**: Rarely used in common setups (avoids conflicts with typical 10.x.x.x or 192.168.x.x)
- **RFC 1918 Compliant**: Within private IP range (172.16.0.0/12)
- **Future-Proof**: Allows 256 different /26 networks for future Frigga tools
- **Professional**: Creates consistent brand identity across all Frigga infrastructure

### **Network Architecture**

```
Frigga VPC: 172.30.0.0/26 (64 total IPs)
├── Public Subnet: 172.30.0.0/27 (30 usable IPs)
│   └── VPN Server: 172.30.0.4 (example)
└── Private Subnet: 172.30.0.32/27 (30 usable IPs)
    └── Database: 172.30.0.36 (example)

VPN Networks:
├── Captive Portal: 172.30.100.0/24 (initial connection - limited access)
└── Full Access: 172.30.8.0/21 (post-authentication - complete access)

Authentication Flow:
├── VPN Connect (portal/access) → Limited routing (172.30.100.1 only)
├── Web Auth (OAuth) → Sets authentication flag
├── VPN Reconnect → Full routing granted
└── VPN Disconnect → Authentication flag cleared (security)
```

### **Frigga IP Allocation Strategy**

| **Service** | **VPC CIDR** | **Purpose** |
|-------------|--------------|-------------|
| **Dvarpala VPN** | `172.30.0.0/26` | VPN infrastructure |
| **Future Tool A** | `172.30.1.0/26` | Next Frigga service |
| **Future Tool B** | `172.30.2.0/26` | Additional service |
| **Development** | `172.30.10.0/26` | Dev environments |
| **Testing** | `172.30.20.0/26` | Test environments |

This creates a **consistent brand identity** where any `172.30.x.x` IP immediately identifies Frigga Cloud Labs infrastructure.

### **🚀 Quick Reference: Captive Portal Configuration**

| **Component** | **Value** | **Purpose** |
|---------------|-----------|-------------|
| **VPN Credentials** | `portal` / `access` | Initial VPN connection |
| **Captive Portal URL** | `http://172.30.100.1:8080` | Web authentication interface |
| **VPN Network** | `172.30.100.0/24` | VPN client IP range |
| **Full Access Network** | `172.30.8.0/21` | Post-authentication routing |
| **Allowed Captive Ports** | `8080`, `53` (DNS) | Firewall-permitted traffic |
| **Blocked Captive Ports** | `22` (SSH), `80`, all others | Restricted until authenticated |
| **Auth Status Location** | `/tmp/dvarpala-auth-status-<user>` | Session management |
| **Connection Scripts** | `/opt/dvarpala/scripts/client-*.sh` | Dynamic routing logic |
| **Auto-Open Script** | `open-captive-portal.sh` | Browser auto-launch |
| **Platform Scripts** | `scripts/` folder | Windows/macOS/Linux variants |

### **Benefits of /26 Network Design**

**Right-Sized for VPN Infrastructure:**
- **64 total IPs** (62 usable) - perfect for small to medium deployments
- **30 IPs each** for public/private subnets - balanced allocation
- **Efficient** - no wasted IP space, minimal attack surface
- **Cost-Effective** - reduces cloud provider IP allocation costs

**Security Advantages:**
- **Small network** = reduced attack surface
- **Segmented** public/private subnets for defense in depth
- **Limited scope** for network scanning attempts
- **Focused monitoring** with manageable IP range

### **🔒 Captive Portal Security Model**

Dvarpala implements a **true captive portal** with strict network access controls:

#### **Access Restrictions by Connection Type:**

| **Connection Type** | **Accessible Services** | **Blocked Services** | **Implementation** |
|-------------------|----------------------|-------------------|------------------|
| **No VPN** | Public ports only (22, 80, 443, 1194, 8080) | All other ports | Cloud security groups |
| **VPN Captive Mode** | Only `172.30.100.1:8080` (captive portal) | SSH, other web services, internet | iptables firewall rules |
| **VPN Full Access** | All services, internet, SSH | None (after authentication) | Dynamic routing + firewall bypass |

#### **Firewall Implementation Details:**

The captive portal uses **multi-layer security**:

1. **Cloud Security Groups**: Control internet → VM traffic
2. **iptables Rules**: Control VPN clients → VM traffic
3. **Dynamic Routing**: Conditional network access based on auth status

**Specific iptables Rules Applied:**
```bash
# Allow ONLY captive portal access
iptables -I FORWARD -s 172.30.100.0/24 -d 172.30.100.1 -p tcp --dport 8080 -j ACCEPT

# Allow DNS for portal functionality  
iptables -I FORWARD -s 172.30.100.0/24 -p udp --dport 53 -j ACCEPT

# Block SSH access until authenticated
iptables -I FORWARD -s 172.30.100.0/24 -d 172.30.100.1 -p tcp --dport 22 -j DROP

# Block ALL other traffic from VPN clients
iptables -A FORWARD -s 172.30.100.0/24 -j DROP
```

#### **Example Scenarios:**

**Scenario 1: NGINX on Port 8081**
- ❌ **No VPN**: Blocked by cloud security groups
- ❌ **Captive VPN**: Blocked by iptables DROP rule
- ✅ **Full VPN**: Accessible after authentication

**Scenario 2: NGINX on Port 80**
- ✅ **No VPN**: Accessible (cloud security group allows port 80)
- ❌ **Captive VPN**: Blocked by iptables (only port 8080 allowed)
- ✅ **Full VPN**: Accessible after authentication

**Scenario 3: SSH Access**
- ✅ **No VPN**: Accessible from internet (cloud security group allows port 22)
- ❌ **Captive VPN**: Explicitly blocked by iptables DROP rule
- ✅ **Full VPN**: Accessible after authentication

## Supported Cloud Providers

- **Amazon Web Services (AWS)**
- **Google Cloud Platform (GCP)**  
- **Microsoft Azure**

## Prerequisites

### Local Machine Requirements

- **Go 1.19+** installed
- **Git** for cloning the repository
- **Internet connection** for downloading tools and packages

### Cloud Provider Setup

Choose one of the following:

#### AWS Setup
- AWS account with billing enabled
- AWS CLI installed (`aws --version`)
- IAM user with appropriate permissions, or
- Existing AWS CLI profile configured

#### GCP Setup  
- Google Cloud project with billing enabled
- gcloud CLI installed (`gcloud --version`)
- Service account key (optional), or
- Existing gcloud authentication

#### Azure Setup
- Azure subscription
- Azure CLI installed (`az --version`)
- Service principal (optional), or
- Existing Azure CLI login

## Installation Methods

### Method 1: Interactive Installation (Recommended)

```bash
# Clone the repository
git clone https://github.com/frigga-cloud/dvarpala.git
cd dvarpala/scripts/installation

# Run the interactive installer
go run cloud-installer.go
```

The installer will guide you through:
1. Cloud provider selection
2. Authentication setup
3. Region and instance configuration  
4. Administrator account setup
5. Infrastructure deployment

### Method 2: Configuration File

Create a configuration file using the examples in `examples/` directory:

```bash
# Copy and customize a configuration template
cp examples/config-aws.json my-config.json

# Edit the configuration
vim my-config.json

# Run with configuration file
go run cloud-installer.go -config=my-config.json -interactive=false
```

### Method 3: Command Line Arguments

```bash
go run cloud-installer.go \
  -provider=aws \
  -region=us-east-1 \
  -interactive=false \
  -output=./my-deployment
```

## Configuration Options

### Cloud Provider Credentials

#### AWS
```json
{
  "cloud": {
    "provider": "aws",
    "region": "us-east-1",
    "credentials": {
      "access_key": "AKIA...",
      "secret_key": "..."
    }
  }
}
```

#### GCP
```json
{
  "cloud": {
    "provider": "gcp", 
    "region": "us-central1",
    "project_id": "my-project",
    "credentials": {
      "service_account_key": "/path/to/key.json"
    }
  }
}
```

#### Azure
```json
{
  "cloud": {
    "provider": "azure",
    "region": "East US",
    "credentials": {
      "client_id": "...",
      "client_secret": "...", 
      "tenant_id": "..."
    }
  }
}
```

### VM Configuration

```json
{
  "vm_config": {
    "instance_type": "t3.medium",  // AWS: t3.medium, GCP: e2-medium, Azure: Standard_B2s
    "disk_size_gb": 50,
    "tags": {
      "Project": "dvarpala",
      "Environment": "production"
    }
  }
}
```

### Network Configuration

```json
{
  "network_config": {
    "vpc_cidr": "172.30.0.0/26",
    "public_subnet_cidr": "172.30.0.0/27",
    "private_subnet_cidr": "172.30.0.32/27",
    "allowed_ips": ["0.0.0.0/0"]
  }
}
```

## What Gets Created

### AWS Infrastructure
- **VPC**: `frigga-labs` with DNS hostnames enabled
- **Subnets**: Public subnet for dvarpala server  
- **Internet Gateway**: For internet access
- **Security Group**: Ports 22, 1194, 8080, 443 open
- **EC2 Instance**: Ubuntu 22.04 with dvarpala installed
- **S3 Bucket**: For configuration backups

### GCP Infrastructure  
- **VPC Network**: `frigga-labs` in custom mode
- **Subnet**: Regional subnet for instances
- **Firewall Rules**: Allow dvarpala traffic
- **Compute Instance**: Ubuntu 22.04 with dvarpala installed
- **Cloud Storage**: Bucket for configuration backups

### Azure Infrastructure
- **Resource Group**: `frigga-labs-rg`
- **Virtual Network**: With subnet and NSG
- **Network Security Group**: Allow dvarpala traffic  
- **Virtual Machine**: Ubuntu 22.04 with dvarpala installed
- **Storage Account**: For configuration backups

## Output Files

After successful installation, you'll find these files in the output directory:

```
dvarpala-deployment/
├── installation-config.json    # Full installation configuration
├── connection-info.txt         # Server details and next steps
├── admin.ovpn                  # Admin VPN configuration file
└── ssh-key.pem                 # SSH private key (AWS/Azure)
```

## Post-Installation

### 1. Connect to Your VPN

Use the `admin.ovpn` file to connect:

```bash
# Using OpenVPN client
sudo openvpn admin.ovpn

# Or import into your VPN client GUI
```

### 2. Initial VPN Connection (Captive Portal Mode)

**First Time Connection:**
1. Import `admin.ovpn` into your VPN client
2. Use these credentials for initial connection:
   - **Username:** `portal`
   - **Password:** `access`
3. You'll get LIMITED access (captive portal only)
4. **Browser automatically opens** to `http://172.30.100.1:8080`

### 3. Complete Authentication via Web Portal

After VPN connection:
- **Browser should auto-open** to the captive portal
- If not, manually browse to `http://172.30.100.1:8080`
- Complete authentication via OAuth provider
- Once authenticated, **disconnect and reconnect VPN** for full access

### 4. Full VPN Access

After web authentication:
- **Disconnect and reconnect VPN** with same credentials (`portal`/`access`)
- You'll now have full network access including:
  - Internet browsing through VPN
  - SSH access to the server
  - Access to any additional services you install
- Configure additional users and settings

#### **Why Reconnection is Required:**

The captive portal uses **session-based authentication**:
1. **Initial connection**: Limited routing (only captive portal)
2. **Web authentication**: Sets authentication flag for your user
3. **Reconnection**: OpenVPN client-connect script detects authentication and grants full routing
4. **Session cleanup**: Authentication cleared on disconnect (security feature)

### 5. SSH Access (Optional)

```bash
# SSH to your server via VPN
ssh -i ssh-key.pem ubuntu@172.30.100.1
```

### 4. Generate User Certificates

```bash
# SSH into the server
ssh -i ssh-key.pem ubuntu@<server-public-ip>

# Generate user certificate
sudo /opt/dvarpala/bin/generate-temp-cert user@example.com
```

## Backup and Recovery

The installer automatically sets up object storage for backups:

- **AWS**: S3 bucket with versioning enabled
- **GCP**: Cloud Storage bucket with versioning
- **Azure**: Storage account with blob versioning

Configuration files are automatically uploaded to `dvarpala/` folder in the bucket.

## Security Features

### Infrastructure Security
- **VPC Isolation**: All resources in dedicated VPC
- **Minimal Attack Surface**: Only required ports open
- **Encrypted Storage**: Object storage encryption enabled
- **Network Segmentation**: Public/private subnet separation

### Access Security  
- **SSH Key Authentication**: No password access
- **VPN-Only SSH**: SSH restricted to VPN network after setup
- **OAuth Integration**: No local password management
- **Certificate-Based VPN**: Strong certificate authentication

## Troubleshooting

### Authentication Issues

```bash
# AWS
aws sts get-caller-identity

# GCP  
gcloud auth list

# Azure
az account show
```

### Cloud CLI Installation

```bash
# AWS CLI
curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip"
unzip awscliv2.zip && sudo ./aws/install

# gcloud CLI
curl https://sdk.cloud.google.com | bash
exec -l $SHELL

# Azure CLI
curl -sL https://aka.ms/InstallAzureCLIDeb | sudo bash
```

### Common Issues

**Issue**: VPC already exists
**Solution**: The installer will use the existing `frigga-labs` VPC

**Issue**: Bucket name conflicts  
**Solution**: Bucket names are generated with random suffixes

**Issue**: Instance fails to start
**Solution**: Check cloud provider quotas and permissions

### Captive Portal Troubleshooting

**Issue**: Can't access captive portal after VPN connection
**Solution**: 
- Verify VPN connected with credentials `portal`/`access`
- Check you're accessing exactly `http://172.30.100.1:8080`
- Ensure no proxy or DNS override in VPN client

**Issue**: Can't access internet/SSH after web authentication
**Solution**:
- **Must disconnect and reconnect VPN** after web authentication
- Authentication status is only checked on new connections
- Check logs: `sudo tail -f /var/log/openvpn/client-connect.log`

**Issue**: Additional services (NGINX, etc.) not accessible in captive mode
**Expected Behavior**: This is intentional security - only port 8080 allowed
- Install services on port 8080, or
- Wait until full VPN access after authentication

**Issue**: SSH blocked even with VPN connected
**Expected Behavior**: SSH is blocked until web authentication completed
- Complete OAuth authentication via captive portal first
- Disconnect and reconnect VPN for full access including SSH

**Issue**: Internet access works without VPN
**Expected Behavior**: Different access layers:
- **Internet → VM**: Controlled by cloud security groups  
- **VPN → VM**: Controlled by iptables firewall rules
- VPN clients have stricter restrictions than internet access

### Monitoring and Logs

**Authentication Logs:**
```bash
# OpenVPN authentication attempts
sudo tail -f /var/log/openvpn/auth.log

# Client connection/disconnection events  
sudo tail -f /var/log/openvpn/client-connect.log
sudo tail -f /var/log/openvpn/client-disconnect.log

# Web authentication events
sudo tail -f /var/log/openvpn/web-auth.log

# OpenVPN server logs
sudo tail -f /var/log/openvpn/openvpn.log
```

**Authentication Status Check:**
```bash
# Check if a user is authenticated
sudo /opt/dvarpala/bin/check-auth-status <username>

# Mark user as authenticated (after web portal login)
sudo /opt/dvarpala/bin/mark-user-authenticated <username>

# View current authentication statuses
ls -la /tmp/dvarpala-auth-status*
```

**Network Testing:**
```bash
# Test captive portal access (should work)
curl -i http://172.30.100.1:8080

# Test blocked access (should fail in captive mode)
curl -i http://172.30.100.1:80
ssh dvarpala@172.30.100.1
```

## 🚀 Auto-Open Captive Portal Feature

Dvarpala automatically opens the captive portal in your browser when you connect to VPN:

### **How It Works:**

1. **Built-in Script**: `admin.ovpn` includes an `up` script that runs after VPN connection
2. **Cross-Platform**: Works on Windows, macOS, and Linux
3. **Smart Detection**: Tests network connectivity before opening browser
4. **Fallback Methods**: Multiple browser detection methods for compatibility

### **Files Created:**

| **File** | **Purpose** | **Platform** |
|----------|-------------|--------------|
| `admin.ovpn` | Main config with auto-open script | All |
| `open-captive-portal.sh` | Built-in auto-open script | Unix/Linux/macOS |
| `scripts/open-captive-portal.bat` | Windows batch script | Windows |
| `scripts/open-captive-portal-unix.sh` | Enhanced Unix script | macOS/Linux |
| `AUTO-OPEN-SETUP.txt` | Setup instructions | All |

### **Compatibility:**

| **OpenVPN Client** | **Auto-Open Support** | **Setup Required** |
|-------------------|----------------------|-------------------|
| **OpenVPN CLI** | ✅ Automatic | None |
| **Tunnelblick (macOS)** | ✅ Automatic | None |
| **OpenVPN GUI (Windows)** | ⚠️ Manual Setup | Copy script to config folder |
| **NetworkManager (Linux)** | ✅ Automatic | None |
| **OpenVPN Connect** | ❌ Not Supported | Manual browser opening |

### **Troubleshooting Auto-Open:**

**Issue**: Browser doesn't open automatically
**Solutions**:
1. Check if your OpenVPN client supports the `up` directive
2. Ensure script execution is enabled in your VPN client
3. Check logs: `~/.dvarpala-client.log` (Unix) or `%TEMP%\dvarpala-client.log` (Windows)
4. Use manual scripts from `scripts/` folder

**Issue**: Script permission denied
**Solution**: 
```bash
chmod +x /path/to/open-captive-portal.sh
```

**Issue**: Corporate firewall blocks browser opening
**Solution**: Manually open `http://172.30.100.1:8080` after VPN connection

## 📁 File Structure

The `scripts/installation/` directory contains all cloud installer components:

### **🚀 Main Entry Points**

| File | Purpose | Usage |
|------|---------|-------|
| **`launcher.go`** | Primary entry point - launches modularized cloud installer | `go run launcher.go` |
| **`installer/cloud-installer.go`** | Core multi-cloud installer application | Main installer logic and UI |
| **`installer/cloud_service.go`** | Service layer abstraction for cloud operations | Service interface and factory |
| **`installer/cloud_wrappers.go`** | Cloud provider wrapper implementations | AWS, GCP, Azure wrapper classes |
| **`installer/databaseInstaller.go`** | Database setup and initialization library | Called by other installers |

### **🔧 Build & Setup Files**

| File | Purpose | Usage |
|------|---------|-------|
| **`installer/build.sh`** | Compiles installer binary (centralized build logic) | `cd installer && ./build.sh` |
| **`installer/install-dvarpala.sh`** | One-click launcher (advanced) - auto-installs Go | `curl ... \| bash` |
| **`installer/install-dvarpala-simple.sh`** | One-click launcher (simple) - requires Go | `curl ... \| bash` |
| **`installer/cloud-setup-server.sh`** | VM installation script executed via user-data | Executed automatically |

### **☁️ Cloud Provider Integration**

| File | Purpose | Supported Features |
|------|---------|-------------------|
| **`providers/aws.go`** | AWS implementation | VPC, EC2, S3, Security Groups |
| **`providers/gcp.go`** | GCP implementation | VPC, Compute Engine, Cloud Storage |
| **`providers/azure.go`** | Azure implementation | Resource Groups, VMs, Storage |

### **📋 Configuration Examples**

| File | Purpose | Contains |
|------|---------|----------|
| **`examples/config-aws.json`** | AWS configuration template | Access keys, instance types, regions |
| **`examples/config-gcp.json`** | GCP configuration template | Service accounts, machine types, zones |
| **`examples/config-azure.json`** | Azure configuration template | Service principals, VM sizes, regions |

### **📖 Documentation & Dependencies**

| File | Purpose | Contains |
|------|---------|----------|
| **`README.md`** | Complete installation guide | Usage, troubleshooting, examples |
| **`go.mod`** | Go module definition | Dependencies, module name, version requirements |
| **`go.sum`** | Dependency checksums | Cryptographic hashes for security |

## 🔄 Installation File Flow

### **Complete Automated Flow:**
```
install-dvarpala.sh (downloads & installs Go if needed)
       ↓
   Downloads repo
       ↓
   Calls build.sh → dvarpala-installer binary
       ↓                    ↓
   Runs via launcher.go → cloud-installer.go
                           ↓
                   Uses providers/{aws,gcp,azure}.go
                           ↓
                   Creates VM with cloud-setup-server.sh (user data)
                           ↓
                   Uses databaseInstaller.go functions for DB setup
                           ↓
              Ready-to-use VPN server with database
```

### **Direct Development Flow:**
```
launcher.go (runs all modular components)
       ↓
   installer/cloud-installer.go (main installer logic & UI)
       ↓
   installer/cloud_service.go (service layer abstraction)
       ↓
   installer/cloud_wrappers.go (provider implementations)
       ↓
   providers/*.go (cloud-specific provider logic)
       ↓
   VM created with installer/cloud-setup-server.sh
       ↓
   installer/databaseInstaller.go (database initialization)
       ↓
   Complete VPN server deployment
```

### **Build-Only Flow:**
```
build.sh
   ↓
go mod tidy + go build (compiles all modular components)
   ↓
dvarpala-installer (standalone binary with all modules)
   ↓
./dvarpala-installer (runs modularized cloud-installer)
```

## 🔧 Usage Examples

### **Direct Method** (Recommended for Development)
```bash
# Clone repository
git clone https://github.com/frigga-cloud/dvarpala.git
cd dvarpala/scripts/installation

# Option 1: Use launcher (recommended)
go run launcher.go

# Option 2: Run modularized cloud installer directly
go run installer/cloud-installer.go installer/cloud_service.go installer/cloud_wrappers.go

# Option 3: With configuration file (via launcher)
go run launcher.go -config=examples/config-aws.json

# Option 4: With command line flags (via launcher)
go run launcher.go -provider=aws -region=us-east-1 -interactive=false
```

### **One-Click Methods** (For End Users)
```bash
# Simple version (requires Go pre-installed)
curl -fsSL https://raw.githubusercontent.com/frigga-cloud/dvarpala/main/scripts/installation/install-dvarpala-simple.sh | bash

# Advanced version (installs Go automatically)
curl -fsSL https://raw.githubusercontent.com/frigga-cloud/dvarpala/main/scripts/installation/install-dvarpala.sh | bash
```

### **Build Standalone Binary**
```bash
# Build distributable installer using centralized build script
cd installer && ./build.sh

# Run standalone binary (from installer directory)
./dvarpala-installer

# Or build manually with all modular components
cd installer && go build -o dvarpala-installer cloud-installer.go cloud_service.go cloud_wrappers.go
```

## ⚠️ Important Notes

### **Compilation Notes**
- **Recommended**: Use `go run launcher.go` or `./build.sh` for best results
- **Modular Architecture**: Code split into `cloud-installer.go`, `cloud_service.go`, and `cloud_wrappers.go`
- **Build process**: `install-dvarpala.sh` now uses `build.sh` (no code duplication)
- **Launcher**: Automatically includes all modular components when running
- **Database setup**: `databaseInstaller.go` provides reusable database functions

### **Cloud Provider Requirements**
- **AWS**: AWS CLI installed and configured, or access keys provided
- **GCP**: gcloud CLI installed and authenticated, or service account key
- **Azure**: Azure CLI installed and logged in, or service principal credentials

### **Current Implementation Status**
- ✅ **Interactive Setup**: Full implementation for all providers
- ✅ **Modular Architecture**: Service layer abstraction with cloud provider wrappers
- ✅ **Clean Code Structure**: Separated into `cloud-installer.go`, `cloud_service.go`, `cloud_wrappers.go`
- ✅ **Build Process**: Centralized build logic in `build.sh` (no duplication)
- ✅ **Entry Points**: Multiple ways to run the installer (`launcher.go`, direct execution)
- ✅ **Database Setup**: Standalone `databaseInstaller.go` with reusable functions
- ✅ **Provider Integration**: Dedicated files for AWS, GCP, and Azure implementations

### **Architecture Improvements**
- **🔄 DRY Principle**: `install-dvarpala.sh` now calls `build.sh` instead of duplicating logic
- **📁 Clear Naming**: `install.go` → `databaseInstaller.go` for better purpose identification  
- **🚀 Multiple Entry Points**: `launcher.go` as primary entry, direct execution as alternative
- **🛠️ Centralized Build**: Single `build.sh` script handles all compilation needs
- **📦 Modular Design**: Service layer abstraction with cloud provider wrappers
- **🎯 Separation of Concerns**: Main installer, service layer, and wrappers in separate files
- **🔧 OOP Implementation**: Eliminated repetitive switch statements with proper OOP patterns

## Cost Optimization

### Instance Sizes
- **Development**: t3.small / e2-small / Standard_B1ms (~$15-20/month)
- **Production**: t3.medium / e2-medium / Standard_B2s (~$25-35/month)
- **High Traffic**: t3.large / e2-standard-2 / Standard_B2ms (~$50-70/month)

### Storage Costs
- Object storage: ~$0.02-0.05/GB/month
- Network egress: Varies by provider and usage

## Support

- **Issues**: [GitHub Issues](https://github.com/frigga-cloud/dvarpala/issues)
- **Documentation**: See `docs/` directory
- **Community**: Discussions on GitHub

## Cleanup

To remove all created resources:

```bash
# The installer doesn't include cleanup yet
# Use cloud provider console or CLI to remove:
# - EC2 instances, VPC (AWS)
# - Compute instances, VPC (GCP)  
# - Resource groups (Azure)
```

---

**Installation Time**: 5-15 minutes depending on cloud provider
**Supported OS**: macOS, Linux, Windows (with WSL)
**License**: See LICENSE file