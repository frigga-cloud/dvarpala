# Dvarpala VPN Database Architecture

This document provides a comprehensive overview of the database structure for the Dvarpala VPN system, including all tables and their relationships.

## 🏗️ Database Overview

The Dvarpala VPN database is designed around a Zero Trust Network Access (ZTNA) model with granular access control, comprehensive audit logging, and hierarchical user management.

### Core Design Principles
- **Zero Trust**: Every access request is verified and logged
- **Hierarchical Structure**: Groups support parent-child relationships for organizational alignment
- **Granular Permissions**: Resource-level access control with multiple permission types
- **Comprehensive Auditing**: All actions are logged with full context
- **Flexible Authentication**: Support for multiple OAuth providers
- **Soft Deletes**: Data is preserved for audit and compliance purposes

## 📊 Database Tables Overview

### Primary Entity Tables (9)
1. **users** - System users with OAuth authentication
2. **groups** - Hierarchical user groups for access control
3. **resources** - Protected system resources requiring access control
4. **vpn_sessions** - VPN connection tracking and monitoring
5. **vpn_configs** - VPN client configurations and certificates
6. **network_routes** - Network routing rules for different access levels
7. **sessions** - Web application session management
8. **oauth_states** - OAuth CSRF protection tokens
9. **ip_whitelists** - IP-based access control restrictions

### Junction/Relationship Tables (3)
10. **user_groups** - Many-to-many: Users ↔ Groups
11. **group_permissions** - Many-to-many: Groups ↔ Resources (with permission type)
12. **group_network_routes** - Many-to-many: Groups ↔ Network Routes

### Audit & Logging Tables (1)
13. **audit_logs** - Complete system audit trail

### Legacy Tables (1)
14. **permissions** - Deprecated permission model (use group_permissions instead)

## 🔗 Table Relationships & Connections

### 1. User Management Flow
```
users (OAuth users)
  ├── user_groups ──► groups (hierarchical)
  ├── vpn_sessions (VPN connections)
  ├── vpn_configs (client configurations)
  ├── sessions (web sessions)
  ├── ip_whitelists (user-specific IP restrictions)
  └── audit_logs (user actions)
```

### 2. Access Control Flow
```
groups (user collections)
  ├── user_groups ──► users
  ├── group_permissions ──► resources (with permission_type)
  ├── group_network_routes ──► network_routes
  ├── ip_whitelists (group-specific IP restrictions)
  └── parent_id ──► groups (self-referencing hierarchy)
```

### 3. VPN Infrastructure Flow
```
users
  ├── vpn_sessions (connection tracking)
  └── vpn_configs (client certificates)

groups
  └── group_network_routes ──► network_routes (routing rules)
```

### 4. Security & Audit Flow
```
users
  ├── sessions (web authentication)
  ├── oauth_states (OAuth CSRF protection)
  ├── ip_whitelists (IP-based restrictions)
  └── audit_logs (action tracking)
```

## 📋 Detailed Table Definitions

### Core Entity Tables

#### 1. `users` - System Users
**Purpose**: OAuth-authenticated users with VPN access

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | SERIAL | PRIMARY KEY | Unique user identifier |
| email | VARCHAR(255) | UNIQUE, NOT NULL | OAuth email (unique login) |
| full_name | VARCHAR(255) | | Display name from OAuth |
| department | VARCHAR(100) | | Organizational unit |
| status | ENUM | DEFAULT 'active' | active/inactive/suspended |
| oauth_provider | VARCHAR(50) | | google/microsoft/github |
| last_login | TIMESTAMP | NULL | Last successful login |
| created_at | TIMESTAMP | NOT NULL | Account creation time |
| updated_at | TIMESTAMP | NOT NULL | Last modification time |
| deleted_at | TIMESTAMP | NULL | Soft delete timestamp |

**Relationships:**
- **1:N** with `vpn_sessions` (user → sessions)
- **1:N** with `vpn_configs` (user → configurations)
- **1:N** with `sessions` (user → web sessions)
- **1:N** with `audit_logs` (user → actions)
- **M:N** with `groups` via `user_groups`
- **1:N** with `ip_whitelists` (user-specific restrictions)

#### 2. `groups` - Hierarchical User Groups
**Purpose**: Collections of users with shared permissions and network access

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | SERIAL | PRIMARY KEY | Unique group identifier |
| name | VARCHAR(100) | UNIQUE, NOT NULL | Group name (e.g., "administrators") |
| description | TEXT | | Purpose and scope description |
| parent_id | INTEGER | NULL, FK→groups.id | Parent group for hierarchy |
| created_at | TIMESTAMP | NOT NULL | Group creation time |
| updated_at | TIMESTAMP | NOT NULL | Last modification time |
| deleted_at | TIMESTAMP | NULL | Soft delete timestamp |

**Relationships:**
- **Self-referencing**: parent_id → groups.id (hierarchy)
- **M:N** with `users` via `user_groups`
- **M:N** with `resources` via `group_permissions`
- **M:N** with `network_routes` via `group_network_routes`
- **1:N** with `ip_whitelists` (group-specific restrictions)

#### 3. `resources` - Protected System Resources
**Purpose**: Assets requiring access control (dashboards, VMs, databases, services)

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | SERIAL | PRIMARY KEY | Unique resource identifier |
| name | VARCHAR(255) | NOT NULL | Resource display name |
| type | ENUM | NOT NULL | dashboard/vm/database/service |
| url | VARCHAR(500) | | Web URL (for dashboards/services) |
| ip_address | VARCHAR(45) | | IP address (for VMs/databases) |
| port | INTEGER | | Network port (for VMs/databases) |
| description | TEXT | | Resource purpose and details |
| created_at | TIMESTAMP | NOT NULL | Resource registration time |
| updated_at | TIMESTAMP | NOT NULL | Last modification time |
| deleted_at | TIMESTAMP | NULL | Soft delete timestamp |

**Relationships:**
- **M:N** with `groups` via `group_permissions`

#### 4. `vpn_sessions` - VPN Connection Tracking
**Purpose**: Monitor and bill VPN connections with traffic data

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | SERIAL | PRIMARY KEY | Unique session identifier |
| user_id | INTEGER | NOT NULL, FK→users.id | Session owner |
| client_ip | VARCHAR(45) | NOT NULL | VPN client IP address |
| server_ip | VARCHAR(45) | | VPN server IP address |
| status | ENUM | DEFAULT 'active' | active/disconnected/expired |
| connected_at | TIMESTAMP | NOT NULL | Connection start time |
| disconnected_at | TIMESTAMP | NULL | Connection end time |
| bytes_in | BIGINT | DEFAULT 0 | Data received by client |
| bytes_out | BIGINT | DEFAULT 0 | Data sent by client |
| created_at | TIMESTAMP | NOT NULL | Record creation time |
| updated_at | TIMESTAMP | NOT NULL | Last update time |
| deleted_at | TIMESTAMP | NULL | Soft delete timestamp |

**Relationships:**
- **N:1** with `users` (sessions belong to user)

#### 5. `vpn_configs` - VPN Client Configurations
**Purpose**: Store VPN certificates and configuration files for users

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | SERIAL | PRIMARY KEY | Unique configuration identifier |
| user_id | INTEGER | NOT NULL, FK→users.id | Configuration owner |
| config_name | VARCHAR(100) | NOT NULL | Config display name |
| client_cert | TEXT | | Client certificate (PEM) |
| client_key | TEXT | | Client private key (PEM) |
| ca_cert | TEXT | | CA certificate (PEM) |
| config_data | TEXT | | Complete OpenVPN config |
| status | ENUM | DEFAULT 'active' | active/inactive/revoked |
| expires_at | TIMESTAMP | NULL | Configuration expiry |
| last_used_at | TIMESTAMP | NULL | Last VPN connection |
| downloaded_at | TIMESTAMP | NULL | User download time |
| created_at | TIMESTAMP | NOT NULL | Creation time |
| updated_at | TIMESTAMP | NOT NULL | Last update time |
| deleted_at | TIMESTAMP | NULL | Soft delete timestamp |

**Relationships:**
- **N:1** with `users` (configs belong to user)

#### 6. `network_routes` - VPN Routing Rules
**Purpose**: Define network access levels for different user groups

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | SERIAL | PRIMARY KEY | Unique route identifier |
| name | VARCHAR(100) | NOT NULL | Route display name |
| destination | VARCHAR(50) | NOT NULL | Network CIDR (e.g., "10.0.0.0/8") |
| gateway | VARCHAR(45) | | Gateway IP address |
| route_type | ENUM | NOT NULL | captive_portal/full_access/restricted |
| priority | INTEGER | DEFAULT 100 | Route priority (lower = higher) |
| is_active | BOOLEAN | DEFAULT true | Enable/disable route |
| description | VARCHAR(255) | | Route purpose description |
| created_at | TIMESTAMP | NOT NULL | Creation time |
| updated_at | TIMESTAMP | NOT NULL | Last update time |
| deleted_at | TIMESTAMP | NULL | Soft delete timestamp |

**Relationships:**
- **M:N** with `groups` via `group_network_routes`

#### 7. `sessions` - Web Application Sessions
**Purpose**: Manage user login sessions for the web interface

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | SERIAL | PRIMARY KEY | Unique session identifier |
| session_id | VARCHAR(255) | UNIQUE, NOT NULL | Session token for cookies |
| user_id | INTEGER | NOT NULL, FK→users.id | Session owner |
| ip_address | VARCHAR(45) | NOT NULL | Client IP address |
| user_agent | VARCHAR(500) | | Browser user agent |
| expires_at | TIMESTAMP | NOT NULL | Session expiry time |
| last_used_at | TIMESTAMP | NOT NULL | Last activity time |
| created_at | TIMESTAMP | NOT NULL | Session creation time |
| updated_at | TIMESTAMP | NOT NULL | Last update time |
| deleted_at | TIMESTAMP | NULL | Soft delete timestamp |

**Relationships:**
- **N:1** with `users` (sessions belong to user)

#### 8. `oauth_states` - OAuth CSRF Protection
**Purpose**: Store state tokens for OAuth flow security

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | SERIAL | PRIMARY KEY | Unique state identifier |
| state | VARCHAR(255) | UNIQUE, NOT NULL | Random CSRF token |
| provider | VARCHAR(50) | NOT NULL | OAuth provider name |
| user_ip | VARCHAR(45) | | Client IP address |
| user_agent | VARCHAR(500) | | Browser user agent |
| expires_at | TIMESTAMP | NOT NULL | Token expiry (short-lived) |
| used | BOOLEAN | DEFAULT false | Token consumption flag |
| created_at | TIMESTAMP | NOT NULL | Token creation time |
| updated_at | TIMESTAMP | NOT NULL | Last update time |
| deleted_at | TIMESTAMP | NULL | Soft delete timestamp |

**Relationships:**
- No direct relationships (standalone security tokens)

#### 9. `ip_whitelists` - IP Access Control
**Purpose**: Restrict access by IP address at user, group, or global level

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | SERIAL | PRIMARY KEY | Unique whitelist identifier |
| ip_address | VARCHAR(45) | NOT NULL | Allowed IP address |
| cidr | VARCHAR(50) | | IP range in CIDR notation |
| type | ENUM | NOT NULL | user/group/global |
| user_id | INTEGER | NULL, FK→users.id | User-specific restriction |
| group_id | INTEGER | NULL, FK→groups.id | Group-specific restriction |
| description | VARCHAR(255) | | Restriction description |
| is_active | BOOLEAN | DEFAULT true | Enable/disable restriction |
| expires_at | TIMESTAMP | NULL | Restriction expiry |
| created_at | TIMESTAMP | NOT NULL | Creation time |
| updated_at | TIMESTAMP | NOT NULL | Last update time |
| deleted_at | TIMESTAMP | NULL | Soft delete timestamp |

**Relationships:**
- **N:1** with `users` (user-specific restrictions)
- **N:1** with `groups` (group-specific restrictions)

### Junction/Relationship Tables

#### 10. `user_groups` - User↔Group Membership
**Purpose**: Many-to-many relationship between users and groups

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| user_id | INTEGER | PRIMARY KEY, FK→users.id | Group member |
| group_id | INTEGER | PRIMARY KEY, FK→groups.id | Target group |
| created_at | TIMESTAMP | NOT NULL | Membership start time |
| updated_at | TIMESTAMP | NOT NULL | Last update time |

**Relationships:**
- **N:1** with `users`
- **N:1** with `groups`

#### 11. `group_permissions` - Group↔Resource Access
**Purpose**: Define what resources each group can access and how

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| group_id | INTEGER | PRIMARY KEY, FK→groups.id | Authorized group |
| resource_id | INTEGER | PRIMARY KEY, FK→resources.id | Target resource |
| permission_type | ENUM | PRIMARY KEY | read/write/admin/ssh/full |
| created_at | TIMESTAMP | NOT NULL | Permission grant time |
| updated_at | TIMESTAMP | NOT NULL | Last update time |

**Relationships:**
- **N:1** with `groups`
- **N:1** with `resources`

#### 12. `group_network_routes` - Group↔Network Route Access
**Purpose**: Define what network routes each group can use

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| group_id | INTEGER | PRIMARY KEY, FK→groups.id | Authorized group |
| network_route_id | INTEGER | PRIMARY KEY, FK→network_routes.id | Accessible route |
| created_at | TIMESTAMP | NOT NULL | Access grant time |

**Relationships:**
- **N:1** with `groups`
- **N:1** with `network_routes`

### Audit & Logging Tables

#### 13. `audit_logs` - System Audit Trail
**Purpose**: Complete audit trail for security and compliance

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | SERIAL | PRIMARY KEY | Unique log entry identifier |
| user_id | INTEGER | NULL, FK→users.id | Action performer (null for system) |
| action | VARCHAR(100) | NOT NULL | Action type (e.g., "user_login") |
| resource_type | VARCHAR(50) | | Affected resource type |
| resource_id | INTEGER | NULL | Specific resource ID |
| ip_address | VARCHAR(45) | | Client IP address |
| user_agent | VARCHAR(500) | | Browser/client info |
| details | JSONB | | Structured action details |
| created_at | TIMESTAMP | NOT NULL | Action timestamp |
| deleted_at | TIMESTAMP | NULL | Soft delete timestamp |

**Relationships:**
- **N:1** with `users` (action performer)

## 🔄 Data Flow Examples

### 1. User Login Flow
```
1. User → oauth_states (CSRF token created)
2. OAuth Provider → User validation
3. User → users (login recorded)
4. User → sessions (web session created)
5. User action → audit_logs (login logged)
```

### 2. VPN Connection Flow
```
1. User → vpn_configs (download config)
2. VPN Client → vpn_sessions (session started)
3. Group membership → group_network_routes → network_routes (routing applied)
4. VPN usage → vpn_sessions (traffic recorded)
5. All actions → audit_logs (activities logged)
```

### 3. Access Control Flow
```
1. User → user_groups → groups (membership check)
2. Groups → group_permissions → resources (permission check)
3. Groups → group_network_routes → network_routes (network access check)
4. IP validation → ip_whitelists (IP restriction check)
5. Access decision → audit_logs (access logged)
```

### 4. Administrative Flow
```
1. Admin → groups (create/modify groups)
2. Admin → group_permissions (assign resource access)
3. Admin → group_network_routes (assign network access)
4. Admin → ip_whitelists (configure IP restrictions)
5. All changes → audit_logs (admin actions logged)
```

## 🔑 Key Database Features

### Hierarchical Groups
- Groups can have parent-child relationships via `parent_id`
- Permissions can be inherited from parent groups
- Organizational structure alignment

### Comprehensive Audit Trail
- All user actions logged in `audit_logs`
- Structured JSON details for rich context
- IP address and user agent tracking
- Support for system-generated events

### Flexible Access Control
- Resource-level permissions with multiple types
- Network-level routing control
- IP-based restrictions at multiple scopes
- Group-based permission inheritance

### VPN Session Management
- Real-time connection tracking
- Traffic monitoring and billing data
- Configuration lifecycle management
- Certificate and key storage

### Security Features
- OAuth CSRF protection with state tokens
- Soft deletes for audit compliance
- Session management with expiry
- Multi-level IP whitelisting

### Performance Considerations
- Indexed foreign keys for fast joins
- Composite primary keys for junction tables
- JSONB for flexible structured data
- Soft deletes to preserve audit trail

## 📈 Scaling Considerations

### Read Optimization
- Index on frequently queried columns (user.email, session.session_id)
- Junction table indexes for permission lookups
- Audit log partitioning by date

### Write Optimization
- Bulk operations for user imports
- Batch audit log writes
- Asynchronous session cleanup

### Storage Optimization
- VPN config compression
- Audit log archival strategy
- Soft delete cleanup policies

This database design provides a robust foundation for the Dvarpala VPN system with comprehensive audit trails, flexible access control, and scalable architecture for enterprise deployments.