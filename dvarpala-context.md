# Dvarpala VPN Project - Development Context

## Project Overview

**Project Name**: Dvarpala VPN  
**Etymology**: Dvarpala (द्वारपाल) - Sanskrit for "gatekeeper" or "door guardian"  
**Purpose**: Enterprise-grade VPN solution with Zero Trust Network Access (ZTNA) and granular access control

## Core Architecture

### Technology Stack
- **Backend**: Python 3.9+ with Flask
- **Database**: PostgreSQL with Redis for session management
- **VPN**: OpenVPN with custom authentication scripts
- **Authentication**: Multi-provider OAuth (Google, Microsoft, GitHub)
- **Frontend**: HTML/CSS/JavaScript with Jinja2 templates
- **Deployment**: Docker, Kubernetes
- **Monitoring**: Prometheus, Grafana

### Key Components
1. **OpenVPN Server** - Primary VPN connectivity with two-stage access
2. **Dvarpala Web Application** - Flask app with admin dashboard and API
3. **Access Control Engine** - Group-based permissions and dynamic routing
4. **Authentication System** - Captive portal with OAuth validation

## Authentication Flow

### User Experience
1. User connects with OpenVPN client using static credentials (`temp_user`/`temp_portal_access`)
2. Gets **captive portal access** (limited network: only dvarpala.yourcompany.com accessible)
3. Browser auto-opens authentication page
4. User completes OAuth login (Google/Microsoft/GitHub)
5. System validates: ✓ OAuth success + ✓ User exists in Dvarpala database
6. If both pass: VPN reconnects with **full access** based on user's group permissions
7. If either fails: Remains in captive portal or gets disconnected

### Technical Implementation
- OpenVPN calls custom auth script: `/etc/openvpn/auth-scripts/dvarpala-auth.py`
- Auth script checks Redis for user session validation
- Web authentication stores session in Redis with TTL
- Dynamic route pushing based on user groups
- Session expiry forces re-authentication

## Database Schema

### Core Tables
```sql
-- Users with OAuth integration
CREATE TABLE users (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    email VARCHAR(255) UNIQUE NOT NULL,
    full_name VARCHAR(255),
    department VARCHAR(100),
    status ENUM('active', 'inactive', 'suspended') DEFAULT 'active',
    oauth_provider ENUM('gmail', 'microsoft', 'github'),
    last_login TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Hierarchical groups
CREATE TABLE groups (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    name VARCHAR(100) UNIQUE NOT NULL,
    description TEXT,
    parent_group_id BIGINT NULL REFERENCES groups(id)
);

-- User-group assignments
CREATE TABLE user_groups (
    user_id BIGINT REFERENCES users(id),
    group_id BIGINT REFERENCES groups(id),
    assigned_by BIGINT REFERENCES users(id),
    PRIMARY KEY (user_id, group_id)
);

-- Resources (dashboards, VMs, services)
CREATE TABLE resources (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    name VARCHAR(255) NOT NULL,
    type ENUM('dashboard', 'vm', 'database', 'service'),
    url VARCHAR(500),
    ip_address INET,
    port INT
);

-- Group permissions
CREATE TABLE group_permissions (
    group_id BIGINT REFERENCES groups(id),
    resource_id BIGINT REFERENCES resources(id),
    permission_type ENUM('read', 'write', 'admin', 'ssh', 'full'),
    UNIQUE KEY (group_id, resource_id)
);

-- VPN session tracking
CREATE TABLE vpn_sessions (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    user_id BIGINT REFERENCES users(id),
    client_ip INET,
    mac_address VARCHAR(17),
    status ENUM('captive', 'authenticated', 'disconnected'),
    connected_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    disconnected_at TIMESTAMP NULL
);

-- Audit logging
CREATE TABLE audit_logs (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    user_id BIGINT REFERENCES users(id),
    action VARCHAR(100),
    resource_type VARCHAR(50),
    details JSON,
    ip_address INET,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

## Project Structure

```
dvarpala/
├── src/                           # Main application code
│   ├── core/                      # Core functionality (config, database, redis)
│   ├── models/                    # SQLAlchemy models
│   ├── auth/                      # Authentication (OAuth providers, decorators)
│   ├── vpn/                       # VPN management (OpenVPN integration)
│   ├── access_control/            # Permissions and access control logic
│   ├── api/v1/                    # REST API endpoints
│   ├── web/                       # Web interface (admin/user dashboards)
│   ├── services/                  # Business logic services
│   ├── utils/                     # Utility functions
│   └── cli/                       # Command line interface
├── templates/                     # Jinja2 HTML templates
├── static/                        # CSS, JS, images
├── config/                        # Configuration files
│   ├── environments/              # Environment-specific configs
│   └── openvpn/                   # OpenVPN server and auth scripts
├── scripts/                       # Setup, migration, deployment scripts
├── tests/                         # Unit, integration, e2e tests
├── deployment/                    # Docker, Kubernetes configurations
└── docs/                          # Documentation
```

## Key Design Patterns

### Service Layer Pattern
- Business logic in `src/services/` (UserService, VPNService, etc.)
- Controllers in `src/api/` and `src/web/` call services
- Services handle database operations and business rules

### Configuration Management
```python
# config/environments/development.py
class DevelopmentConfig:
    DATABASE_URL = 'postgresql://user:pass@localhost/dvarpala'
    REDIS_URL = 'redis://localhost:6379/0'
    ALLOWED_DOMAINS = ['yourcompany.com']
    SESSION_DURATION = 28800  # 8 hours
```

### OAuth Integration
```python
# src/auth/oauth/google.py
class GoogleOAuthProvider:
    def validate_token(self, token):
        # Validate OAuth token and return user email
        
    def get_user_info(self, token):
        # Get user profile information
```

## API Design

### Authentication API
```
POST /api/v1/auth/validate
POST /api/v1/auth/ssh/authorize
```

### User Management API
```
GET /api/v1/users
POST /api/v1/users
POST /api/v1/users/bulk
PUT /api/v1/users/{id}
DELETE /api/v1/users/{id}
```

### Group Management API
```
GET /api/v1/groups
POST /api/v1/groups
GET /api/v1/groups/{id}/users
POST /api/v1/groups/{id}/users
```

### VPN Management API
```
GET /api/v1/vpn/sessions
POST /api/v1/vpn/disconnect
GET /api/v1/vpn/stats
```

## Security Requirements

### Access Control
- **Two-factor validation**: OAuth + Database authorization
- **Session management**: Redis with configurable TTL
- **IP blocking**: Progressive blocking for failed attempts
- **Audit logging**: All actions logged with user attribution

### Network Security
- **Captive portal**: Limited initial access
- **Dynamic routing**: Routes based on group permissions
- **Certificate-based**: Client certificates for device trust
- **Firewall rules**: iptables integration for network control

## Environment Configuration

### Required Environment Variables
```bash
# Database
DATABASE_URL=postgresql://user:pass@host:5432/dvarpala
REDIS_URL=redis://localhost:6379/0

# OAuth Providers
GOOGLE_CLIENT_ID=your-google-client-id
GOOGLE_CLIENT_SECRET=your-google-client-secret
MICROSOFT_CLIENT_ID=your-microsoft-client-id
MICROSOFT_CLIENT_SECRET=your-microsoft-client-secret
GITHUB_CLIENT_ID=your-github-client-id
GITHUB_CLIENT_SECRET=your-github-client-secret

# Security
SECRET_KEY=your-secret-key
ALLOWED_DOMAINS=yourcompany.com,subsidiary.com

# VPN Configuration
OPENVPN_MGMT_HOST=localhost
OPENVPN_MGMT_PORT=7505
SESSION_DURATION=28800
CAPTIVE_PORTAL_TIMEOUT=300

# Security Policies
FAILED_LOGIN_THRESHOLD=3
IP_BLOCK_DURATION=1200
```

## Development Guidelines

### Code Style
- **Python**: PEP 8 compliance, type hints where applicable
- **Flask**: Application factory pattern with blueprints
- **Database**: SQLAlchemy with Alembic migrations
- **Testing**: pytest with fixtures and mocking

### Error Handling
```python
# Custom exceptions in src/core/exceptions.py
class DvarpalaException(Exception):
    pass

class AuthenticationError(DvarpalaException):
    pass

class AuthorizationError(DvarpalaException):
    pass
```

### Logging
```python
import logging
logger = logging.getLogger(__name__)

# Standard log levels
logger.info("User authenticated successfully")
logger.warning("Failed authentication attempt")
logger.error("Database connection failed")
```

### Testing Strategy
- **Unit tests**: Individual functions and methods
- **Integration tests**: API endpoints and database operations
- **E2E tests**: Complete user workflows
- **Security tests**: Authentication and authorization flows

## OpenVPN Integration

### Server Configuration
```bash
# Key settings in server.conf
auth-user-pass-verify /etc/openvpn/auth-scripts/dvarpala-auth.py via-env
client-connect /etc/openvpn/auth-scripts/client-connect.sh
client-disconnect /etc/openvpn/auth-scripts/client-disconnect.sh
management localhost 7505
```

### Authentication Script
```python
# /etc/openvpn/auth-scripts/dvarpala-auth.py
def authenticate_user():
    username = os.environ.get('username')
    password = os.environ.get('password')
    client_ip = os.environ.get('untrusted_ip')
    
    if username == "temp_user" and password == "temp_portal_access":
        # Check Redis for web authentication
        # Grant captive portal or full access based on session
        return True
    return False
```

## Common Development Tasks

### Adding New OAuth Provider
1. Create provider class in `src/auth/oauth/`
2. Implement `validate_token()` and `get_user_info()` methods
3. Add provider configuration to environment configs
4. Update authentication flow in web interface

### Adding New Resource Type
1. Add new enum value to `resources.type`
2. Update permission checking logic in `src/access_control/`
3. Add route generation logic in VPN service
4. Update admin interface for resource management

### Creating New API Endpoint
1. Add route in appropriate `src/api/v1/` module
2. Implement request validation
3. Call appropriate service layer function
4. Add API tests in `tests/integration/`

### Adding New User Group
1. Use admin interface or CLI: `python -m src.cli group create`
2. Assign users to group
3. Configure resource permissions
4. Test access control with VPN connection

## Deployment Considerations

### Development
```bash
# Local development with Docker
docker-compose up --build

# Database setup
docker-compose exec web python scripts/migration/migrate.py
```

### Production
```bash
# Kubernetes deployment
kubectl apply -f deployment/kubernetes/

# Health checks
kubectl get pods -n dvarpala
```

### Monitoring
- **Prometheus metrics**: Authentication rates, session counts, errors
- **Grafana dashboards**: VPN overview, user activity, system health
- **Log aggregation**: ELK stack for centralized logging

## Integration Points

### External Tool Authentication
```python
# Example: SSH access control
response = requests.post('https://dvarpala.company.com/api/v1/auth/ssh/authorize', {
    'email': 'user@company.com',
    'server_ip': '192.168.1.10'
})
```

### Database Access Control
```python
# Example: PostgreSQL access validation
response = requests.post('https://dvarpala.company.com/api/v1/auth/validate', {
    'email': 'user@company.com',
    'resource': 'production-database',
    'access_type': 'read'
})
```

## Troubleshooting Common Issues

### VPN Connection Issues
- Check OpenVPN server logs: `/var/log/openvpn/server.log`
- Verify authentication script: `/etc/openvpn/auth-scripts/dvarpala-auth.py`
- Check Redis session: `redis-cli get "auth:CLIENT_IP"`

### Authentication Failures
- Verify OAuth configuration
- Check user exists in database with active status
- Validate email domain in ALLOWED_DOMAINS

### Permission Issues
- Check user group assignments
- Verify group permissions for resources
- Test with admin user account

This context provides everything needed to understand and develop the Dvarpala VPN system effectively.