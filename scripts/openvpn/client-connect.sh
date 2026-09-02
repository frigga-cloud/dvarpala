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

# Two resolvers, because a client in the walled garden and a client that has
# signed in are in different situations.
#
# Unidentified, it is given this machine: it cannot reach any other resolver,
# which is what closes DNS as a way out of the garden, and this one answers
# "signin" with the sign-in page so somebody can be told an address they can
# remember.
#
# Signed in, the tunnel carries only the company's addresses and everything
# else goes over the person's own connection - so their own resolver is the
# right one, and taking it over would be reaching further into their machine
# than this needs to.
GARDEN_DNS="${DVARPALA_GARDEN_DNS:-$PORTAL_IP}"
WORK_DNS="${DVARPALA_DNS:-}"

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
    echo "push \"dhcp-option DNS $GARDEN_DNS\""
}

# open_mail_briefly lets this one client reach mail for a few minutes, so the
# sign-in code it is about to be sent can actually be read.
#
# Per client and self-expiring, because the ranges involved are Google's,
# Microsoft's and Apple's, and none of them separate mail from everything else
# they run. Opened for everybody permanently - which is what this used to be -
# it is not a walled garden at all.
#
# Never fatal: a client that cannot reach webmail can still read the code on a
# phone, which is what most people do anyway.
open_mail_briefly() {
    local client="${1:-}"
    [[ -n "$client" && -x "$FIREWALL" ]] || return 0
    "$FIREWALL" mail-open "$client" >/dev/null 2>&1 \
        && log "  mail opened briefly for $client so the code can be read" \
        || log "  note: could not open mail for $client; the code can still be read elsewhere"
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
    # Only if this deployment names one. Empty means the client keeps the
    # resolver it already had, which is what a split tunnel implies.
    [[ -n "$WORK_DNS" ]] && echo "push \"dhcp-option DNS $WORK_DNS\""
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
    open_mail_briefly "$CLIENT_IP"
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
    open_mail_briefly "$CLIENT_IP"
    exit 0
fi

EMAIL=$(echo "$RESPONSE" | python3 -c \
    'import json,sys; print(json.load(sys.stdin).get("email",""))' 2>/dev/null)

# Signed in: the default route goes away and only the company's addresses are
# carried from here on.
work_only > "$CONFIG_FILE"

# Each route is read twice over: as a pair for the client's routing table,
# which is what OpenVPN wants, and as a range for the firewall, which is what
# ipset wants.
#
# The two must agree. Handing the firewall a bare 10.0.5.0 where the route
# covers 10.0.5.0/24 opens the name of the range and none of the machines in
# it - the client sends all of them down the tunnel and every packet is
# dropped. That reads as a broken network rather than a missing permission,
# which is the worst way for a permission to fail.
COUNT=0
DESTINATIONS=()
while IFS=$'\t' read -r network netmask cidr resource; do
    [[ -z "$network" ]] && continue
    echo "push \"route $network $netmask\"" >> "$CONFIG_FILE"
    DESTINATIONS+=("$cidr")
    log "  route $network $netmask  -> firewall $cidr  ($resource)"
    COUNT=$((COUNT + 1))
done < <(echo "$RESPONSE" | python3 -c '
import ipaddress, json, sys

for r in json.load(sys.stdin).get("routes", []):
    network = r["network"]
    netmask = r["netmask"]
    try:
        # strict=False so a host address with a wider mask is accepted rather
        # than rejected: the operator meant the range it sits in.
        cidr = ipaddress.IPv4Network(f"{network}/{netmask}", strict=False).with_prefixlen
    except ValueError:
        # Unparseable: skip it rather than grant something unintended.
        continue
    print("\t".join([network, netmask, cidr, r.get("resource", "")]))
' 2>/dev/null)

# The routes tell the client where to send packets; this decides which of them
# are forwarded. Only the destinations this person was granted are opened -
# signing in on its own opens nothing.
if [[ -x "$FIREWALL" ]]; then
    if [[ ${#DESTINATIONS[@]} -eq 0 ]]; then
        # Signed in and entitled to nothing. Still marked as signed in, so the
        # portal stops intercepting their web requests and they are told that
        # rather than being sent round to the sign-in page for ever.
        "$FIREWALL" allow "$CLIENT_IP" >/dev/null 2>&1
        log "  no destinations granted; signed in but with nothing to reach"
    elif "$FIREWALL" allow "$CLIENT_IP" "${DESTINATIONS[@]}" >/dev/null 2>&1; then
        log "  firewall: opened ${#DESTINATIONS[@]} destination(s) for $CLIENT_IP"
    else
        log "  WARNING: firewall promotion failed for $CLIENT_IP"
    fi
fi

log "  authenticated as $EMAIL; pushed $COUNT route(s)"
exit 0
