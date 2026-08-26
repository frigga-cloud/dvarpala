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
PORTAL_IP="${DVARPALA_PORTAL_IP:-172.30.100.1}"
DNS="${DVARPALA_DNS:-8.8.8.8}"

mkdir -p "$(dirname "$LOG")" 2>/dev/null

log() { echo "$(date '+%Y-%m-%d %H:%M:%S') $*" >> "$LOG"; }

# A client is given one of two shapes, and which one is the whole design.
#
#   before signing in   the default route, so we carry everything
#   after signing in    only the routes to what that person may reach
#
# walled_garden is the first. The default route is taken deliberately, and it
# is the reason the walled garden is a restriction rather than a suggestion.
# Withholding a route is not a denial: a laptop not told to send something
# through the tunnel sends it over its own wifi instead, where this server
# never sees it and cannot refuse it. Taking the default route means every
# packet arrives here to be judged - and, for a web request, to be answered
# with the sign-in page.
#
# DNS goes with it, because a client whose traffic we are carrying and which
# has no resolver cannot look anything up at all, including the sign-in page
# it is being sent to.
#
# The explicit portal route is redundant while the default route is in place.
# It is what still works if a client declines the default route for reasons of
# its own.
walled_garden() {
    echo 'push "redirect-gateway def1 bypass-dhcp"'
    echo "push \"route $PORTAL_IP 255.255.255.255\""
    echo "push \"dhcp-option DNS $DNS\""
}

# work_only is the second shape: the company's addresses through the tunnel,
# everything else left alone.
#
# Note what is absent - redirect-gateway. Signing in gives this person their
# own network back. Their personal traffic never touches this server, which is
# what makes this a tool for reaching work rather than a tap on everything
# they do.
#
# This is only safe because of the order it happens in. A client holds the
# default route from the moment it connects until the moment it authenticates,
# so there is no window in which it is both unidentified and free to go where
# it likes. Dvarpala forces the reconnect that applies this the instant a
# sign-in completes.
work_only() {
    echo "push \"route $PORTAL_IP 255.255.255.255\""
    echo "push \"dhcp-option DNS $DNS\""
}

CLIENT_IP="${ifconfig_pool_remote_ip:-}"
CN="${common_name:-unknown}"

log "connect: cn=$CN tunnel_ip=$CLIENT_IP real_ip=${trusted_ip:-?}"

if [[ -z "$CLIENT_IP" ]]; then
    log "  no tunnel IP supplied by OpenVPN; granting captive portal only"
    walled_garden > "$CONFIG_FILE"
    exit 0
fi

# Start from the walled garden. Anything below that authenticates will lift it.
[[ -x "$FIREWALL" ]] && "$FIREWALL" revoke "$CLIENT_IP" >/dev/null 2>&1

# Ask Dvarpala what this client may reach.
# The certificate's common name travels with the request. A session is stored
# against the tunnel address, and tunnel addresses are reused: without this,
# whoever is handed this address next inherits whatever the last holder had
# earned, for as long as the disconnect grace period lasts.
CN_ARG=$(printf '%s' "$CN" | sed 's/[^a-zA-Z0-9@._-]/_/g')

RESPONSE=$(curl -sf --max-time 5 \
    "$API/api/internal/vpn/access/$CLIENT_IP?cn=$CN_ARG" 2>/dev/null)
CURL_STATUS=$?

if [[ $CURL_STATUS -ne 0 || -z "$RESPONSE" ]]; then
    # Dvarpala is unreachable. Fail CLOSED: grant portal access only.
    # Failing open would hand full network access to everyone whenever the
    # management server is down.
    log "  ERROR: could not reach Dvarpala at $API (curl exit $CURL_STATUS)"
    log "  failing closed: captive portal only"
    walled_garden > "$CONFIG_FILE"
    exit 0
fi

AUTHENTICATED=$(echo "$RESPONSE" | python3 -c \
    'import json,sys; print(str(json.load(sys.stdin).get("authenticated", False)).lower())' 2>/dev/null)

if [[ "$AUTHENTICATED" != "true" ]]; then
    REASON=$(echo "$RESPONSE" | python3 -c \
        'import json,sys; print(json.load(sys.stdin).get("reason",""))' 2>/dev/null)
    log "  not authenticated: $REASON"
    log "  granting captive portal only"
    walled_garden > "$CONFIG_FILE"
    exit 0
fi

EMAIL=$(echo "$RESPONSE" | python3 -c \
    'import json,sys; print(json.load(sys.stdin).get("email",""))' 2>/dev/null)

# Signed in: the default route goes away and only the company's addresses are
# carried from here on.
work_only > "$CONFIG_FILE"

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
