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
#   client -> a destination granted to it -> ACCEPT
#   destined for the portal     -> ACCEPT
#   DNS, to this server only    -> ACCEPT   (so anything can be resolved at all)
#   destined for a mail service -> ACCEPT   (so the code can be read)
#   anything else, over TCP     -> REJECT  (so a browser fails at once)
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
#   dvarpala-firewall.sh allow <ip> <dst>...  grant a client its destinations
#   dvarpala-firewall.sh revoke <ip>          remove everything a client holds
#   dvarpala-firewall.sh status           show rules, sets and counters

set -uo pipefail

PORTAL_IP="${DVARPALA_PORTAL_IP:-172.30.100.1}"
PORTAL_PORT="${DVARPALA_PORTAL_PORT:-8080}"
VPN_SUBNET="${DVARPALA_VPN_SUBNET:-172.30.100.0/24}"
WAN_IF="${DVARPALA_WAN_IF:-$(ip route show default 2>/dev/null | awk '/default/{print $5; exit}')}"

CHAIN="DVARPALA"
NAT_CHAIN="DVARPALA_PORTAL"
ACCESS_SET="dvarpala_access"
SIGNEDIN_SET="dvarpala_signedin"
MAIL_SET="dvarpala_mail"

# How long a signed-out client may reach mail for, in seconds. Long enough to
# open a webmail tab and read a code that takes a minute or two to arrive;
# short enough that a client which connects and does nothing is not left with
# a way out of the garden. A code itself lasts five minutes.
MAIL_WINDOW="${DVARPALA_MAIL_WINDOW:-600}"

die() { echo "error: $*" >&2; exit 1; }

require_root() {
    [[ $EUID -eq 0 ]] || die "must run as root"
}

# Addresses an unidentified client may reach, beyond the sign-in page.
#
# There is exactly one reason to allow anything here: the sign-in code arrives
# by email, and somebody has to be able to read it. If their mail client
# cannot reach its server, the code sits unread and the only way in is blocked
# by the thing they are signing in to. Observed: imap.gmail.com refused while
# the message waited, and it appeared the moment the tunnel was closed.
#
# So this is a list of mail services, not of login providers. It began as OAuth
# provider ranges, which stopped being needed the day sign-in became codes by
# email and would otherwise have sat here opening far more than was necessary.
#
# Kept as address ranges rather than names: iptables and ipset resolve a name
# once, at insertion, and a large mail service moves between many addresses.
#
# Add the ranges of whatever mail service your people actually use. Every one
# of these is a small hole in the garden - an unidentified client can hold a
# conversation with anything inside them - so keep the list short.
mail_ranges() {
    cat <<'EOF'
# Google - Gmail and Workspace, web and IMAP
142.250.0.0/15
172.217.0.0/16
74.125.0.0/16
192.178.0.0/15
209.85.128.0/17
64.233.160.0/19
66.102.0.0/20
# Microsoft - Outlook and Office 365
40.92.0.0/15
40.107.0.0/16
52.100.0.0/14
104.47.0.0/17
40.104.0.0/15
# Apple - iCloud Mail
17.0.0.0/8
EOF
}

# portal_interception answers a signed-out client's web requests with the
# sign-in page, whatever it asked for.
#
# Dropping those requests is not enough. A browser asked for a host it cannot
# reach waits, and then reports that the site is down - so the person is told
# their internet is broken rather than that they need to sign in, and never
# sees the portal at all.
#
# Rewriting the destination instead means anything they open lands on the
# sign-in page. It is also what makes a phone or laptop notice by itself:
# every operating system tests its connection by fetching a known URL over
# plain HTTP, and an answer that is not the expected one is precisely how it
# decides to show its "sign in to network" panel.
#
# Only port 80. An HTTPS request cannot be answered by anyone but the site it
# was addressed to - that is the point of it - so those are refused by the
# filter chain instead, with a reset so the browser says so straight away.
# Every captive portal behaves this way.
portal_interception() {
    iptables -t nat -N "$NAT_CHAIN" 2>/dev/null || true
    iptables -t nat -F "$NAT_CHAIN"

    # Signed in: their web traffic is their own and is not rewritten. Without
    # this, an internal dashboard on port 80 would be answered by the sign-in
    # page instead of by the dashboard.
    iptables -t nat -A "$NAT_CHAIN" -m set --match-set "$SIGNEDIN_SET" src -j RETURN

    # Everyone else asking for a web page gets the sign-in page - including
    # somebody who asked for the portal itself.
    #
    # There used to be a rule above this returning anything addressed to the
    # portal, on the reasoning that it needed no rewriting. It never did
    # protect anything: this chain is only entered for port 80, and the portal
    # listens on 8080, so its own traffic never arrives here. What it did do
    # was break the one address people are given. Told to open http://signin,
    # a browser asks for port 80 on this machine, that rule sent it through
    # untouched, and nothing was listening - so the address that exists to be
    # memorable was the one address that did not work.
    iptables -t nat -A "$NAT_CHAIN" -p tcp --dport 80 \
        -j DNAT --to-destination "$PORTAL_IP:$PORTAL_PORT"

    iptables -t nat -C PREROUTING -i tun+ -p tcp --dport 80 -j "$NAT_CHAIN" 2>/dev/null || \
        iptables -t nat -I PREROUTING 1 -i tun+ -p tcp --dport 80 -j "$NAT_CHAIN"
}

setup() {
    require_root
    command -v ipset >/dev/null || die "ipset is not installed (apt-get install ipset)"

    echo "Setting up Dvarpala firewall"
    echo "  vpn subnet: $VPN_SUBNET"
    echo "  portal:     $PORTAL_IP"
    echo "  wan iface:  ${WAN_IF:-<none found>}"

    # Sets. Created empty; membership changes as clients authenticate.
    # hash:net,net holds a source and a destination together, so one rule and
    # one lookup answer "may this client reach that host" however many clients
    # and resources there are. It takes plain addresses as well as ranges, so
    # a resource may be one machine or a whole subnet.
    ipset create "$ACCESS_SET" hash:net,net -exist

    # A plain list of clients who have signed in, used for one thing only:
    # deciding whether to rewrite a web request to the sign-in page. That
    # question is about the client alone, and a pair cannot answer it.
    #
    # It grants nothing. Being in it does not open a single destination.
    ipset create "$SIGNEDIN_SET" hash:ip -exist

    # Mail is opened per client and expires by itself.
    #
    # This used to be a plain list of destinations, matched on destination
    # alone: every unidentified client could reach every range in it, for as
    # long as the server was up. Since the ranges are Google's, Microsoft's
    # and Apple's - and none of them separate mail from search, storage or
    # anything else they run - that opened most of the useful internet to
    # anybody holding a certificate, which is the opposite of a walled garden.
    #
    # Now each entry is a (client, destination) pair carrying a timeout, added
    # when that client connects unauthenticated and removed by the kernel a
    # few minutes later. Nobody else inherits it, and nothing has to remember
    # to clean it up.
    # Chain. Rebuilt from scratch so repeated runs are idempotent.
    iptables -N "$CHAIN" 2>/dev/null || true
    iptables -F "$CHAIN"

    # Now the chain holds no rules, nothing references the sets, and an old
    # one can be replaced. This matters on an upgrade: the previous version
    # created MAIL_SET as hash:net, and "create -exist" will not change the
    # type of a set that already exists - it succeeds and leaves the old one,
    # after which the src,dst rule below cannot be added at all. Silently, so
    # the garden would come back up with no mail rule and no complaint.
    if ipset list -t "$MAIL_SET" >/dev/null 2>&1; then
        if ! ipset list -t "$MAIL_SET" 2>/dev/null | grep -q "Type: hash:net,net"; then
            echo "  replacing $MAIL_SET, which is the previous per-destination kind"
            ipset destroy "$MAIL_SET" 2>/dev/null || true
        fi
    fi
    ipset create "$MAIL_SET" hash:net,net timeout "$MAIL_WINDOW" -exist

    # Established traffic first: replies to permitted requests must return.
    iptables -A "$CHAIN" -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT

    # Granted destinations, per client.
    #
    # Note what is absent: nothing accepts traffic merely because it comes
    # from somebody signed in. Authentication decides whether you have any
    # destinations at all; it does not decide where you may go.
    #
    # It used to be the other way round - one rule accepted everything from an
    # authenticated source, leaving the pushed routes as the only thing
    # keeping anybody to their own resources. Routes are instructions to the
    # client's own machine. Adding one by hand was enough to reach a server
    # that had never been granted, and nothing recorded it. Demonstrated on a
    # live server, in three packets.
    iptables -A "$CHAIN" -m set --match-set "$ACCESS_SET" src,dst -j ACCEPT

    # The walled garden, for everyone else.
    iptables -A "$CHAIN" -d "$PORTAL_IP" -j ACCEPT

    # DNS to this server only.
    #
    # A signed-out client is given the resolver on this machine, and nothing
    # else. Allowing port 53 to anywhere - which is what this did before - left
    # a two-way channel out of the walled garden that everything else here
    # exists to prevent: DNS carries arbitrary data in both directions, slowly
    # but reliably, and tunnelling over it is a well-worn technique.
    #
    # Traffic to the resolver arrives on INPUT rather than FORWARD, because the
    # resolver is this machine. These rules cover a client that was pushed a
    # different resolver and has not noticed yet.
    iptables -A "$CHAIN" -p udp --dport 53 -d "$PORTAL_IP" -j ACCEPT
    iptables -A "$CHAIN" -p tcp --dport 53 -d "$PORTAL_IP" -j ACCEPT

    # So a person can read the code that was just sent to them - but only the
    # client this was opened for, and only until it expires.
    iptables -A "$CHAIN" -m set --match-set "$MAIL_SET" src,dst -j ACCEPT

    # Log a sample of refusals, then refuse.
    iptables -A "$CHAIN" -m limit --limit 1/min -j LOG --log-prefix "[dvarpala-blocked] "

    # Refuse a connection outright rather than letting it hang.
    #
    # An HTTPS request cannot be answered by anybody but the site it was
    # addressed to - that is what TLS is for - so it cannot be turned into the
    # sign-in page the way a plain HTTP one is. Since nearly every site is
    # HTTPS now, that is what most people meet first.
    #
    # Dropping it silently leaves the browser waiting a minute before saying
    # the site took too long, which reads as a broken network. A reset tells
    # it at once: the browser reports a refused connection within a second,
    # and any captive-portal check the operating system makes reaches its
    # verdict immediately instead of timing out.
    iptables -A "$CHAIN" -p tcp -j REJECT --reject-with tcp-reset

    # Anything that is not TCP has no way of being told, so it is dropped.
    iptables -A "$CHAIN" -j DROP

    # Send tunnel traffic through the chain, exactly once.
    iptables -C FORWARD -i tun+ -j "$CHAIN" 2>/dev/null || \
        iptables -I FORWARD 1 -i tun+ -j "$CHAIN"

    # The portal has to be reachable from the tunnel. Traffic addressed to
    # this machine's own tunnel address arrives on INPUT, not FORWARD, so the
    # rule above never sees it - stating this explicitly means the portal does
    # not depend on INPUT happening to default to ACCEPT.
    iptables -C INPUT -i tun+ -p tcp --dport "$PORTAL_PORT" -j ACCEPT 2>/dev/null || \
        iptables -I INPUT 1 -i tun+ -p tcp --dport "$PORTAL_PORT" -j ACCEPT

    # The resolver, for the same reason.
    iptables -C INPUT -i tun+ -p udp --dport 53 -j ACCEPT 2>/dev/null || \
        iptables -I INPUT 1 -i tun+ -p udp --dport 53 -j ACCEPT
    iptables -C INPUT -i tun+ -p tcp --dport 53 -j ACCEPT 2>/dev/null || \
        iptables -I INPUT 1 -i tun+ -p tcp --dport 53 -j ACCEPT

    portal_interception

    # NAT, so permitted traffic can reach the wider network.
    if [[ -n "$WAN_IF" ]]; then
        iptables -t nat -C POSTROUTING -s "$VPN_SUBNET" -o "$WAN_IF" -j MASQUERADE 2>/dev/null || \
            iptables -t nat -A POSTROUTING -s "$VPN_SUBNET" -o "$WAN_IF" -j MASQUERADE
    fi

    sysctl -qw net.ipv4.ip_forward=1

    echo "  done. every client takes the default route, so all of their traffic"
    echo "  arrives here. Signed out, a web request is answered by the portal;"
    echo "  anything else is refused."
}

# allow grants one client the destinations it is entitled to.
#
# Every destination is given at once rather than one call each, so there is no
# moment in which a client holds half of what it was granted.
#
# A destination may be a single address or a range - 10.0.5.20 or 10.0.5.0/24.
# The caller must send the range as it is, because a bare address is read as
# one host: granting 10.0.5.0 where 10.0.5.0/24 was meant opens the name of the
# range and none of the machines in it, while the client is still told to send
# all of them down the tunnel. Every packet would be dropped, and it would look
# like a broken network rather than a missing permission.
allow() {
    require_root
    local ip="${1:-}"
    [[ -n "$ip" ]] || die "usage: $0 allow <client-ip> [destination]..."
    shift

    # No destinations is allowed, and means exactly what it says: this person
    # has signed in and is entitled to nothing. They are marked as signed in
    # so the portal stops answering their web requests with the sign-in page -
    # being sent back to a page you have already completed is worse than being
    # told plainly that you have no access.
    local granted=0 dest
    for dest in "$@"; do
        [[ -n "$dest" ]] || continue
        if ipset add "$ACCESS_SET" "$ip,$dest" -exist; then
            granted=$((granted + 1))
        else
            echo "warning: could not grant $ip -> $dest" >&2
        fi
    done

    # Marks them as signed in, which exempts their web traffic from being
    # rewritten to the sign-in page. It opens nothing on its own.
    ipset add "$SIGNEDIN_SET" "$ip" -exist

    echo "allowed $ip to reach $granted destination(s)"
}

# revoke removes everything a client holds.
#
# ipset cannot delete by half a key, so the client's entries are found and
# removed one at a time. Missing any would leave a disconnected client's
# grants in place for whoever is given that tunnel address next.
revoke() {
    require_root
    local ip="${1:-}"; [[ -n "$ip" ]] || die "usage: $0 revoke <ip>"

    local removed=0 member
    while read -r member; do
        [[ -n "$member" ]] || continue
        if ipset del "$ACCESS_SET" "$member" -exist 2>/dev/null; then
            removed=$((removed + 1))
        fi
    done < <(ipset list "$ACCESS_SET" 2>/dev/null |
             sed -n '/^Members:/,$p' | tail -n +2 |
             awk -F, -v c="$ip" '$1 == c {print $0}')

    ipset del "$SIGNEDIN_SET" "$ip" -exist 2>/dev/null

    # Drop existing flows, or an open connection would survive revocation.
    command -v conntrack >/dev/null && conntrack -D -s "$ip" >/dev/null 2>&1
    echo "revoked $ip ($removed destination(s))"
}

status() {
    echo "── portal interception (signed-out web requests) ──"
    iptables -t nat -L "$NAT_CHAIN" -v -n --line-numbers 2>/dev/null | sed 's/^/  /'
    echo "── who may reach what (client,destination) ──"
    ipset list "$ACCESS_SET" 2>/dev/null | sed -n '/Members/,$p' | tail -n +2 | sed 's/^/  /'
    echo "── signed in (exempt from portal interception; grants nothing) ──"
    ipset list "$SIGNEDIN_SET" 2>/dev/null | sed -n '/Members/,$p' | tail -n +2 | sed 's/^/  /'
    echo "── chain (packets/bytes) ──"
    iptables -L "$CHAIN" -v -n --line-numbers 2>/dev/null | sed 's/^/  /'
}

# mail_open lets one signed-out client reach mail for a while, so the code it
# is waiting for can be read. Called by the connect hook when a client lands in
# the walled garden.
mail_open() {
    local client="${1:-}" window="${2:-$MAIL_WINDOW}"
    [[ -n "$client" ]] || die "mail-open needs a client address"

    ipset create "$MAIL_SET" hash:net,net timeout "$MAIL_WINDOW" -exist

    local n=0
    while read -r cidr; do
        [[ "$cidr" == \#* ]] && continue
        [[ -z "$cidr" ]] && continue
        ipset add "$MAIL_SET" "$client,$cidr" timeout "$window" -exist && n=$((n + 1))
    done < <(mail_ranges)

    echo "mail opened for $client for ${window}s ($n ranges)"
}

case "${1:-}" in
    setup)  setup ;;
    allow)  shift; allow "$@" ;;
    revoke) revoke "${2:-}" ;;
    mail-open) mail_open "${2:-}" "${3:-}" ;;
    status) status ;;
    *) echo "usage: $0 {setup|allow <client-ip> <destination>...|revoke <ip>|mail-open <client-ip> [seconds]|status}"; exit 1 ;;
esac
