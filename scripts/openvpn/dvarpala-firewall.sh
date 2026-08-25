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
# One ipset holds (client, destination) pairs: who may reach what. Everything
# arriving on a tun interface passes through one chain:
#
#   client -> a destination granted to them  -> ACCEPT
#   destined for the portal                  -> ACCEPT
#   DNS                                      -> ACCEPT   (to resolve at all)
#   destined for an OAuth host               -> ACCEPT   (so people can sign in)
#   anything else                            -> DROP
#
# Note what is absent: there is no rule that accepts traffic merely because it
# comes from somebody who has signed in. Authentication decides whether you
# have any destinations at all; it does not decide where you may go. That is
# the difference between a VPN with a login page and Zero Trust, and it used
# to be the other way round here - one rule accepted everything from an
# authenticated source, leaving the routes as the only thing keeping anybody
# to their own resources. Routes are a suggestion. This is not.
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
#   dvarpala-firewall.sh allow <ip> <dst>...  grant a client its destinations
#   dvarpala-firewall.sh revoke <ip>          remove everything a client holds
#   dvarpala-firewall.sh status           show rules, sets and counters

set -uo pipefail

PORTAL_IP="${DVARPALA_PORTAL_IP:-172.30.100.1}"
VPN_SUBNET="${DVARPALA_VPN_SUBNET:-172.30.100.0/24}"
WAN_IF="${DVARPALA_WAN_IF:-$(ip route show default 2>/dev/null | awk '/default/{print $5; exit}')}"

CHAIN="DVARPALA"
ACCESS_SET="dvarpala_access"
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
    # hash:net,net stores a source and a destination together, so one rule and
    # one lookup answer "may this client reach that host" however many clients
    # and resources exist. It takes plain addresses as well as ranges, so a
    # resource may be a single host or a whole subnet.
    ipset create "$ACCESS_SET" hash:net,net -exist
    ipset create "$OAUTH_SET"  hash:net   -exist
    ipset flush "$OAUTH_SET"
    while read -r cidr; do
        [[ -n "$cidr" ]] && ipset add "$OAUTH_SET" "$cidr" -exist
    done < <(oauth_ranges)

    # Chain. Rebuilt from scratch so repeated runs are idempotent.
    iptables -N "$CHAIN" 2>/dev/null || true
    iptables -F "$CHAIN"

    # Established traffic first: replies to permitted requests must return.
    iptables -A "$CHAIN" -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT

    # Granted destinations, per client. Nothing else about being signed in
    # opens anything.
    iptables -A "$CHAIN" -m set --match-set "$ACCESS_SET" src,dst -j ACCEPT

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

    echo "  done. everyone reaches the portal, DNS and OAuth; anything more"
    echo "  must be granted per client."
}

# allow grants one client the destinations it is entitled to.
#
# Called with every destination at once rather than one per invocation: a
# client is granted its whole set in a single step, so there is no window in
# which it holds half of them.
allow() {
    require_root
    local ip="${1:-}"; [[ -n "$ip" ]] || die "usage: $0 allow <client-ip> <destination>..."
    shift
    [[ $# -gt 0 ]] || die "usage: $0 allow <client-ip> <destination>..."

    local granted=0 dest
    for dest in "$@"; do
        if ipset add "$ACCESS_SET" "$ip,$dest" -exist; then
            granted=$((granted + 1))
        fi
    done

    echo "allowed $ip to reach $granted destination(s)"
}

# revoke removes every destination a client holds.
#
# ipset cannot delete by a partial key, so the client's entries are found and
# removed individually. Missing this would leave a disconnected client's
# grants in place for whoever is given that tunnel address next.
revoke() {
    require_root
    local ip="${1:-}"; [[ -n "$ip" ]] || die "usage: $0 revoke <ip>"

    local removed=0 member
    while read -r member; do
        [[ -n "$member" ]] || continue
        ipset del "$ACCESS_SET" "$member" -exist 2>/dev/null && removed=$((removed + 1))
    done < <(ipset list "$ACCESS_SET" 2>/dev/null |
             sed -n '/^Members:/,$p' | tail -n +2 |
             awk -v c="$ip" -F, '$1 == c {print $0}')

    # Drop existing flows, or an open connection would survive revocation.
    command -v conntrack >/dev/null && conntrack -D -s "$ip" >/dev/null 2>&1
    echo "revoked $ip ($removed destination(s))"
}

status() {
    echo "── who may reach what (client,destination) ──"
    ipset list "$ACCESS_SET" 2>/dev/null | sed -n '/Members/,$p' | tail -n +2 | sed 's/^/  /'
    echo "── chain (packets/bytes) ──"
    iptables -L "$CHAIN" -v -n --line-numbers 2>/dev/null | sed 's/^/  /'
}

case "${1:-}" in
    setup)  setup ;;
    allow)  shift; allow "$@" ;;
    revoke) revoke "${2:-}" ;;
    status) status ;;
    *) echo "usage: $0 {setup|allow <client-ip> <destination>...|revoke <ip>|status}"; exit 1 ;;
esac
