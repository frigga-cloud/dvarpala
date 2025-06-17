# Dvarpala VPN Database Schema Documentation

This document provides a comprehensive overview of all database models and their relationships in the Dvarpala VPN system.

## Core Entity Models

### 1. User Model (`users` table)
**Purpose**: Represents system users with OAuth authentication and VPN access

| Column | Type | Purpose |
|--------|------|---------|
| `id` | uint | Primary identifier for the user |
| `email` | string | User's email address - unique identifier for OAuth validation |
| `full_name` | string | User's full display name from OAuth provider |
| `department` | string | User's department/organization unit for grouping |
| `status` | UserStatus | Current account status (active/inactive/suspended) |
| `oauth_provider` | string | OAuth provider used (google, microsoft, github) |
| `last_login` | *time.Time | Timestamp of user's last successful login |
| `created_at` | time.Time | When user account was created |
| `updated_at` | time.Time | When user account was last modified |
| `deleted_at` | *time.Time | Soft delete timestamp |

**Relationships**:
- Many-to-many with `groups` via `user_groups`
- One-to-many with `vpn_sessions`
- One-to-many with `audit_logs`

### 2. Group Model (`groups` table)
**Purpose**: Collection of users with shared access permissions and network routes, supports hierarchy

| Column | Type | Purpose |
|--------|------|---------|
| `id` | uint | Primary identifier for the group |
| `name` | string | Unique name of the group |
| `description` | text | Human-readable description of group's purpose |
| `parent_id` | *uint | Foreign key to parent group (for hierarchy) |
| `created_at` | time.Time | When group was created |
| `updated_at` | time.Time | When group was last modified |
| `deleted_at` | *time.Time | Soft delete timestamp |

**Relationships**:
- Self-referencing hierarchy (parent/children)
- Many-to-many with `users` via `user_groups`
- Many-to-many with `resources` via `group_permissions`
- Many-to-many with `network_routes` via `group_network_routes`

### 3. Resource Model (`resources` table)
**Purpose**: Protected system resources that require access control

| Column | Type | Purpose |
|--------|------|---------|
| `id` | uint | Primary identifier for the resource |
| `name` | string | Human-readable name of the resource |
| `type` | ResourceType | Category (dashboard/vm/database/service) |
| `url` | string | Web URL for dashboard/service resources |
| `ip_address` | string | IP address for VM/database resources |
| `port` | int | Network port for VM/database resources |
| `description` | text | Detailed description of resource purpose |
| `created_at` | time.Time | When resource was registered |
| `updated_at` | time.Time | When resource was last updated |
| `deleted_at` | *time.Time | Soft delete timestamp |

**Relationships**:
- Many-to-many with `groups` via `group_permissions`

## VPN Infrastructure Models

### 4. VPNSession Model (`vpn_sessions` table)
**Purpose**: Tracks individual VPN connections for monitoring and billing

| Column | Type | Purpose |
|--------|------|---------|
| `id` | uint | Primary identifier for the VPN session |
| `user_id` | uint | Foreign key to user who initiated session |
| `client_ip` | string | Client's IP in VPN network |
| `server_ip` | string | VPN server's IP address |
| `status` | SessionStatus | Current session status (active/disconnected/expired) |
| `connected_at` | time.Time | When VPN connection was established |
| `disconnected_at` | *time.Time | When VPN connection ended |
| `bytes_in` | uint64 | Total bytes received by client |
| `bytes_out` | uint64 | Total bytes sent by client |
| `created_at` | time.Time | When session record was created |
| `updated_at` | time.Time | When session was last updated |
| `deleted_at` | *time.Time | Soft delete timestamp |

**Relationships**:
- Many-to-one with `users`

### 5. VPNConfig Model (`vpn_configs` table)
**Purpose**: Stores VPN client configurations and certificates

| Column | Type | Purpose |
|--------|------|---------|
| `id` | uint | Primary identifier for the configuration |
| `user_id` | uint | Foreign key to user who owns config |
| `config_name` | string | Human-readable name for configuration |
| `client_cert` | text | Client certificate in PEM format |
| `client_key` | text | Client private key in PEM format |
| `ca_cert` | text | Certificate Authority certificate |
| `config_data` | text | Complete OpenVPN configuration file |
| `status` | VPNConfigStatus | Configuration status (active/inactive/revoked) |
| `expires_at` | *time.Time | When configuration expires |
| `last_used_at` | *time.Time | When configuration was last used |
| `downloaded_at` | *time.Time | When user downloaded configuration |
| `created_at` | time.Time | When configuration was created |
| `updated_at` | time.Time | When configuration was last updated |
| `deleted_at` | *time.Time | Soft delete timestamp |

**Relationships**:
- Many-to-one with `users`

### 6. NetworkRoute Model (`network_routes` table)
**Purpose**: Defines network routing rules for VPN clients based on group membership

| Column | Type | Purpose |
|--------|------|---------|
| `id` | uint | Primary identifier for the route |
| `name` | string | Human-readable name for route |
| `destination` | string | Destination network in CIDR notation |
| `gateway` | string | Gateway IP address for routing |
| `route_type` | RouteType | Access level (captive_portal/full_access/restricted) |
| `priority` | int | Priority for route ordering (lower = higher priority) |
| `is_active` | bool | Flag to enable/disable route |
| `description` | string | Detailed description of route purpose |
| `created_at` | time.Time | When route was created |
| `updated_at` | time.Time | When route was last updated |
| `deleted_at` | *time.Time | Soft delete timestamp |

**Relationships**:
- Many-to-many with `groups` via `group_network_routes`

## Authentication & Security Models

### 7. Session Model (`sessions` table)
**Purpose**: User web application sessions for maintaining login state

| Column | Type | Purpose |
|--------|------|---------|
| `id` | uint | Primary identifier for session |
| `session_id` | string | Unique session identifier for cookies |
| `user_id` | uint | Foreign key to user who owns session |
| `ip_address` | string | IP address where session was created |
| `user_agent` | string | Browser user agent for security |
| `expires_at` | time.Time | When session becomes invalid |
| `last_used_at` | time.Time | Last activity timestamp |
| `created_at` | time.Time | When session was created |
| `updated_at` | time.Time | When session was last updated |
| `deleted_at` | *time.Time | Soft delete timestamp |

**Relationships**:
- Many-to-one with `users`

### 8. OAuthState Model (`oauth_states` table)
**Purpose**: OAuth state tokens for CSRF protection during OAuth flows

| Column | Type | Purpose |
|--------|------|---------|
| `id` | uint | Primary identifier for OAuth state |
| `state` | string | Unique random state token for CSRF protection |
| `provider` | string | OAuth provider name |
| `user_ip` | string | IP address of user initiating OAuth |
| `user_agent` | string | Browser user agent |
| `expires_at` | time.Time | When state token expires |
| `used` | bool | Flag if token has been consumed |
| `created_at` | time.Time | When token was created |
| `updated_at` | time.Time | When token was last updated |
| `deleted_at` | *time.Time | Soft delete timestamp |

### 9. IPWhitelist Model (`ip_whitelists` table)
**Purpose**: IP-based access control for enhanced security

| Column | Type | Purpose |
|--------|------|---------|
| `id` | uint | Primary identifier for IP restriction |
| `ip_address` | string | Single IP address that is allowed |
| `cidr` | string | CIDR notation for IP range |
| `type` | IPWhitelistType | Scope (user/group/global) |
| `user_id` | *uint | Foreign key for user-specific restrictions |
| `group_id` | *uint | Foreign key for group-specific restrictions |
| `description` | string | Human-readable description |
| `is_active` | bool | Flag to enable/disable restriction |
| `expires_at` | *time.Time | When restriction expires |
| `created_at` | time.Time | When restriction was created |
| `updated_at` | time.Time | When restriction was last updated |
| `deleted_at` | *time.Time | Soft delete timestamp |

**Relationships**:
- Many-to-one with `users` (optional)
- Many-to-one with `groups` (optional)

## Audit & Logging Models

### 10. AuditLog Model (`audit_logs` table)
**Purpose**: Tracks all user actions and system events for security and compliance

| Column | Type | Purpose |
|--------|------|---------|
| `id` | uint | Primary identifier for audit entry |
| `user_id` | *uint | Foreign key to user who performed action |
| `action` | string | Description of action performed |
| `resource_type` | string | Type of resource affected |
| `resource_id` | *uint | ID of specific resource affected |
| `ip_address` | string | IP address where action was performed |
| `user_agent` | string | Browser/client user agent |
| `details` | jsonb | Additional structured data about action |
| `created_at` | time.Time | When action occurred |
| `deleted_at` | *time.Time | Soft delete timestamp |

**Relationships**:
- Many-to-one with `users`

## Junction Table Models

### 11. UserGroup Model (`user_groups` table)
**Purpose**: Many-to-many relationship between users and groups

| Column | Type | Purpose |
|--------|------|---------|
| `user_id` | uint | Foreign key to user |
| `group_id` | uint | Foreign key to group |
| `created_at` | time.Time | When membership was created |
| `updated_at` | time.Time | When membership was last updated |

### 12. GroupPermission Model (`group_permissions` table)
**Purpose**: Many-to-many relationship between groups and resource permissions

| Column | Type | Purpose |
|--------|------|---------|
| `group_id` | uint | Foreign key to group |
| `resource_id` | uint | Foreign key to resource |
| `permission_type` | PermissionType | Type of permission (read/write/admin/ssh/full) |
| `created_at` | time.Time | When permission was granted |
| `updated_at` | time.Time | When permission was last updated |

### 13. GroupNetworkRoute Model (`group_network_routes` table)
**Purpose**: Many-to-many relationship between groups and network routes

| Column | Type | Purpose |
|--------|------|---------|
| `group_id` | uint | Foreign key to group |
| `network_route_id` | uint | Foreign key to network route |
| `created_at` | time.Time | When access was granted |

## Deprecated Models

### 14. Permission Model (`permissions` table)
**Purpose**: Legacy permission model (deprecated in favor of GroupPermission)

This model is maintained for backward compatibility but new implementations should use the `GroupPermission` junction table instead.

## Enums and Constants

### UserStatus
- `active` - User can access the system
- `inactive` - User account is disabled
- `suspended` - User is temporarily blocked

### ResourceType
- `dashboard` - Web dashboard or UI interface
- `vm` - Virtual machine or server
- `database` - Database server
- `service` - API service or microservice

### SessionStatus (VPN)
- `active` - Session is currently connected
- `disconnected` - Session ended normally
- `expired` - Session timed out

### VPNConfigStatus
- `active` - Configuration is valid and usable
- `inactive` - Configuration is disabled
- `revoked` - Configuration has been revoked for security

### PermissionType
- `read` - View-only access
- `write` - Read and modify access
- `admin` - Full administrative access
- `ssh` - SSH/remote access to VM resources
- `full` - Complete access to all functions

### RouteType
- `captive_portal` - Limited access for authentication
- `full_access` - Complete network access
- `restricted` - Limited access to specific resources

### IPWhitelistType
- `user` - IP restriction for specific user
- `group` - IP restriction for group members
- `global` - System-wide IP restriction

## Database Relationships Summary

```
Users ──┬── UserGroups ──── Groups (hierarchical)
        │                     │
        ├── VPNSessions       ├── GroupPermissions ──── Resources
        │                     │
        ├── VPNConfigs        └── GroupNetworkRoutes ──── NetworkRoutes
        │
        ├── Sessions
        │
        ├── IPWhitelists
        │
        └── AuditLogs
```

This schema provides:
- **Flexible user management** with OAuth integration
- **Hierarchical group structure** for organizational alignment
- **Granular access control** with resource-level permissions
- **Comprehensive VPN management** with session tracking
- **Enhanced security** with IP whitelisting and audit trails
- **Network-level access control** with routing rules