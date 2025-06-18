# Dvarpala Cloud Installer

The Dvarpala Cloud Installer is a comprehensive tool that allows you to deploy dvarpala VPN servers to AWS, Google Cloud Platform, or Microsoft Azure directly from your local machine.

## Overview

Instead of manually setting up a VM and installing dvarpala, this installer:

1. **Authenticates** with your chosen cloud provider
2. **Creates infrastructure** including VPC, subnets, security groups, and VM
3. **Installs dvarpala** server automatically on the new VM
4. **Sets up object storage** for configuration backups
5. **Provides connection details** for immediate use

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
git clone https://github.com/yourcompany/dvarpala.git
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
    "vpc_cidr": "10.0.0.0/16",
    "public_subnet_cidr": "10.0.1.0/24",
    "private_subnet_cidr": "10.0.2.0/24",
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

### 2. Access Web Interface

Once connected to VPN:
- Browse to `http://192.168.100.1:8080`
- Login with OAuth provider
- Configure additional users and settings

### 3. SSH Access (Optional)

```bash
# SSH to your server via VPN
ssh -i ssh-key.pem ubuntu@192.168.100.1
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

## 📁 File Structure

The `scripts/installation/` directory contains all cloud installer components:

### **🚀 Main Entry Points**

| File | Purpose | Usage |
|------|---------|-------|
| **`launcher.go`** | Primary entry point - launches cloud installer | `go run launcher.go` |
| **`installer/cloud-installer.go`** | Core multi-cloud installer application | `go run installer/cloud-installer.go` |
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
launcher.go (or go run installer/cloud-installer.go)
       ↓
   installer/cloud-installer.go (interactive wizard)
       ↓
   providers/*.go (cloud-specific logic)
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
go mod tidy + go build
   ↓
dvarpala-installer (standalone binary)
   ↓
./dvarpala-installer (runs cloud-installer.go)
```

## 🔧 Usage Examples

### **Direct Method** (Recommended for Development)
```bash
# Clone repository
git clone https://github.com/yourcompany/dvarpala.git
cd dvarpala/scripts/installation

# Option 1: Use launcher (recommended)
go run launcher.go

# Option 2: Run cloud installer directly
go run installer/cloud-installer.go

# Option 3: With configuration file
go run installer/cloud-installer.go -config=../examples/config-aws.json

# Option 4: With command line flags
go run installer/cloud-installer.go -provider=aws -region=us-east-1 -interactive=false
```

### **One-Click Methods** (For End Users)
```bash
# Simple version (requires Go pre-installed)
curl -fsSL https://raw.githubusercontent.com/yourcompany/dvarpala/main/scripts/installation/install-dvarpala-simple.sh | bash

# Advanced version (installs Go automatically)
curl -fsSL https://raw.githubusercontent.com/yourcompany/dvarpala/main/scripts/installation/install-dvarpala.sh | bash
```

### **Build Standalone Binary**
```bash
# Build distributable installer using centralized build script
cd installer && ./build.sh

# Run standalone binary (from installer directory)
./dvarpala-installer

# Or build manually (not recommended - use build.sh instead)
cd installer && go build -o dvarpala-installer cloud-installer.go
```

## ⚠️ Important Notes

### **Compilation Notes**
- **Recommended**: Use `go run launcher.go` or `./build.sh` for best results
- **Alternative**: Run `go run cloud-installer.go` directly 
- **Build process**: `install-dvarpala.sh` now uses `build.sh` (no code duplication)
- **Architecture**: Modular design with separate files for specific purposes
- **Database setup**: `databaseInstaller.go` provides reusable database functions

### **Cloud Provider Requirements**
- **AWS**: AWS CLI installed and configured, or access keys provided
- **GCP**: gcloud CLI installed and authenticated, or service account key
- **Azure**: Azure CLI installed and logged in, or service principal credentials

### **Current Implementation Status**
- ✅ **Interactive Setup**: Full implementation for all providers
- ✅ **Modular Architecture**: Clean separation of concerns with dedicated files
- ✅ **Build Process**: Centralized build logic in `build.sh` (no duplication)
- ✅ **Entry Points**: Multiple ways to run the installer (`launcher.go`, direct execution)
- ✅ **Database Setup**: Standalone `databaseInstaller.go` with reusable functions
- ✅ **Provider Integration**: Dedicated files for AWS, GCP, and Azure implementations

### **Architecture Improvements**
- **🔄 DRY Principle**: `install-dvarpala.sh` now calls `build.sh` instead of duplicating logic
- **📁 Clear Naming**: `install.go` → `databaseInstaller.go` for better purpose identification  
- **🚀 Multiple Entry Points**: `launcher.go` as primary entry, direct execution as alternative
- **🛠️ Centralized Build**: Single `build.sh` script handles all compilation needs
- **📦 Modular Design**: Each file has a single, clear responsibility

## Cost Optimization

### Instance Sizes
- **Development**: t3.small / e2-small / Standard_B1ms (~$15-20/month)
- **Production**: t3.medium / e2-medium / Standard_B2s (~$25-35/month)
- **High Traffic**: t3.large / e2-standard-2 / Standard_B2ms (~$50-70/month)

### Storage Costs
- Object storage: ~$0.02-0.05/GB/month
- Network egress: Varies by provider and usage

## Support

- **Issues**: [GitHub Issues](https://github.com/yourcompany/dvarpala/issues)
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