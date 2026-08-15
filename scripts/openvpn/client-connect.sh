#!/bin/bash
#
# Dvarpala OpenVPN client-connect hook.
#
# OpenVPN runs this every time a client connects, and reads the file it writes
# at $1 to decide what to push to that one client.
#
# OpenVPN provides these environment variables:
#   $common_name              the client certificate / username
#   $ifconfig_pool_remote_ip  the tunnel IP this client was given
#   $trusted_ip               the client's real internet address
#
# Install with, in server.conf:
#   script-security 3
#   client-connect /opt/dvarpala/scripts/client-connect.sh

set -uo pipefail

CONFIG_FILE="$1"
API="${DVARPALA_API:-http://127.0.0.1:8080}"
LOG="/var/log/openvpn/dvarpala-connect.log"
FIREWALL="${DVARPALA_FIREWALL:-/opt/dvarpala/scripts/dvarpala-firewall.sh}"

mkdir -p "$(dirname "$LOG")" 2>/dev/null

log() { echo "$(date '+%Y-%m-%d %H:%M:%S') $*" >> "$LOG"; }

CLIENT_IP="${ifconfig_pool_remote_ip:-}"
CN="${common_name:-unknown}"

log "connect: cn=$CN tunnel_ip=$CLIENT_IP real_ip=${trusted_ip:-?}"

if [[ -z "$CLIENT_IP" ]]; then
    log "  no tunnel IP supplied by OpenVPN; granting captive portal only"
    echo 'push "route 172.30.100.1 255.255.255.255"' > "$CONFIG_FILE"
    exit 0
fi

# Start from the walled garden. Anything below that authenticates will lift it.
[[ -x "$FIREWALL" ]] && "$FIREWALL" revoke "$CLIENT_IP" >/dev/null 2>&1

# Ask Dvarpala what this client may reach.
RESPONSE=$(curl -sf --max-time 5 "$API/api/internal/vpn/access/$CLIENT_IP" 2>/dev/null)
CURL_STATUS=$?

if [[ $CURL_STATUS -ne 0 || -z "$RESPONSE" ]]; then
    # Dvarpala is unreachable. Fail CLOSED: grant portal access only.
    # Failing open would hand full network access to everyone whenever the
    # management server is down.
    log "  ERROR: could not reach Dvarpala at $API (curl exit $CURL_STATUS)"
    log "  failing closed: captive portal only"
    echo 'push "route 172.30.100.1 255.255.255.255"' > "$CONFIG_FILE"
    exit 0
fi

AUTHENTICATED=$(echo "$RESPONSE" | python3 -c \
    'import json,sys; print(str(json.load(sys.stdin).get("authenticated", False)).lower())' 2>/dev/null)

if [[ "$AUTHENTICATED" != "true" ]]; then
    REASON=$(echo "$RESPONSE" | python3 -c \
        'import json,sys; print(json.load(sys.stdin).get("reason",""))' 2>/dev/null)
    log "  not authenticated: $REASON"
    log "  granting captive portal only"
    echo 'push "route 172.30.100.1 255.255.255.255"' > "$CONFIG_FILE"
    exit 0
fi

EMAIL=$(echo "$RESPONSE" | python3 -c \
    'import json,sys; print(json.load(sys.stdin).get("email",""))' 2>/dev/null)

# Write one push line per route the user is entitled to.
: > "$CONFIG_FILE"
COUNT=0
while IFS=$'\t' read -r network netmask resource; do
    [[ -z "$network" ]] && continue
    echo "push \"route $network $netmask\"" >> "$CONFIG_FILE"
    log "  route $network $netmask  ($resource)"
    COUNT=$((COUNT + 1))
done < <(echo "$RESPONSE" | python3 -c '
import json, sys
for r in json.load(sys.stdin).get("routes", []):
    print("\t".join([r["network"], r["netmask"], r.get("resource", "")]))
' 2>/dev/null)

# DNS, so the routed names resolve.
echo 'push "dhcp-option DNS 8.8.8.8"' >> "$CONFIG_FILE"

# Routes alone are only half of it: the firewall must also stop dropping this
# client's traffic.
if [[ -x "$FIREWALL" ]]; then
    if "$FIREWALL" allow "$CLIENT_IP" >/dev/null 2>&1; then
        log "  firewall: $CLIENT_IP promoted out of the walled garden"
    else
        log "  WARNING: firewall promotion failed for $CLIENT_IP"
    fi
fi

log "  authenticated as $EMAIL; pushed $COUNT route(s)"
exit 0
