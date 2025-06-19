# Dvarpala Resource Management API Documentation

The Dvarpala Resource Management API provides endpoints for managing selective VPN blocking, allowing administrators to control which resources are blocked for VPN clients while allowing direct internet access.

## Overview

**Base URL**: `http://172.30.100.1:8081/api`  
**Authentication**: API Key required in `X-API-Key` header  
**Content-Type**: `application/json`

## Traffic Flow Model

### Selective Blocking Architecture
```
VPN Client Traffic Flow:
├── Internet Traffic (YouTube, etc.) → Direct Route ✅ ALLOWED
├── Blocked IPs/Domains → VPN Tunnel → Server → DROP ❌ BLOCKED  
├── VPC Resources → VPN Tunnel → Server → DROP ❌ BLOCKED
└── Captive Portal → VPN Tunnel → Server → ALLOW ✅ (if unauthenticated)
```

### Key Benefits
- **Cost Effective**: Only blocked traffic uses VM bandwidth
- **Performance**: Direct internet access at full speed
- **Granular Control**: Block specific resources, not everything
- **Dynamic**: Real-time updates without VPN reconnection

## Authentication

All API endpoints require an API key passed in the request header:

```bash
X-API-Key: your-api-key-here
```

### Creating API Keys

API keys are stored in the `api_tokens` table. To create one:

```sql
INSERT INTO api_tokens (token_hash, token_name, permissions, created_by) 
VALUES ('your-secure-api-key', 'Admin Key', '{"read": true, "write": true, "admin": true}', 'admin');
```

## Resource Management Endpoints

### 1. List Blocked Resources

**GET** `/api/resources`

Returns all blocked resources in the system.

**Response:**
```json
{
  "success": true,
  "message": "Found 5 blocked resources",
  "data": [
    {
      "id": 1,
      "resource_type": "ip",
      "resource_value": "172.30.0.10",
      "description": "Internal database server",
      "created_by": "admin",
      "created_at": "2024-01-15T10:30:00Z",
      "updated_at": "2024-01-15T10:30:00Z",
      "is_active": true
    }
  ]
}
```

### 2. Add Blocked Resource

**POST** `/api/resources`

Adds a new resource to the blocked list.

**Request Body:**
```json
{
  "resource_type": "ip|domain|cidr|vpc_resource",
  "resource_value": "172.30.0.10",
  "description": "Internal database server",
  "created_by": "admin"
}
```

**Resource Types:**
- `ip`: Single IP address (e.g., "192.168.1.10")
- `domain`: Domain name (e.g., "internal.company.com")
- `cidr`: CIDR network (e.g., "10.0.0.0/24")
- `vpc_resource`: Reference to auto-discovered VPC resource

**Response:**
```json
{
  "success": true,
  "message": "Successfully added ip 172.30.0.10 to blocked resources",
  "data": {
    "id": 123
  }
}
```

### 3. Remove Blocked Resource

**DELETE** `/api/resources/{id}`

Removes a resource from the blocked list.

**Response:**
```json
{
  "success": true,
  "message": "Successfully removed ip 172.30.0.10 from blocked resources"
}
```

### 4. Bulk Add Resources

**POST** `/api/resources/bulk`

Adds multiple resources in a single request.

**Request Body:**
```json
{
  "created_by": "admin",
  "resources": [
    {
      "resource_type": "ip",
      "resource_value": "172.30.0.10",
      "description": "Database server"
    },
    {
      "resource_type": "domain", 
      "resource_value": "internal.company.com",
      "description": "Internal portal"
    }
  ]
}
```

**Response:**
```json
{
  "success": true,
  "message": "Successfully added 2 out of 2 resources",
  "data": {
    "added_resources": ["ip:172.30.0.10", "domain:internal.company.com"]
  }
}
```

## VPC Resource Discovery

### 5. List VPC Resources

**GET** `/api/vpc-resources`

Returns auto-discovered VPC resources.

**Response:**
```json
{
  "success": true,
  "message": "Found 3 VPC resources",
  "data": [
    {
      "id": 1,
      "resource_id": "i-1234567890abcdef0",
      "resource_type": "vm",
      "ip_address": "172.30.0.10",
      "domain_name": null,
      "port_range": null,
      "cloud_provider": "aws",
      "vpc_id": "vpc-12345678",
      "auto_discovered": true,
      "last_scan": "2024-01-15T10:30:00Z",
      "is_active": true
    }
  ]
}
```

### 6. Trigger VPC Discovery

**POST** `/api/discover-vpc-resources`

Triggers automatic discovery of VPC resources.

**Response:**
```json
{
  "success": true,
  "message": "VPC resource discovery triggered"
}
```

## Example Usage

### Block a Specific IP Address

```bash
curl -X POST http://172.30.100.1:8081/api/resources \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-api-key" \
  -d '{
    "resource_type": "ip",
    "resource_value": "10.0.0.100",
    "description": "Internal file server",
    "created_by": "admin"
  }'
```

### Block Multiple Company Domains

```bash
curl -X POST http://172.30.100.1:8081/api/resources/bulk \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-api-key" \
  -d '{
    "created_by": "admin",
    "resources": [
      {
        "resource_type": "domain",
        "resource_value": "internal.company.com",
        "description": "Internal company portal"
      },
      {
        "resource_type": "domain", 
        "resource_value": "admin.company.com",
        "description": "Admin interface"
      },
      {
        "resource_type": "cidr",
        "resource_value": "192.168.100.0/24",
        "description": "DMZ network"
      }
    ]
  }'
```

### List All Blocked Resources

```bash
curl -X GET http://172.30.100.1:8081/api/resources \
  -H "X-API-Key: your-api-key"
```

### Remove a Blocked Resource

```bash
curl -X DELETE http://172.30.100.1:8081/api/resources/123 \
  -H "X-API-Key: your-api-key"
```

## Error Responses

All endpoints return consistent error responses:

```json
{
  "success": false,
  "error": "Error description here"
}
```

**Common HTTP Status Codes:**
- `200`: Success
- `400`: Bad Request (validation error)
- `401`: Unauthorized (invalid API key)
- `404`: Resource not found
- `500`: Internal server error

## Network Update Process

When resources are added or removed, the system automatically:

1. **Updates Database**: Adds/removes entries in `blocked_resources` table
2. **Updates iptables**: Runs `/opt/dvarpala/scripts/update-iptables-blocking.sh`
3. **Updates DNS**: Runs `/opt/dvarpala/scripts/update-dns-blocking.sh`
4. **Logs Action**: Records action in `resource_audit_log` table

**No VPN reconnection required** - changes take effect immediately for new connections and existing connections will get updated routes.

## Integration Examples

### Web Dashboard Integration

```javascript
// Add blocked resource via web interface
async function blockResource(type, value, description) {
  const response = await fetch('/api/resources', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-API-Key': getApiKey()
    },
    body: JSON.stringify({
      resource_type: type,
      resource_value: value,
      description: description,
      created_by: getCurrentUser()
    })
  });
  
  return await response.json();
}
```

### Automated Security Integration

```bash
#!/bin/bash
# Block suspicious IPs from security logs

API_KEY="your-api-key"
SUSPICIOUS_IPS=$(grep "FAILED_LOGIN" /var/log/auth.log | awk '{print $11}' | sort -u)

for ip in $SUSPICIOUS_IPS; do
  curl -X POST http://172.30.100.1:8081/api/resources \
    -H "Content-Type: application/json" \
    -H "X-API-Key: $API_KEY" \
    -d "{
      \"resource_type\": \"ip\",
      \"resource_value\": \"$ip\",
      \"description\": \"Blocked due to failed login attempts\",
      \"created_by\": \"security-automation\"
    }"
done
```

## Monitoring and Audit

### Audit Log Query

```sql
-- View recent resource management actions
SELECT 
  action_type,
  resource_type,
  resource_value,
  performed_by,
  performed_at,
  client_ip
FROM resource_audit_log 
ORDER BY performed_at DESC 
LIMIT 50;
```

### Resource Statistics

```sql
-- Get resource blocking statistics
SELECT 
  resource_type,
  COUNT(*) as total_count,
  SUM(CASE WHEN is_active THEN 1 ELSE 0 END) as active_count
FROM blocked_resources 
GROUP BY resource_type;
```

## Best Practices

### Security
1. **Rotate API Keys**: Regularly update API tokens
2. **Limit Permissions**: Use read-only keys for monitoring
3. **Monitor Usage**: Review audit logs regularly
4. **Validate Input**: Always validate resource values before adding

### Performance
1. **Batch Operations**: Use bulk endpoints for multiple resources
2. **Cleanup**: Regularly remove inactive resources
3. **Monitor Impact**: Check iptables rule count (limit: ~1000 rules)

### Operational
1. **Test Changes**: Verify blocking works as expected
2. **Documentation**: Document why resources are blocked
3. **Automation**: Use scripts for routine blocking tasks
4. **Monitoring**: Set up alerts for API failures

## Troubleshooting

### API Not Responding
```bash
# Check API service status
systemctl status dvarpala-api

# Check logs
tail -f /var/log/dvarpala/api.log
```

### Database Connection Issues
```bash
# Test database connectivity
sudo -u postgres psql -d dvarpala -c "SELECT COUNT(*) FROM blocked_resources;"
```

### Network Rules Not Applied
```bash
# Check iptables rules
iptables -L FORWARD -n | grep 172.30.100

# Manually update rules
/opt/dvarpala/scripts/update-iptables-blocking.sh
```

### DNS Blocking Not Working
```bash
# Check dnsmasq status
systemctl status dnsmasq

# Test DNS resolution
dig @172.30.100.1 blocked-domain.com

# Update DNS rules
/opt/dvarpala/scripts/update-dns-blocking.sh
```