#!/bin/bash
#
# Dvarpala firewall: the walled garden.
#
# Routes tell a client where to send packets. This decides which packets the
# server will actually forward - it is what makes the captive portal a
# restriction rather than a suggestion.
#
# Design
# ------
# One ipset holds the tunnel IPs of authenticated clients. Everything arriving
# on a tun interface passes through one chain:
#
#   authenticated source        -> ACCEPT   (their routes decide reachability)
#   destined for the portal     -> ACCEPT
#   DNS                         -> ACCEPT   (needed to resolve the OAuth hosts)
#   destined for an OAuth host  -> ACCEPT   (so people can actually sign in)
#   anything else               -> DROP
#
# Why an ipset rather than per-client rules:
#
# The previous design marked packets from unauthenticated clients and inserted
# a "set mark 0" rule to promote someone. That could never work: MARK is a
# non-terminating target, so traversal continued to the next rule and the
# packet was immediately re-marked as unauthenticated. ACCEPT and DROP do
# terminate, and set membership is a single O(1) lookup regardless of how many
# clients are connected.
#
# Usage:
#   dvarpala-firewall.sh setup            create chains, sets and base rules
#   dvarpala-firewall.sh allow   <ip>     promote a client (authenticated)
#   dvarpala-firewall.sh revoke  <ip>     demote a client
#   dvarpala-firewall.sh status           show rules, sets and counters

set -uo pipefail

PORTAL_IP="${DVARPALA_PORTAL_IP:-172.30.100.1}"
VPN_SUBNET="${DVARPALA_VPN_SUBNET:-172.30.100.0/24}"
WAN_IF="${DVARPALA_WAN_IF:-$(ip route show default 2>/dev/null | awk '/default/{print $5; exit}')}"

CHAIN="DVARPALA"
AUTH_SET="dvarpala_auth"
OAUTH_SET="dvarpala_oauth"

die() { echo "error: $*" >&2; exit 1; }

require_root() {
    [[ $EUID -eq 0 ]] || die "must run as root"
}

# OAuth provider address ranges, so an unauthenticated user can reach a login
# page. Hostnames are deliberately not used: iptables and ipset resolve a name
# once, at insertion, and CDN-backed endpoints move.
oauth_ranges() {
    cat <<'EOF'
20.190.128.0/18
40.126.0.0/18
13.107.6.0/24
13.107.9.0/24
172.217.0.0/16
172.253.0.0/16
142.250.0.0/15
74.125.0.0/16
140.82.112.0/20
192.30.252.0/22
185.199.108.0/22
35.231.145.151/32
34.74.90.64/28
34.74.226.0/24
EOF
}

setup() {
    require_root
    command -v ipset >/dev/null || die "ipset is not installed (apt-get install ipset)"

    echo "Setting up Dvarpala firewall"
    echo "  vpn subnet: $VPN_SUBNET"
    echo "  portal:     $PORTAL_IP"
    echo "  wan iface:  ${WAN_IF:-<none found>}"

    # Sets. Created empty; membership changes as clients authenticate.
    ipset create "$AUTH_SET"  hash:ip  -exist
    ipset create "$OAUTH_SET" hash:net -exist
    ipset flush "$OAUTH_SET"
    while read -r cidr; do
        [[ -n "$cidr" ]] && ipset add "$OAUTH_SET" "$cidr" -exist
    done < <(oauth_ranges)

    # Chain. Rebuilt from scratch so repeated runs are idempotent.
    iptables -N "$CHAIN" 2>/dev/null || true
    iptables -F "$CHAIN"

    # Established traffic first: replies to permitted requests must return.
    iptables -A "$CHAIN" -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT

    # Authenticated clients: allowed onward. What they can actually reach is
    # governed by the routes their client-connect hook pushed.
    iptables -A "$CHAIN" -m set --match-set "$AUTH_SET" src -j ACCEPT

    # The walled garden, for everyone else.
    iptables -A "$CHAIN" -d "$PORTAL_IP" -j ACCEPT
    iptables -A "$CHAIN" -p udp --dport 53 -j ACCEPT
    iptables -A "$CHAIN" -p tcp --dport 53 -j ACCEPT
    iptables -A "$CHAIN" -m set --match-set "$OAUTH_SET" dst -j ACCEPT

    # Log a sample of refusals, then refuse.
    iptables -A "$CHAIN" -m limit --limit 1/min -j LOG --log-prefix "[dvarpala-blocked] "
    iptables -A "$CHAIN" -j DROP

    # Send tunnel traffic through the chain, exactly once.
    iptables -C FORWARD -i tun+ -j "$CHAIN" 2>/dev/null || \
        iptables -I FORWARD 1 -i tun+ -j "$CHAIN"

    # NAT, so permitted traffic can reach the wider network.
    if [[ -n "$WAN_IF" ]]; then
        iptables -t nat -C POSTROUTING -s "$VPN_SUBNET" -o "$WAN_IF" -j MASQUERADE 2>/dev/null || \
            iptables -t nat -A POSTROUTING -s "$VPN_SUBNET" -o "$WAN_IF" -j MASQUERADE
    fi

    sysctl -qw net.ipv4.ip_forward=1

    echo "  done. unauthenticated clients can reach the portal, DNS and OAuth only."
}

allow() {
    require_root
    local ip="${1:-}"; [[ -n "$ip" ]] || die "usage: $0 allow <ip>"
    ipset add "$AUTH_SET" "$ip" -exist || die "could not add $ip"
    echo "allowed $ip"
}

revoke() {
    require_root
    local ip="${1:-}"; [[ -n "$ip" ]] || die "usage: $0 revoke <ip>"
    ipset del "$AUTH_SET" "$ip" -exist 2>/dev/null
    # Drop existing flows, or an open connection would survive revocation.
    command -v conntrack >/dev/null && conntrack -D -s "$ip" >/dev/null 2>&1
    echo "revoked $ip"
}

status() {
    echo "── authenticated clients ──"
    ipset list "$AUTH_SET" 2>/dev/null | sed -n '/Members/,$p' | tail -n +2 | sed 's/^/  /'
    echo "── chain (packets/bytes) ──"
    iptables -L "$CHAIN" -v -n --line-numbers 2>/dev/null | sed 's/^/  /'
}

case "${1:-}" in
    setup)  setup ;;
    allow)  allow "${2:-}" ;;
    revoke) revoke "${2:-}" ;;
    status) status ;;
    *) echo "usage: $0 {setup|allow <ip>|revoke <ip>|status}"; exit 1 ;;
esac
