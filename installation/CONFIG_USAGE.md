# Dvarpala Configuration-Based Installation

The Dvarpala installer now requires a configuration file for all installations. This approach provides better reproducibility, version control, and automation capabilities.

## 🚀 Quick Start

### 1. Choose a Configuration Template

Navigate to the `examples/` directory and choose the appropriate configuration template:

- **AWS**: `examples/aws-config.json`
- **GCP**: `examples/gcp-config.json`
- **Azure**: `examples/azure-config.json`
- **Generic**: `examples/config-template.json`

### 2. Create Your Configuration

Copy a template and customize it for your environment:

```bash
cp examples/gcp-config.json my-config.json
```

Edit `my-config.json` with your specific settings:

```json
{
  "cloud": {
    "provider": "gcp",
    "region": "us-central1",
    "project_id": "my-project-123",
    "credentials": {
      "service_account_key": "/path/to/my-service-account.json"
    }
  },
  "admin": {
    "email": "admin@mycompany.com",
    "full_name": "VPN Administrator"
  }
}
```

### 3. Run the Installation

```bash
# Using launcher (recommended)
go run launcher.go --config=my-config.json

# Or directly
go run installer/installer.go installer/cloud_service.go installer/cloud_wrappers.go --config=my-config.json
```

## 📋 Configuration Reference

### Cloud Provider Settings

#### AWS Configuration
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

#### GCP Configuration
```json
{
  "cloud": {
    "provider": "gcp",
    "region": "us-central1",
    "project_id": "your-project-id",
    "credentials": {
      "service_account_key": "/path/to/service-account.json"
    }
  }
}
```

#### Azure Configuration
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
    "instance_type": "e2-medium",
    "disk_size_gb": 50,
    "tags": {
      "Project": "dvarpala",
      "Environment": "production"
    }
  }
}
```

**Instance Type Recommendations:**
- **AWS**: `t3.small`, `t3.medium`, `t3.large`
- **GCP**: `e2-small`, `e2-medium`, `e2-standard-2`
- **Azure**: `Standard_B1ms`, `Standard_B2s`, `Standard_B2ms`

### Network Configuration
```json
{
  "network_config": {
    "vpc_cidr": "172.30.0.0/26",
    "public_subnet_cidr": "172.30.0.0/27",
    "private_subnet_cidr": "172.30.0.32/27",
    "allowed_ips": ["203.0.113.0/24"]
  }
}
```

### Admin Configuration
```json
{
  "admin": {
    "email": "admin@yourdomain.com",
    "full_name": "Admin User"
  }
}
```

## 🔐 Authentication Methods

### AWS Authentication
1. **Access Keys** (in config file)
2. **AWS CLI Profile** (omit credentials in config)
3. **IAM Role** (for EC2/Lambda execution)

### GCP Authentication
1. **Service Account Key** (in config file)
2. **gcloud CLI** (omit credentials in config)

### Azure Authentication
1. **Service Principal** (in config file)
2. **Azure CLI** (omit credentials in config)

## 🔒 Security Best Practices

### 1. Protect Configuration Files
- Never commit config files with credentials to version control
- Use environment variables for sensitive data
- Store config files with restricted permissions: `chmod 600 config.json`

### 2. Use Environment Variables
Replace sensitive values with environment variables:
```json
{
  "cloud": {
    "credentials": {
      "access_key": "${AWS_ACCESS_KEY_ID}",
      "secret_key": "${AWS_SECRET_ACCESS_KEY}"
    }
  }
}
```

### 3. Credential Files
For credential files, use absolute paths:
```json
{
  "credentials": {
    "service_account_key": "/home/user/.gcp/service-account.json"
  }
}
```

## 📁 Output and Deployment

After successful installation, you'll find:

```
dvarpala-deployment/
├── admin.ovpn              # OpenVPN client configuration
├── installation-config.json # Full installation configuration
├── connection-info.txt     # Connection details and next steps
└── friggalabs-vm-xxxxx-key # SSH private key (cloud-specific)
```

## 🧪 Configuration Validation

The installer validates your configuration before starting:

- ✅ Required fields are present
- ✅ Cloud provider is supported
- ✅ Network CIDRs don't overlap
- ✅ Instance types are valid for the provider
- ✅ Credentials format is correct

## 🚨 Troubleshooting

### Configuration File Not Found
```
❌ Configuration file is required!
```
**Solution**: Provide the `--config` flag with a valid JSON file.

### Invalid Configuration
```
❌ Failed to load config file: invalid character...
```
**Solution**: Validate your JSON syntax using `jq` or an online JSON validator.

### Authentication Failures
```
❌ Cloud authentication failed
```
**Solution**: Verify your credentials and permissions.

## 🔄 Migration from Interactive Mode

If you were using interactive mode before:

1. Run the installer once interactively to generate a config
2. Copy the generated `installation-config.json` 
3. Use it as your template for future installations
4. Remove sensitive data and replace with environment variables

## 📖 Examples

### Basic GCP Installation
```bash
# Create config
cat > gcp-prod.json << EOF
{
  "cloud": {
    "provider": "gcp",
    "region": "us-central1",
    "project_id": "my-prod-project"
  },
  "admin": {
    "email": "admin@company.com",
    "full_name": "Production Admin"
  }
}
EOF

# Run installation
go run launcher.go --config=gcp-prod.json
```

### AWS with Custom Network
```bash
# Create config with custom networking
cat > aws-custom.json << EOF
{
  "cloud": {
    "provider": "aws",
    "region": "eu-west-1"
  },
  "network_config": {
    "vpc_cidr": "10.0.0.0/24",
    "public_subnet_cidr": "10.0.0.0/25", 
    "private_subnet_cidr": "10.0.0.128/25"
  },
  "vm_config": {
    "instance_type": "t3.large",
    "disk_size_gb": 100
  }
}
EOF

# Run installation
go run launcher.go --config=aws-custom.json
```