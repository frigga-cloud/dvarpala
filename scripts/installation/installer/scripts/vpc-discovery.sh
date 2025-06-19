#!/bin/bash

# Dvarpala VPC Resource Auto-Discovery Script
# Discovers and catalogs cloud resources for selective blocking

DB_NAME="${DB_NAME:-dvarpala}"
DB_USER="${DB_USER:-dvarpala}"
LOG_FILE="/var/log/dvarpala/vpc-discovery.log"
CONFIG_FILE="/opt/dvarpala/config/cloud-config.json"

# Logging function
log() {
    echo "$(date '+%Y-%m-%d %H:%M:%S'): $1" | tee -a "$LOG_FILE"
}

# Read cloud configuration
get_cloud_config() {
    if [[ ! -f "$CONFIG_FILE" ]]; then
        log "ERROR: Cloud configuration file not found: $CONFIG_FILE"
        exit 1
    fi
    
    CLOUD_PROVIDER=$(jq -r '.cloud.provider' "$CONFIG_FILE")
    CLOUD_REGION=$(jq -r '.cloud.region' "$CONFIG_FILE")
    VPC_NAME=$(jq -r '.resource_names.vpc_name' "$CONFIG_FILE")
    
    log "Cloud Provider: $CLOUD_PROVIDER, Region: $CLOUD_REGION, VPC: $VPC_NAME"
}

# Discover AWS resources
discover_aws_resources() {
    log "Discovering AWS resources in VPC: $VPC_NAME"
    
    # Get VPC ID
    VPC_ID=$(aws ec2 describe-vpcs --filters "Name=tag:Name,Values=$VPC_NAME" --query 'Vpcs[0].VpcId' --output text 2>/dev/null)
    
    if [[ "$VPC_ID" == "None" || -z "$VPC_ID" ]]; then
        log "WARNING: VPC $VPC_NAME not found in AWS"
        return 1
    fi
    
    log "Found VPC ID: $VPC_ID"
    
    # Discover EC2 instances
    log "Discovering EC2 instances..."
    aws ec2 describe-instances \
        --filters "Name=vpc-id,Values=$VPC_ID" "Name=instance-state-name,Values=running,pending,stopped" \
        --query 'Reservations[].Instances[].[InstanceId,PrivateIpAddress,PublicIpAddress,Tags[?Key==`Name`].Value|[0],InstanceType]' \
        --output text | while read -r instance_id private_ip public_ip name instance_type; do
        
        if [[ -n "$instance_id" && "$instance_id" != "None" ]]; then
            # Insert/update database
            sudo -u postgres psql -d "$DB_NAME" -c "
                INSERT INTO vpc_resources (resource_id, resource_type, ip_address, cloud_provider, vpc_id, auto_discovered, last_scan)
                VALUES ('$instance_id', 'vm', '$private_ip', 'aws', '$VPC_ID', true, NOW())
                ON CONFLICT (resource_id, cloud_provider) 
                DO UPDATE SET ip_address = '$private_ip', last_scan = NOW(), is_active = true;
            " 2>/dev/null
            
            log "Added/Updated EC2 instance: $instance_id ($private_ip)"
            
            # Also add public IP if exists
            if [[ -n "$public_ip" && "$public_ip" != "None" ]]; then
                sudo -u postgres psql -d "$DB_NAME" -c "
                    INSERT INTO vpc_resources (resource_id, resource_type, ip_address, cloud_provider, vpc_id, auto_discovered, last_scan)
                    VALUES ('${instance_id}-public', 'vm', '$public_ip', 'aws', '$VPC_ID', true, NOW())
                    ON CONFLICT (resource_id, cloud_provider) 
                    DO UPDATE SET ip_address = '$public_ip', last_scan = NOW(), is_active = true;
                " 2>/dev/null
                
                log "Added/Updated EC2 public IP: $instance_id ($public_ip)"
            fi
        fi
    done
    
    # Discover RDS instances
    log "Discovering RDS instances..."
    aws rds describe-db-instances \
        --query 'DBInstances[?DBSubnetGroup.VpcId==`'$VPC_ID'`].[DBInstanceIdentifier,Endpoint.Address,Endpoint.Port,DBInstanceClass]' \
        --output text | while read -r db_id endpoint port instance_class; do
        
        if [[ -n "$db_id" && "$db_id" != "None" ]]; then
            sudo -u postgres psql -d "$DB_NAME" -c "
                INSERT INTO vpc_resources (resource_id, resource_type, domain_name, port_range, cloud_provider, vpc_id, auto_discovered, last_scan)
                VALUES ('$db_id', 'database', '$endpoint', '$port', 'aws', '$VPC_ID', true, NOW())
                ON CONFLICT (resource_id, cloud_provider) 
                DO UPDATE SET domain_name = '$endpoint', port_range = '$port', last_scan = NOW(), is_active = true;
            " 2>/dev/null
            
            log "Added/Updated RDS instance: $db_id ($endpoint:$port)"
        fi
    done
    
    # Discover Load Balancers
    log "Discovering Application Load Balancers..."
    aws elbv2 describe-load-balancers \
        --query 'LoadBalancers[?VpcId==`'$VPC_ID'`].[LoadBalancerName,DNSName,LoadBalancerArn]' \
        --output text | while read -r lb_name dns_name lb_arn; do
        
        if [[ -n "$lb_name" && "$lb_name" != "None" ]]; then
            sudo -u postgres psql -d "$DB_NAME" -c "
                INSERT INTO vpc_resources (resource_id, resource_type, domain_name, cloud_provider, vpc_id, auto_discovered, last_scan)
                VALUES ('$lb_name', 'lb', '$dns_name', 'aws', '$VPC_ID', true, NOW())
                ON CONFLICT (resource_id, cloud_provider) 
                DO UPDATE SET domain_name = '$dns_name', last_scan = NOW(), is_active = true;
            " 2>/dev/null
            
            log "Added/Updated ALB: $lb_name ($dns_name)"
        fi
    done
    
    log "AWS resource discovery completed"
}

# Discover GCP resources
discover_gcp_resources() {
    log "Discovering GCP resources in VPC: $VPC_NAME"
    
    PROJECT_ID=$(jq -r '.cloud.project_id' "$CONFIG_FILE")
    
    # Discover Compute Engine instances
    log "Discovering Compute Engine instances..."
    gcloud compute instances list \
        --filter="networkInterfaces.network:$VPC_NAME" \
        --format="value(name,networkInterfaces[0].networkIP,networkInterfaces[0].accessConfigs[0].natIP,zone,machineType.scope(machineTypes))" | while IFS=$'\t' read -r name internal_ip external_ip zone machine_type; do
        
        if [[ -n "$name" ]]; then
            # Add internal IP
            sudo -u postgres psql -d "$DB_NAME" -c "
                INSERT INTO vpc_resources (resource_id, resource_type, ip_address, cloud_provider, vpc_id, auto_discovered, last_scan)
                VALUES ('$name', 'vm', '$internal_ip', 'gcp', '$VPC_NAME', true, NOW())
                ON CONFLICT (resource_id, cloud_provider) 
                DO UPDATE SET ip_address = '$internal_ip', last_scan = NOW(), is_active = true;
            " 2>/dev/null
            
            log "Added/Updated GCE instance: $name ($internal_ip)"
            
            # Add external IP if exists
            if [[ -n "$external_ip" && "$external_ip" != "" ]]; then
                sudo -u postgres psql -d "$DB_NAME" -c "
                    INSERT INTO vpc_resources (resource_id, resource_type, ip_address, cloud_provider, vpc_id, auto_discovered, last_scan)
                    VALUES ('${name}-external', 'vm', '$external_ip', 'gcp', '$VPC_NAME', true, NOW())
                    ON CONFLICT (resource_id, cloud_provider) 
                    DO UPDATE SET ip_address = '$external_ip', last_scan = NOW(), is_active = true;
                " 2>/dev/null
                
                log "Added/Updated GCE external IP: $name ($external_ip)"
            fi
        fi
    done
    
    # Discover Cloud SQL instances
    log "Discovering Cloud SQL instances..."
    gcloud sql instances list \
        --format="value(name,ipAddresses[0].ipAddress,region)" | while IFS=$'\t' read -r instance_name ip_address region; do
        
        if [[ -n "$instance_name" ]]; then
            sudo -u postgres psql -d "$DB_NAME" -c "
                INSERT INTO vpc_resources (resource_id, resource_type, ip_address, cloud_provider, vpc_id, auto_discovered, last_scan)
                VALUES ('$instance_name', 'database', '$ip_address', 'gcp', '$VPC_NAME', true, NOW())
                ON CONFLICT (resource_id, cloud_provider) 
                DO UPDATE SET ip_address = '$ip_address', last_scan = NOW(), is_active = true;
            " 2>/dev/null
            
            log "Added/Updated Cloud SQL instance: $instance_name ($ip_address)"
        fi
    done
    
    log "GCP resource discovery completed"
}

# Discover Azure resources
discover_azure_resources() {
    log "Discovering Azure resources in Resource Group: $VPC_NAME"
    
    # Discover Virtual Machines
    log "Discovering Azure Virtual Machines..."
    az vm list --resource-group "$VPC_NAME" \
        --query '[].{name:name,privateIp:privateIps,publicIp:publicIps,location:location}' \
        --output tsv | while IFS=$'\t' read -r name private_ip public_ip location; do
        
        if [[ -n "$name" ]]; then
            # Add private IP
            sudo -u postgres psql -d "$DB_NAME" -c "
                INSERT INTO vpc_resources (resource_id, resource_type, ip_address, cloud_provider, vpc_id, auto_discovered, last_scan)
                VALUES ('$name', 'vm', '$private_ip', 'azure', '$VPC_NAME', true, NOW())
                ON CONFLICT (resource_id, cloud_provider) 
                DO UPDATE SET ip_address = '$private_ip', last_scan = NOW(), is_active = true;
            " 2>/dev/null
            
            log "Added/Updated Azure VM: $name ($private_ip)"
            
            # Add public IP if exists
            if [[ -n "$public_ip" && "$public_ip" != "null" ]]; then
                sudo -u postgres psql -d "$DB_NAME" -c "
                    INSERT INTO vpc_resources (resource_id, resource_type, ip_address, cloud_provider, vpc_id, auto_discovered, last_scan)
                    VALUES ('${name}-public', 'vm', '$public_ip', 'azure', '$VPC_NAME', true, NOW())
                    ON CONFLICT (resource_id, cloud_provider) 
                    DO UPDATE SET ip_address = '$public_ip', last_scan = NOW(), is_active = true;
                " 2>/dev/null
                
                log "Added/Updated Azure VM public IP: $name ($public_ip)"
            fi
        fi
    done
    
    # Discover SQL Databases
    log "Discovering Azure SQL Databases..."
    az sql server list --resource-group "$VPC_NAME" \
        --query '[].{name:name,fqdn:fullyQualifiedDomainName}' \
        --output tsv | while IFS=$'\t' read -r server_name fqdn; do
        
        if [[ -n "$server_name" ]]; then
            sudo -u postgres psql -d "$DB_NAME" -c "
                INSERT INTO vpc_resources (resource_id, resource_type, domain_name, port_range, cloud_provider, vpc_id, auto_discovered, last_scan)
                VALUES ('$server_name', 'database', '$fqdn', '1433', 'azure', '$VPC_NAME', true, NOW())
                ON CONFLICT (resource_id, cloud_provider) 
                DO UPDATE SET domain_name = '$fqdn', port_range = '1433', last_scan = NOW(), is_active = true;
            " 2>/dev/null
            
            log "Added/Updated Azure SQL Server: $server_name ($fqdn)"
        fi
    done
    
    log "Azure resource discovery completed"
}

# Mark stale resources as inactive
cleanup_stale_resources() {
    log "Cleaning up stale resources..."
    
    # Mark resources as inactive if not seen in last scan
    AFFECTED_ROWS=$(sudo -u postgres psql -t -d "$DB_NAME" -c "
        UPDATE vpc_resources 
        SET is_active = false 
        WHERE auto_discovered = true 
        AND last_scan < NOW() - INTERVAL '1 hour'
        AND is_active = true;
        SELECT ROW_COUNT();
    " 2>/dev/null | tr -d ' ')
    
    log "Marked $AFFECTED_ROWS stale resources as inactive"
}

# Update blocked resources based on VPC discovery
update_blocked_resources() {
    log "Updating blocked resources from VPC discovery..."
    
    # Auto-block all discovered VPC resources
    sudo -u postgres psql -d "$DB_NAME" -c "
        INSERT INTO blocked_resources (resource_type, resource_value, description, created_by, is_active)
        SELECT 
            CASE 
                WHEN ip_address IS NOT NULL THEN 'ip'
                WHEN domain_name IS NOT NULL THEN 'domain'
            END as resource_type,
            COALESCE(ip_address::text, domain_name) as resource_value,
            'Auto-discovered ' || resource_type || ' from ' || cloud_provider || ' VPC' as description,
            'vpc-discovery' as created_by,
            true as is_active
        FROM vpc_resources 
        WHERE is_active = true 
        AND (ip_address IS NOT NULL OR domain_name IS NOT NULL)
        ON CONFLICT (resource_type, resource_value) WHERE is_active = true
        DO UPDATE SET description = EXCLUDED.description, updated_at = NOW();
    " 2>/dev/null
    
    log "Updated blocked resources from VPC discovery"
}

# Generate discovery report
generate_report() {
    log "Generating discovery report..."
    
    TOTAL_RESOURCES=$(sudo -u postgres psql -t -d "$DB_NAME" -c "SELECT COUNT(*) FROM vpc_resources WHERE is_active = true;" 2>/dev/null | tr -d ' ')
    BLOCKED_RESOURCES=$(sudo -u postgres psql -t -d "$DB_NAME" -c "SELECT COUNT(*) FROM blocked_resources WHERE is_active = true;" 2>/dev/null | tr -d ' ')
    
    cat > /opt/dvarpala/reports/vpc-discovery-$(date +%Y%m%d-%H%M%S).json << EOF
{
    "discovery_timestamp": "$(date -Iseconds)",
    "cloud_provider": "$CLOUD_PROVIDER",
    "vpc_name": "$VPC_NAME",
    "total_vpc_resources": $TOTAL_RESOURCES,
    "total_blocked_resources": $BLOCKED_RESOURCES,
    "resources_by_type": $(sudo -u postgres psql -t -d "$DB_NAME" -c "
        SELECT json_object_agg(resource_type, count) 
        FROM (
            SELECT resource_type, COUNT(*) as count 
            FROM vpc_resources 
            WHERE is_active = true 
            GROUP BY resource_type
        ) subquery;" 2>/dev/null)
}
EOF
    
    log "Discovery report generated: $TOTAL_RESOURCES VPC resources, $BLOCKED_RESOURCES blocked resources"
}

# Main execution
main() {
    log "Starting VPC resource discovery..."
    
    # Ensure directories exist
    mkdir -p /var/log/dvarpala
    mkdir -p /opt/dvarpala/reports
    
    # Get cloud configuration
    get_cloud_config
    
    # Discover resources based on cloud provider
    case "$CLOUD_PROVIDER" in
        "aws")
            if command -v aws >/dev/null 2>&1; then
                discover_aws_resources
            else
                log "ERROR: AWS CLI not found"
                exit 1
            fi
            ;;
        "gcp")
            if command -v gcloud >/dev/null 2>&1; then
                discover_gcp_resources
            else
                log "ERROR: gcloud CLI not found"
                exit 1
            fi
            ;;
        "azure")
            if command -v az >/dev/null 2>&1; then
                discover_azure_resources
            else
                log "ERROR: Azure CLI not found"
                exit 1
            fi
            ;;
        *)
            log "ERROR: Unsupported cloud provider: $CLOUD_PROVIDER"
            exit 1
            ;;
    esac
    
    # Post-processing
    cleanup_stale_resources
    update_blocked_resources
    generate_report
    
    # Update network configuration
    /opt/dvarpala/scripts/update-iptables-blocking.sh
    /opt/dvarpala/scripts/update-dns-blocking.sh
    
    log "VPC resource discovery completed successfully"
}

# Run discovery if called directly
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi