# Dvarpala VPN System

**Dvarpala** (द्वारपाल) - Sanskrit for "gatekeeper" or "door guardian" - is an enterprise-grade VPN solution with Zero Trust Network Access (ZTNA) and granular access control.

## 🚀 Quick Start

### Prerequisites

- Go 1.21 or higher
- PostgreSQL 12+
- Redis 6+
- OpenVPN (for VPN functionality)

### Installation

1. **Clone the repository**
   ```bash
   git clone https://github.com/yourcompany/dvarpala.git
   cd dvarpala
   ```

2. **Install dependencies**
   ```bash
   make deps
   ```

3. **Configure the system**
   ```bash
   cp configs/environments/development.yaml configs/environments/local.yaml
   # Edit configs/environments/local.yaml with your database and Redis settings
   ```

4. **Set up database**
   ```bash
   # Create PostgreSQL database
   createdb dvarpala_dev
   
   # Install system with development data
   make install-dev
   ```

5. **Start the server**
   ```bash
   make dev
   ```

6. **Access the system**
   - Web interface: http://localhost:8080
   - API endpoint: http://localhost:8080/api/v1

## 📋 Core Database Tables

The installation script creates the following core tables:

### Core Entity Tables
- **`users`** - User accounts with OAuth integration
- **`groups`** - Hierarchical user groups  
- **`resources`** - Protected resources (dashboards, VMs, databases, services)
- **`audit_logs`** - Complete audit trail of all actions

### VPN-Related Tables
- **`vpn_sessions`** - Active and historical VPN connections
- **`vpn_configs`** - Client VPN configurations and certificates
- **`network_routes`** - Network routing rules for different access levels

### Authentication Tables
- **`sessions`** - Web user sessions
- **`oauth_states`** - OAuth state tokens for security

### Security Tables
- **`ip_whitelists`** - IP-based access control

### Junction Tables (Many-to-Many)
- **`user_groups`** - User-Group relationships
- **`group_permissions`** - Group-Resource permissions
- **`group_network_routes`** - Group-Network route assignments

## 🛠️ Available Make Commands

```bash
# Build and Installation
make build              # Build all binaries
make install            # Install system (create tables)
make install-fresh      # Fresh install (drop and recreate tables)
make install-dev        # Install with development data

# Development
make dev                # Start development server
make test               # Run tests
make test-coverage      # Run tests with coverage report
make fmt                # Format code
make lint               # Lint code

# Data Management
make create-test-users  # Create test users
make seed-data          # Seed development data

# Utilities
make clean              # Clean build artifacts
make docker             # Build Docker image
make help               # Show all available commands
```

## 🗃️ Database Schema Details

### User Management
```sql
-- Users with OAuth integration
users (
  id, email (unique), full_name, department, 
  status, oauth_provider, last_login, 
  created_at, updated_at, deleted_at
)

-- Hierarchical groups
groups (
  id, name (unique), description, parent_id,
  created_at, updated_at, deleted_at
)
```

### VPN Infrastructure
```sql
-- VPN session tracking
vpn_sessions (
  id, user_id, client_ip, server_ip, status,
  connected_at, disconnected_at, bytes_in, bytes_out,
  created_at, updated_at, deleted_at
)

-- Client configurations
vpn_configs (
  id, user_id, config_name, client_cert, client_key,
  ca_cert, config_data, status, expires_at,
  created_at, updated_at, deleted_at
)
```

### Access Control
```sql
-- Protected resources
resources (
  id, name, type, url, ip_address, port,
  description, created_at, updated_at, deleted_at
)

-- Network routing rules
network_routes (
  id, name, destination, gateway, route_type,
  priority, is_active, description,
  created_at, updated_at, deleted_at
)
```

## 🔐 Security Features

- **OAuth Integration** - Google, Microsoft, GitHub
- **Session Management** - Secure web sessions with Redis
- **IP Whitelisting** - Per-user, per-group, and global IP restrictions
- **Audit Logging** - Complete audit trail of all actions
- **Two-Stage VPN Auth** - Captive portal + OAuth validation
- **Granular Permissions** - Resource-level access control

## 🏗️ Architecture

### Authentication Flow
1. **Initial VPN Connection** - Static credentials → Captive portal access
2. **Web Authentication** - OAuth validation → Database check → Full access
3. **Session Management** - JWT tokens + Redis sessions
4. **Audit Trail** - All actions logged with user context

### Network Access Levels
- **Captive Portal** - Limited access for authentication
- **Full Access** - Complete network access based on group membership
- **Restricted** - Custom routing rules per group

## 📊 Default Data Created

The installation creates essential system data:

### Default Groups
- `system_admins` - System administrators
- `vpn_users` - Standard VPN users  
- `guests` - Limited access guests

### Default Resources
- `System Administration` - Admin dashboard
- `User Dashboard` - User portal
- `VPN Server` - Main VPN endpoint

### Default Network Routes
- Captive portal access (DNS only)
- Internal network access (10.0.0.0/8)
- Internet access (0.0.0.0/0)

## 🔧 Configuration

Edit `configs/environments/development.yaml`:

```yaml
database:
  host: localhost
  port: 5432
  name: dvarpala_dev
  user: dvarpala
  password: your_password

oauth:
  google:
    client_id: "your-google-client-id"
    client_secret: "your-google-client-secret"
  
auth:
  jwt_secret: "your-jwt-secret"
  allowed_domains:
    - "yourcompany.com"
```

## 📈 Development Workflow

1. **Initial Setup**
   ```bash
   make install-dev  # Creates tables + test data
   ```

2. **Development**
   ```bash
   make dev         # Start server with hot reload
   ```

3. **Testing**
   ```bash
   make test        # Run all tests
   make test-coverage  # Generate coverage report
   ```

4. **Data Management**
   ```bash
   make create-test-users  # Add test users
   make seed-data         # Add comprehensive test data
   ```

## 🎯 Next Steps

After installation:

1. Configure OAuth providers in your config file
2. Set up OpenVPN server integration
3. Create user groups and assign permissions
4. Configure network routing rules
5. Set up monitoring and logging

## 📚 Additional Resources

- [Configuration Guide](docs/configuration.md)
- [API Documentation](docs/api.md)
- [Deployment Guide](docs/deployment.md)
- [Troubleshooting](docs/troubleshooting.md)