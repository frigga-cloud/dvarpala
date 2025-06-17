#!/bin/bash

# Basic OpenVPN Authentication Script for Dvarpala
# This script provides initial authentication before the full Dvarpala system is operational

# Log file
LOG_FILE="/var/log/openvpn/auth.log"

# Function to log messages
log_message() {
    echo "$(date '+%Y-%m-%d %H:%M:%S') - $1" >> "$LOG_FILE"
}

# Get credentials from environment variables set by OpenVPN
USERNAME="$username"
PASSWORD="$password"
CLIENT_IP="${untrusted_ip:-unknown}"

log_message "Authentication attempt: user=$USERNAME, ip=$CLIENT_IP"

# Basic authentication logic
# In the full system, this will check the database and user authorization status
if [[ "$USERNAME" == "temp_user" && "$PASSWORD" == "temp_portal_access" ]]; then
    log_message "Basic authentication successful for $USERNAME from $CLIENT_IP"
    exit 0  # Success
else
    log_message "Authentication failed for $USERNAME from $CLIENT_IP"
    exit 1  # Failure
fi