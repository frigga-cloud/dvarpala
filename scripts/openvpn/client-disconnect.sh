#!/bin/bash
#
# Dvarpala OpenVPN client-disconnect hook.
#
# Tells Dvarpala the client has gone. The session is shortened to a brief
# grace window rather than deleted, so that a reconnect - which is how a newly
# authenticated client receives its routes - can still succeed.
#
# Install with, in server.conf:
#   client-disconnect /opt/dvarpala/scripts/client-disconnect.sh

set -uo pipefail

API="${DVARPALA_API:-http://127.0.0.1:8080}"
LOG="/var/log/openvpn/dvarpala-connect.log"
FIREWALL="${DVARPALA_FIREWALL:-/opt/dvarpala/scripts/dvarpala-firewall.sh}"

mkdir -p "$(dirname "$LOG")" 2>/dev/null
log() { echo "$(date '+%Y-%m-%d %H:%M:%S') $*" >> "$LOG"; }

CLIENT_IP="${ifconfig_pool_remote_ip:-}"
CN="${common_name:-unknown}"

log "disconnect: cn=$CN tunnel_ip=$CLIENT_IP duration=${time_duration:-?}s " \
    "bytes_in=${bytes_received:-?} bytes_out=${bytes_sent:-?}"

if [[ -z "$CLIENT_IP" ]]; then
    log "  no tunnel IP; nothing to revoke"
    exit 0
fi

# Put the client back behind the walled garden immediately. This must happen
# even if Dvarpala is unreachable below.
if [[ -x "$FIREWALL" ]]; then
    "$FIREWALL" revoke "$CLIENT_IP" >/dev/null 2>&1 && \
        log "  firewall: $CLIENT_IP returned to the walled garden"
fi

if curl -sf --max-time 5 -X DELETE \
        "$API/api/internal/vpn/session/$CLIENT_IP" > /dev/null 2>&1; then
    log "  session moved to grace period for $CLIENT_IP"
else
    # Not fatal: Redis expiry will remove the session anyway. But it means the
    # user could reconnect without signing in until the TTL runs out, so it is
    # worth alerting on.
    log "  WARNING: could not revoke session for $CLIENT_IP (Dvarpala unreachable)"
fi

exit 0
