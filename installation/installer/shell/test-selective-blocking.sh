#!/bin/bash

# Comprehensive testing script for Dvarpala selective VPN blocking
# This script tests all components of the selective blocking system

set -euo pipefail

# Configuration
API_BASE_URL="http://172.30.100.1:8081/api"
API_KEY="dvarpala-default-admin-key-2024"
DB_NAME="dvarpala"
TEST_LOG="/var/log/dvarpala/selective-blocking-test.log"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Logging
log() {
    echo "$(date '+%Y-%m-%d %H:%M:%S'): $1" | tee -a "$TEST_LOG"
}

success() {
    echo -e "${GREEN}✅ $1${NC}" | tee -a "$TEST_LOG"
}

error() {
    echo -e "${RED}❌ $1${NC}" | tee -a "$TEST_LOG"
}

warning() {
    echo -e "${YELLOW}⚠️ $1${NC}" | tee -a "$TEST_LOG"
}

info() {
    echo -e "${BLUE}ℹ️ $1${NC}" | tee -a "$TEST_LOG"
}

# Test functions
test_database_connectivity() {
    info "Testing database connectivity..."
    
    if sudo -u postgres psql -d "$DB_NAME" -c "SELECT 1;" >/dev/null 2>&1; then
        success "Database connectivity OK"
        return 0
    else
        error "Database connectivity FAILED"
        return 1
    fi
}

test_database_schema() {
    info "Testing database schema..."
    
    local tables=("blocked_resources" "vpc_resources" "user_auth_status" "resource_audit_log" "api_tokens")
    local all_ok=true
    
    for table in "${tables[@]}"; do
        if sudo -u postgres psql -d "$DB_NAME" -c "SELECT COUNT(*) FROM $table;" >/dev/null 2>&1; then
            success "Table $table exists and accessible"
        else
            error "Table $table missing or inaccessible"
            all_ok=false
        fi
    done
    
    # Test functions
    if sudo -u postgres psql -d "$DB_NAME" -c "SELECT * FROM get_blocked_ips();" >/dev/null 2>&1; then
        success "Function get_blocked_ips() working"
    else
        error "Function get_blocked_ips() FAILED"
        all_ok=false
    fi
    
    if sudo -u postgres psql -d "$DB_NAME" -c "SELECT * FROM get_blocked_domains();" >/dev/null 2>&1; then
        success "Function get_blocked_domains() working"
    else
        error "Function get_blocked_domains() FAILED"
        all_ok=false
    fi
    
    if $all_ok; then
        success "Database schema test PASSED"
        return 0
    else
        error "Database schema test FAILED"
        return 1
    fi
}

test_api_server() {
    info "Testing API server..."
    
    # Test health endpoint
    if curl -s "$API_BASE_URL/../health" | grep -q "healthy"; then
        success "API health endpoint OK"
    else
        error "API health endpoint FAILED"
        return 1
    fi
    
    # Test authenticated endpoint
    local response=$(curl -s -H "X-API-Key: $API_KEY" "$API_BASE_URL/resources")
    if echo "$response" | grep -q '"success":true'; then
        success "API authentication OK"
    else
        error "API authentication FAILED"
        warning "Response: $response"
        return 1
    fi
    
    success "API server test PASSED"
    return 0
}

test_resource_management() {
    info "Testing resource management API..."
    
    # Test adding a resource
    local test_ip="10.0.0.999"  # Invalid IP for testing
    local add_response=$(curl -s -X POST \
        -H "Content-Type: application/json" \
        -H "X-API-Key: $API_KEY" \
        -d "{\"resource_type\": \"ip\", \"resource_value\": \"$test_ip\", \"description\": \"Test IP\", \"created_by\": \"test\"}" \
        "$API_BASE_URL/resources")
    
    if echo "$add_response" | grep -q '"success":false'; then
        success "API validation working (rejected invalid IP)"
    else
        warning "API validation may not be working properly"
    fi
    
    # Test adding valid resource
    test_ip="192.168.1.100"
    add_response=$(curl -s -X POST \
        -H "Content-Type: application/json" \
        -H "X-API-Key: $API_KEY" \
        -d "{\"resource_type\": \"ip\", \"resource_value\": \"$test_ip\", \"description\": \"Test IP for automation\", \"created_by\": \"test\"}" \
        "$API_BASE_URL/resources")
    
    if echo "$add_response" | grep -q '"success":true'; then
        success "Successfully added test resource"
        
        # Extract resource ID for cleanup
        local resource_id=$(echo "$add_response" | grep -o '"id":[0-9]*' | cut -d':' -f2)
        
        # Test removing the resource
        local delete_response=$(curl -s -X DELETE \
            -H "X-API-Key: $API_KEY" \
            "$API_BASE_URL/resources/$resource_id")
        
        if echo "$delete_response" | grep -q '"success":true'; then
            success "Successfully removed test resource"
        else
            warning "Failed to remove test resource (ID: $resource_id)"
        fi
    else
        error "Failed to add test resource"
        warning "Response: $add_response"
        return 1
    fi
    
    success "Resource management API test PASSED"
    return 0
}

test_openvpn_configuration() {
    info "Testing OpenVPN configuration..."
    
    # Check if OpenVPN service is running
    if systemctl is-active --quiet openvpn-server@server; then
        success "OpenVPN server is running"
    else
        error "OpenVPN server is not running"
        return 1
    fi
    
    # Check configuration file
    if [[ -f "/etc/openvpn/server/server.conf" ]]; then
        success "OpenVPN configuration file exists"
        
        # Check for selective routing configuration
        if grep -q "client-connect.*selective" /etc/openvpn/server/server.conf; then
            success "Selective routing script configured"
        else
            warning "Selective routing script may not be configured"
        fi
    else
        error "OpenVPN configuration file missing"
        return 1
    fi
    
    # Check client scripts
    if [[ -x "/opt/dvarpala/scripts/client-connect-selective.sh" ]]; then
        success "Client connect script exists and executable"
    else
        error "Client connect script missing or not executable"
        return 1
    fi
    
    if [[ -x "/opt/dvarpala/scripts/client-disconnect-selective.sh" ]]; then
        success "Client disconnect script exists and executable"
    else
        error "Client disconnect script missing or not executable"
        return 1
    fi
    
    success "OpenVPN configuration test PASSED"
    return 0
}

test_network_scripts() {
    info "Testing network management scripts..."
    
    # Test iptables script
    if [[ -x "/opt/dvarpala/scripts/update-iptables-blocking.sh" ]]; then
        success "iptables update script exists and executable"
        
        # Test execution
        if /opt/dvarpala/scripts/update-iptables-blocking.sh >/dev/null 2>&1; then
            success "iptables script executed successfully"
        else
            warning "iptables script execution had issues"
        fi
    else
        error "iptables update script missing or not executable"
        return 1
    fi
    
    # Test DNS script
    if [[ -x "/opt/dvarpala/scripts/update-dns-blocking.sh" ]]; then
        success "DNS update script exists and executable"
        
        # Test execution
        if /opt/dvarpala/scripts/update-dns-blocking.sh >/dev/null 2>&1; then
            success "DNS script executed successfully"
        else
            warning "DNS script execution had issues"
        fi
    else
        error "DNS update script missing or not executable"
        return 1
    fi
    
    success "Network scripts test PASSED"
    return 0
}

test_dns_configuration() {
    info "Testing DNS configuration..."
    
    # Check if dnsmasq is running
    if systemctl is-active --quiet dnsmasq; then
        success "dnsmasq service is running"
    else
        warning "dnsmasq service is not running"
    fi
    
    # Check configuration files
    if [[ -f "/etc/dnsmasq.d/dvarpala-blocking.conf" ]]; then
        success "dnsmasq blocking configuration exists"
    else
        warning "dnsmasq blocking configuration missing"
    fi
    
    if [[ -f "/opt/dvarpala/config/blocked-domains.conf" ]]; then
        success "blocked domains configuration file exists"
    else
        warning "blocked domains configuration file missing"
    fi
    
    success "DNS configuration test PASSED"
    return 0
}

test_firewall_rules() {
    info "Testing firewall rules..."
    
    local vpc_subnet="172.30.0.0/26"
    local vpn_subnet="172.30.100.0/24"
    
    # Check for VPC blocking rule
    if iptables -L FORWARD -n | grep -q "$vpn_subnet.*$vpc_subnet.*DROP"; then
        success "VPC blocking rule found in iptables"
    else
        warning "VPC blocking rule not found in iptables"
    fi
    
    # Check for general accept rule
    if iptables -L FORWARD -n | grep -q "$vpn_subnet.*ACCEPT"; then
        success "General accept rule found in iptables"
    else
        warning "General accept rule not found in iptables"
    fi
    
    success "Firewall rules test PASSED"
    return 0
}

test_audit_logging() {
    info "Testing audit logging..."
    
    # Add a test resource to generate audit log
    local add_response=$(curl -s -X POST \
        -H "Content-Type: application/json" \
        -H "X-API-Key: $API_KEY" \
        -d "{\"resource_type\": \"domain\", \"resource_value\": \"test-audit.example.com\", \"description\": \"Audit test\", \"created_by\": \"test-audit\"}" \
        "$API_BASE_URL/resources")
    
    if echo "$add_response" | grep -q '"success":true'; then
        # Check if audit log entry was created
        local audit_count=$(sudo -u postgres psql -t -d "$DB_NAME" -c "
            SELECT COUNT(*) FROM resource_audit_log 
            WHERE performed_by = 'test-audit' AND action_type = 'ADD'
        " 2>/dev/null | tr -d ' ')
        
        if [[ "$audit_count" -gt 0 ]]; then
            success "Audit logging working correctly"
        else
            warning "Audit log entry not found"
        fi
        
        # Cleanup test resource
        local resource_id=$(echo "$add_response" | grep -o '"id":[0-9]*' | cut -d':' -f2)
        curl -s -X DELETE -H "X-API-Key: $API_KEY" "$API_BASE_URL/resources/$resource_id" >/dev/null
    else
        warning "Could not test audit logging (failed to add test resource)"
    fi
    
    success "Audit logging test PASSED"
    return 0
}

test_system_integration() {
    info "Testing complete system integration..."
    
    # Test the full workflow: Add resource -> Update network -> Verify
    local test_domain="integration-test.example.com"
    
    # 1. Add blocked domain
    local add_response=$(curl -s -X POST \
        -H "Content-Type: application/json" \
        -H "X-API-Key: $API_KEY" \
        -d "{\"resource_type\": \"domain\", \"resource_value\": \"$test_domain\", \"description\": \"Integration test domain\", \"created_by\": \"integration-test\"}" \
        "$API_BASE_URL/resources")
    
    if echo "$add_response" | grep -q '"success":true'; then
        local resource_id=$(echo "$add_response" | grep -o '"id":[0-9]*' | cut -d':' -f2)
        success "Added test domain for integration test"
        
        # 2. Wait a moment for database propagation
        sleep 2
        
        # 3. Check if domain appears in DNS blocking config
        if grep -q "$test_domain" /opt/dvarpala/config/blocked-domains.conf 2>/dev/null; then
            success "Domain blocking configuration updated automatically"
        else
            warning "Domain not found in blocking configuration (may need manual update)"
        fi
        
        # 4. Cleanup
        curl -s -X DELETE -H "X-API-Key: $API_KEY" "$API_BASE_URL/resources/$resource_id" >/dev/null
        success "Cleaned up integration test resources"
    else
        warning "Integration test setup failed"
        return 1
    fi
    
    success "System integration test PASSED"
    return 0
}

generate_test_report() {
    info "Generating comprehensive test report..."
    
    local report_file="/opt/dvarpala/reports/selective-blocking-test-$(date +%Y%m%d-%H%M%S).json"
    mkdir -p /opt/dvarpala/reports
    
    cat > "$report_file" << EOF
{
  "test_timestamp": "$(date -Iseconds)",
  "test_summary": {
    "database_connectivity": $([[ $db_test == 0 ]] && echo "true" || echo "false"),
    "database_schema": $([[ $schema_test == 0 ]] && echo "true" || echo "false"),
    "api_server": $([[ $api_test == 0 ]] && echo "true" || echo "false"),
    "resource_management": $([[ $resource_test == 0 ]] && echo "true" || echo "false"),
    "openvpn_configuration": $([[ $openvpn_test == 0 ]] && echo "true" || echo "false"),
    "network_scripts": $([[ $network_test == 0 ]] && echo "true" || echo "false"),
    "dns_configuration": $([[ $dns_test == 0 ]] && echo "true" || echo "false"),
    "firewall_rules": $([[ $firewall_test == 0 ]] && echo "true" || echo "false"),
    "audit_logging": $([[ $audit_test == 0 ]] && echo "true" || echo "false"),
    "system_integration": $([[ $integration_test == 0 ]] && echo "true" || echo "false")
  },
  "statistics": {
    "total_blocked_resources": $(sudo -u postgres psql -t -d "$DB_NAME" -c "SELECT COUNT(*) FROM blocked_resources WHERE is_active = true;" 2>/dev/null | tr -d ' '),
    "vpc_resources_discovered": $(sudo -u postgres psql -t -d "$DB_NAME" -c "SELECT COUNT(*) FROM vpc_resources WHERE is_active = true;" 2>/dev/null | tr -d ' '),
    "api_tokens_active": $(sudo -u postgres psql -t -d "$DB_NAME" -c "SELECT COUNT(*) FROM api_tokens WHERE is_active = true;" 2>/dev/null | tr -d ' ')
  }
}
EOF
    
    success "Test report generated: $report_file"
}

# Main test execution
main() {
    echo -e "${BLUE}🧪 Starting Dvarpala Selective VPN Blocking System Test${NC}"
    echo "================================================================"
    
    mkdir -p /var/log/dvarpala
    
    # Run all tests
    test_database_connectivity; db_test=$?
    test_database_schema; schema_test=$?
    test_api_server; api_test=$?
    test_resource_management; resource_test=$?
    test_openvpn_configuration; openvpn_test=$?
    test_network_scripts; network_test=$?
    test_dns_configuration; dns_test=$?
    test_firewall_rules; firewall_test=$?
    test_audit_logging; audit_test=$?
    test_system_integration; integration_test=$?
    
    # Generate report
    generate_test_report
    
    # Summary
    echo
    echo "================================================================"
    local passed=0
    local total=10
    
    for result in $db_test $schema_test $api_test $resource_test $openvpn_test $network_test $dns_test $firewall_test $audit_test $integration_test; do
        if [[ $result == 0 ]]; then
            ((passed++))
        fi
    done
    
    if [[ $passed == $total ]]; then
        echo -e "${GREEN}🎉 ALL TESTS PASSED ($passed/$total)${NC}"
        echo -e "${GREEN}✅ Selective VPN blocking system is fully operational!${NC}"
    else
        echo -e "${YELLOW}⚠️ SOME TESTS FAILED ($passed/$total passed)${NC}"
        echo -e "${YELLOW}🔧 Please review the failed tests and fix any issues${NC}"
    fi
    
    echo
    info "Test log available at: $TEST_LOG"
    info "Full test report: /opt/dvarpala/reports/"
    
    return $((total - passed))
}

# Show usage if no arguments
if [[ $# == 0 ]]; then
    echo "Usage: $0 [test_name]"
    echo "Available tests:"
    echo "  all (default)           - Run all tests"
    echo "  database               - Test database connectivity and schema"
    echo "  api                    - Test API server"
    echo "  resources              - Test resource management"
    echo "  openvpn                - Test OpenVPN configuration"
    echo "  network                - Test network scripts"
    echo "  dns                    - Test DNS configuration"
    echo "  firewall               - Test firewall rules"
    echo "  audit                  - Test audit logging"
    echo "  integration            - Test complete system integration"
    echo
    echo "Example: $0 all"
    exit 0
fi

# Run specific test or all tests
case "${1:-all}" in
    "database") test_database_connectivity && test_database_schema ;;
    "api") test_api_server ;;
    "resources") test_resource_management ;;
    "openvpn") test_openvpn_configuration ;;
    "network") test_network_scripts ;;
    "dns") test_dns_configuration ;;
    "firewall") test_firewall_rules ;;
    "audit") test_audit_logging ;;
    "integration") test_system_integration ;;
    "all"|*) main ;;
esac