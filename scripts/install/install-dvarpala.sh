#!/bin/bash
#
# Dvarpala server installer.
#
# Turns a fresh Ubuntu 22.04 machine into a working Dvarpala VPN server:
# PostgreSQL, Redis, OpenVPN with the Dvarpala hooks, the firewall, and the
# Dvarpala application itself.
#
# The cloud installer copies the source here and runs this over SSH. It is
# also runnable by hand, which is how it gets tested.
#
#   sudo ./install-dvarpala.sh --source /opt/dvarpala/src --host vpn.example.com
#
# Every step is idempotent: running it twice is safe.

set -euo pipefail

# ── configuration ────────────────────────────────────────────────────────────

DVARPALA_USER="dvarpala"
DVARPALA_DIR="/opt/dvarpala"
CONFIG_DIR="$DVARPALA_DIR/config"
CERT_DIR="$DVARPALA_DIR/certs"
SCRIPT_DIR="$DVARPALA_DIR/scripts"

SOURCE_DIR="${DVARPALA_SOURCE:-$DVARPALA_DIR/src}"
SERVER_HOST="${DVARPALA_HOST:-}"
ADMIN_EMAIL="${DVARPALA_ADMIN_EMAIL:-}"
VPN_SUBNET="172.30.100.0/24"
PORTAL_IP="172.30.100.1"
GO_VERSION="1.23.4"

while [[ $# -gt 0 ]]; do
    case "$1" in
        --source) SOURCE_DIR="$2"; shift 2 ;;
        --host)   SERVER_HOST="$2"; shift 2 ;;
        --admin)  ADMIN_EMAIL="$2"; shift 2 ;;
        *) echo "unknown option: $1" >&2; exit 1 ;;
    esac
done

log()  { echo -e "\033[0;34m==>\033[0m $*"; }
ok()   { echo -e "  \033[0;32m✓\033[0m $*"; }
warn() { echo -e "  \033[1;33m!\033[0m $*"; }
die()  { echo -e "\033[0;31merror:\033[0m $*" >&2; exit 1; }

[[ $EUID -eq 0 ]] || die "must run as root"
[[ -d "$SOURCE_DIR" ]] || die "source directory not found: $SOURCE_DIR"

# is_private reports whether an address is one only reachable from inside a
# private network.
is_private() {
    case "$1" in
        10.*|127.*|169.254.*|192.168.*) return 0 ;;
        172.1[6-9].*|172.2[0-9].*|172.3[0-1].*) return 0 ;;
        *) return 1 ;;
    esac
}

# public_ip_from_metadata asks the cloud provider for this machine's public
# address, printing nothing if there is no metadata service to ask.
#
# This matters more than it looks. On a cloud instance the public address is
# NAT'd and never appears on any interface, so `hostname -I` returns the
# private one - and that address goes into every .ovpn this install issues.
# The result installs cleanly, starts every service, and hands out profiles
# nobody outside the VPC can connect to, with no error at any point.
public_ip_from_metadata() {
    local ip token

    # AWS, tokened (IMDSv2) then untokened (IMDSv1).
    token="$(curl -sf --max-time 2 -X PUT \
        -H 'X-aws-ec2-metadata-token-ttl-seconds: 60' \
        http://169.254.169.254/latest/api/token 2>/dev/null)" || true
    if [[ -n "${token:-}" ]]; then
        ip="$(curl -sf --max-time 2 -H "X-aws-ec2-metadata-token: $token" \
            http://169.254.169.254/latest/meta-data/public-ipv4 2>/dev/null)" || true
    else
        ip="$(curl -sf --max-time 2 \
            http://169.254.169.254/latest/meta-data/public-ipv4 2>/dev/null)" || true
    fi
    [[ -n "${ip:-}" ]] && { echo "$ip"; return; }

    # Google Cloud.
    ip="$(curl -sf --max-time 2 -H 'Metadata-Flavor: Google' \
        'http://169.254.169.254/computeMetadata/v1/instance/network-interfaces/0/access-configs/0/external-ip' \
        2>/dev/null)" || true
    [[ -n "${ip:-}" ]] && { echo "$ip"; return; }

    # Azure.
    ip="$(curl -sf --max-time 2 -H 'Metadata:true' \
        'http://169.254.169.254/metadata/instance/network/interface/0/ipv4/ipAddress/0/publicIpAddress?api-version=2021-02-01&format=text' \
        2>/dev/null)" || true
    [[ -n "${ip:-}" ]] && { echo "$ip"; return 0; }

    # Found nothing, which is the ordinary case off a cloud instance. Return
    # success anyway: under `set -e` a non-zero status here would abort the
    # install at the assignment, silently, on every machine that has no
    # metadata service to ask.
    return 0
}

if [[ -z "$SERVER_HOST" ]]; then
    SERVER_HOST="$(public_ip_from_metadata)"
    if [[ -n "$SERVER_HOST" ]]; then
        HOST_SOURCE="cloud metadata"
    else
        SERVER_HOST="$(hostname -I | awk '{print $1}')"
        HOST_SOURCE="this machine's own interface"
    fi
fi

log "Installing Dvarpala"
echo "    source: $SOURCE_DIR"
echo "    host:   $SERVER_HOST${HOST_SOURCE:+  (detected from $HOST_SOURCE)}"

# Every profile this install issues will tell clients to connect to
# SERVER_HOST. A private address is right for an internal deployment and wrong
# for anything a client reaches over the internet, and we cannot tell which
# this is - so say so plainly rather than discovering it when nobody can
# connect.
if [[ -n "${HOST_SOURCE:-}" ]] && is_private "$SERVER_HOST"; then
    warn "$SERVER_HOST is a private address."
    warn "Clients outside this network will not be able to reach it, and every"
    warn "VPN profile issued here will point at it."
    warn "If that is wrong, stop now and re-run with:  --host <PUBLIC-IP-OR-DOMAIN>"
    echo
fi

# ── 1. packages ──────────────────────────────────────────────────────────────

log "Installing system packages"
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y -qq \
    postgresql postgresql-contrib redis-server \
    openvpn easy-rsa iptables ipset conntrack \
    curl ca-certificates python3 >/dev/null
ok "postgresql, redis, openvpn, ipset installed"

# ── 2. Go toolchain ──────────────────────────────────────────────────────────

if ! /usr/local/go/bin/go version >/dev/null 2>&1; then
    log "Installing Go $GO_VERSION"
    curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" \
        -o /tmp/go.tar.gz
    rm -rf /usr/local/go
    tar -C /usr/local -xzf /tmp/go.tar.gz
    rm -f /tmp/go.tar.gz
fi
export PATH="/usr/local/go/bin:$PATH"
ok "$(go version)"

# ── 3. user and directories ──────────────────────────────────────────────────

log "Creating service user and directories"
id -u "$DVARPALA_USER" >/dev/null 2>&1 || \
    useradd --system --shell /usr/sbin/nologin --home "$DVARPALA_DIR" "$DVARPALA_USER"
mkdir -p "$DVARPALA_DIR"/{bin,config,certs,scripts,web} /var/log/dvarpala /var/log/openvpn
ok "$DVARPALA_USER, $DVARPALA_DIR"

# ── 4. build ─────────────────────────────────────────────────────────────────

log "Building Dvarpala"
cd "$SOURCE_DIR"
go build -o "$DVARPALA_DIR/bin/dvarpala-server" ./cmd/dvarpala-server
go build -o "$DVARPALA_DIR/bin/dvarpala-cli"    ./cmd/dvarpala-cli
ok "dvarpala-server, dvarpala-cli"

# The portal templates and static assets are loaded from disk at runtime.
cp -r "$SOURCE_DIR/web/." "$DVARPALA_DIR/web/"
ok "portal templates and assets"

# ── 5. PostgreSQL ────────────────────────────────────────────────────────────

log "Configuring PostgreSQL"
systemctl enable --now postgresql >/dev/null 2>&1

DB_PASSWORD_FILE="$CONFIG_DIR/.db_password"
if [[ -f "$DB_PASSWORD_FILE" ]]; then
    DB_PASSWORD="$(cat "$DB_PASSWORD_FILE")"
else
    DB_PASSWORD="$(openssl rand -base64 24 | tr -d '/+=' | head -c 24)"
    install -m 600 /dev/null "$DB_PASSWORD_FILE"
    printf '%s' "$DB_PASSWORD" > "$DB_PASSWORD_FILE"
fi

sudo -u postgres psql -tAc \
    "SELECT 1 FROM pg_roles WHERE rolname='dvarpala'" | grep -q 1 || \
    sudo -u postgres psql -qc "CREATE USER dvarpala WITH PASSWORD '$DB_PASSWORD';"
sudo -u postgres psql -qc \
    "ALTER USER dvarpala WITH PASSWORD '$DB_PASSWORD';"
sudo -u postgres psql -tAc \
    "SELECT 1 FROM pg_database WHERE datname='dvarpala'" | grep -q 1 || \
    sudo -u postgres createdb -O dvarpala dvarpala
ok "database 'dvarpala', user 'dvarpala'"

# ── 6. Redis ─────────────────────────────────────────────────────────────────

log "Configuring Redis"
systemctl enable --now redis-server >/dev/null 2>&1
redis-cli ping >/dev/null 2>&1 || die "redis is not responding"
ok "redis responding"

# ── 7. configuration ─────────────────────────────────────────────────────────
#
# Written as YAML with literal values. The application reads this with Viper,
# which does not expand ${VAR} placeholders - a previous version of this
# installer wrote a dotenv file, which produced an entirely empty
# configuration and a server that could not start.

# An empty allow-list accepts any domain, which on an internet-facing server
# means anyone with an account at the identity provider. Default it to the
# administrator's own domain instead: wrong for a company that spans several,
# but wrong in the direction that refuses people rather than admitting them.
if [[ -n "$ADMIN_EMAIL" && "$ADMIN_EMAIL" == *@* ]]; then
    ALLOWED_DOMAINS_YAML=$'\n    - "'"${ADMIN_EMAIL#*@}"'"'
else
    ALLOWED_DOMAINS_YAML=" []"
fi

log "Writing configuration"
cat > "$CONFIG_DIR/environment.yaml" <<YAML
# Generated by install-dvarpala.sh on $(date -Iseconds)
server:
  port: 8080
  mode: release
  read_timeout: 30s
  write_timeout: 30s
  trusted_proxies: []

database:
  host: localhost
  port: 5432
  name: dvarpala
  user: dvarpala
  password: "$DB_PASSWORD"
  ssl_mode: disable
  max_open_conns: 25
  max_idle_conns: 5

redis:
  addr: localhost:6379
  password: ""
  db: 0
  pool_size: 10

auth:
  session_duration: 28800
  captive_portal_timeout: 1800
  jwt_secret: "$(openssl rand -hex 32)"

  # Who may sign in at all. An empty list accepts any domain, which on a
  # server reachable from the internet accepts anybody with a Google account.
  allowed_domains:$ALLOWED_DOMAINS_YAML

  # Signing in with a code sent by email.
  #
  # This is the method that works without HTTPS, because the code travels to
  # the person's mailbox rather than through the browser. Turn it on by
  # setting enabled to true and filling in the mail server below.
  otp:
    enabled: false

  # The mail server that delivers those codes.
  #
  # Sending directly from this machine does not work: a new server has no
  # sending reputation, so codes land in spam, and cloud providers block the
  # port direct delivery uses. Use the mail service the organisation already
  # has. For Google Workspace:
  #
  #     host: smtp.gmail.com
  #     port: 587
  #     username: noreply@your-domain
  #
  # The password must be an app password, not an account password. Keep it
  # out of this file and in the environment instead:
  #
  #     systemctl edit dvarpala
  #     [Service]
  #     Environment="AUTH_SMTP_PASSWORD=..."
  #
  # Then check delivery before anyone depends on it:
  #
  #     dvarpala-cli mail test you@your-domain
  smtp:
    host: ""
    port: 587
    username: ""
    password: ""        # leave empty; use AUTH_SMTP_PASSWORD
    from: ""
    from_name: "Dvarpala"

oauth:
  google:
    client_id: ""
    client_secret: ""
    redirect_url: http://$SERVER_HOST:8080/auth/google/callback

openvpn:
  management:
    host: localhost
    port: 7505
  networks:
    captive_portal: $VPN_SUBNET
    full_access: 172.30.8.0/21
  pki:
    ca_cert: $CERT_DIR/ca.crt
    ca_key: $CERT_DIR/ca.key
    ta_key: $CERT_DIR/ta.key
    auto_create: true
    client_cert_days: 365
  server:
    host: $SERVER_HOST
    port: 1194
    proto: udp

security:
  failed_login_threshold: 5
  ip_block_duration: 1800
  bcrypt_cost: 12

logging:
  level: info
  format: json
  output: stdout
YAML
chmod 600 "$CONFIG_DIR/environment.yaml"
ok "$CONFIG_DIR/environment.yaml"

# ── 8. certificate authority ─────────────────────────────────────────────────
#
# The application creates its CA on first start (pki.auto_create), and signs
# both the server certificate and every user certificate with it, so a client
# profile carries exactly one CA.

log "Preparing PKI"
[[ -f "$CERT_DIR/ta.key" ]] || openvpn --genkey secret "$CERT_DIR/ta.key" 2>/dev/null \
    || openvpn --genkey --secret "$CERT_DIR/ta.key"
chmod 600 "$CERT_DIR/ta.key"
ok "tls-crypt key"

# ── 9. hooks and firewall ────────────────────────────────────────────────────

log "Installing VPN hooks and firewall"
cp "$SOURCE_DIR/scripts/openvpn/client-connect.sh"    "$SCRIPT_DIR/"
cp "$SOURCE_DIR/scripts/openvpn/client-disconnect.sh" "$SCRIPT_DIR/"
cp "$SOURCE_DIR/scripts/openvpn/dvarpala-firewall.sh" "$SCRIPT_DIR/"
chmod +x "$SCRIPT_DIR"/*.sh
ok "client-connect, client-disconnect, firewall"

# ── 10. ownership ────────────────────────────────────────────────────────────

chown -R "$DVARPALA_USER:$DVARPALA_USER" "$DVARPALA_DIR" /var/log/dvarpala
chmod 700 "$CERT_DIR"

# ── 11. systemd ──────────────────────────────────────────────────────────────

log "Creating systemd service"
cat > /etc/systemd/system/dvarpala.service <<UNIT
[Unit]
Description=Dvarpala VPN management server
After=network.target postgresql.service redis-server.service
Requires=postgresql.service redis-server.service

[Service]
Type=simple
User=$DVARPALA_USER
WorkingDirectory=$DVARPALA_DIR
ExecStart=$DVARPALA_DIR/bin/dvarpala-server --config $CONFIG_DIR/environment.yaml
Restart=always
RestartSec=10

# The portal is the only thing an unauthenticated client can reach, so it
# must survive a restart loop rather than being locked down further.
NoNewPrivileges=true
PrivateTmp=true

[Install]
WantedBy=multi-user.target
UNIT

systemctl daemon-reload
systemctl enable dvarpala >/dev/null 2>&1
systemctl restart dvarpala
ok "dvarpala.service"

# Wait for it, so a failure surfaces here rather than at first use.
for i in $(seq 1 20); do
    sleep 1
    if curl -sf --max-time 2 "http://127.0.0.1:8080/api/internal/vpn/access/0.0.0.0" >/dev/null 2>&1; then
        ok "application responding on :8080"
        break
    fi
    [[ $i -eq 20 ]] && {
        journalctl -u dvarpala -n 20 --no-pager
        die "dvarpala did not start"
    }
done

# ── 12. server certificate ───────────────────────────────────────────────────
#
# Signed by the CA the application just created, so clients trust this server
# using the same CA certificate embedded in their profile.

log "Issuing the VPN server certificate"
sudo -u "$DVARPALA_USER" "$DVARPALA_DIR/bin/dvarpala-cli" \
    --config "$CONFIG_DIR/environment.yaml" \
    vpn server-cert --host "$SERVER_HOST" \
    --out-cert "$CERT_DIR/server.crt" --out-key "$CERT_DIR/server.key"
ok "server certificate"

# ── 13. OpenVPN ──────────────────────────────────────────────────────────────

log "Configuring OpenVPN"
mkdir -p /etc/openvpn/server
cat > /etc/openvpn/server/server.conf <<CONF
# Generated by install-dvarpala.sh
port 1194
proto udp
dev tun

ca $CERT_DIR/ca.crt
cert $CERT_DIR/server.crt
key $CERT_DIR/server.key
dh none
tls-crypt $CERT_DIR/ta.key

server ${VPN_SUBNET%/*} 255.255.255.0
topology subnet

# The connect hook pushes the default route to every client, signed in or
# not, so that all of their traffic arrives here to be judged. This route is
# the fallback for a client that declines the default route for reasons of
# its own: it can still reach the sign-in page.
push "route $PORTAL_IP 255.255.255.255"

script-security 3
client-connect $SCRIPT_DIR/client-connect.sh
client-disconnect $SCRIPT_DIR/client-disconnect.sh
setenv DVARPALA_API http://127.0.0.1:8080

keepalive 10 120
cipher AES-256-GCM
persist-key
persist-tun
status /var/log/openvpn/openvpn-status.log
log-append /var/log/openvpn/openvpn.log
verb 3

# The control channel. It is the only way to act on a tunnel that is already
# up: Dvarpala uses it to end somebody's session the moment they are
# deactivated, and to make a client reconnect after signing in so its routes
# arrive without the person doing anything.
#
# Anything that can reach this port controls the VPN completely, so it is
# bound to the loopback address and must never be exposed.
management 127.0.0.1 7505
CONF

# OpenVPN drops privileges but the hooks must read the certificates.
chmod 750 "$CERT_DIR"
chmod 644 "$CERT_DIR/ca.crt" 2>/dev/null || true

sysctl -qw net.ipv4.ip_forward=1
grep -q "^net.ipv4.ip_forward=1" /etc/sysctl.conf || echo "net.ipv4.ip_forward=1" >> /etc/sysctl.conf

systemctl enable openvpn-server@server >/dev/null 2>&1
systemctl restart openvpn-server@server
sleep 3
systemctl is-active --quiet openvpn-server@server || {
    journalctl -u openvpn-server@server -n 20 --no-pager
    die "openvpn did not start"
}
ok "openvpn-server@server"

# ── 14. firewall ─────────────────────────────────────────────────────────────

# Applied through a systemd unit rather than by calling the script once,
# because iptables rules and ipsets live only in kernel memory. Run directly,
# a reboot brings the server back with OpenVPN accepting clients and nothing
# enforcing the walled garden - and every other signal still reporting health.

log "Applying the walled-garden firewall"
cat > /etc/systemd/system/dvarpala-firewall.service <<UNIT
[Unit]
Description=Dvarpala walled-garden firewall
Documentation=file://$SCRIPT_DIR/dvarpala-firewall.sh

# The rules must exist before any client can connect, and the MASQUERADE rule
# needs the outbound interface to be up before the script goes looking for it.
After=network-online.target
Wants=network-online.target
Before=openvpn-server@server.service

[Service]
Type=oneshot
RemainAfterExit=yes
ExecStart=$SCRIPT_DIR/dvarpala-firewall.sh setup

[Install]
WantedBy=multi-user.target
UNIT

systemctl daemon-reload
systemctl enable dvarpala-firewall >/dev/null 2>&1
systemctl restart dvarpala-firewall

# Fail loudly here rather than leaving an unguarded server that looks healthy.
systemctl is-active --quiet dvarpala-firewall || {
    journalctl -u dvarpala-firewall -n 20 --no-pager
    die "the walled-garden firewall did not apply"
}
ok "unauthenticated clients are confined to the portal, and it survives reboot"

# ── 15. first administrator ──────────────────────────────────────────────────

if [[ -n "$ADMIN_EMAIL" ]]; then
    log "Creating the first administrator"

    cli() {
        sudo -u "$DVARPALA_USER" "$DVARPALA_DIR/bin/dvarpala-cli" \
            --config "$CONFIG_DIR/environment.yaml" "$@"
    }

    # Creating a user or a membership that already exists is not a failure -
    # this script is meant to be safe to re-run - so those two are allowed to
    # fail and the end state is checked instead. What must never happen is a
    # failure followed by a green tick: an administrator who was silently not
    # created only shows up much later, at a login page, as "user not found".
    create_out=$(cli user create --email "$ADMIN_EMAIL" --name "Administrator" 2>&1) || true
    assign_out=$(cli group assign --user "$ADMIN_EMAIL" --group system_admins 2>&1) || true

    # One row, carrying both answers: the address exists, and the groups it
    # belongs to. "group list" reports member counts rather than names, so it
    # cannot answer the second question.
    admin_row=$(cli user list 2>/dev/null | grep -F "$ADMIN_EMAIL" || true)

    if [[ -z "$admin_row" ]]; then
        echo "$create_out" >&2
        die "could not create the administrator $ADMIN_EMAIL"
    fi
    if [[ "$admin_row" != *system_admins* ]]; then
        echo "$assign_out" >&2
        die "$ADMIN_EMAIL was created but could not be added to system_admins"
    fi

    # This one has no idempotent excuse: if a profile cannot be issued, the
    # install has not produced anything anybody can connect with.
    if ! cli vpn issue --user "$ADMIN_EMAIL" --name admin \
            --output "$CERT_DIR/admin.ovpn" >/dev/null; then
        die "could not issue a VPN profile for $ADMIN_EMAIL"
    fi

    ok "$ADMIN_EMAIL created; profile at $CERT_DIR/admin.ovpn"
fi

# ── done ─────────────────────────────────────────────────────────────────────

echo
# A fresh install has no way for anybody to sign in: OAuth has no credentials,
# codes have no mail server, and the development login is refused outside debug
# mode. Saying "complete" here and leaving one warning in the journal is how an
# administrator ends up issuing VPN profiles for a door with no handle on the
# inside - the tunnel works, the walled garden holds, and nobody can ever leave
# it. So do not call it complete.
LOGIN_READY=no
# An OAuth provider with a client id, or codes by email switched on. Written
# as grep rather than sed so it behaves the same everywhere; otp.enabled is
# the only "enabled" key in the file this writes.
if grep -qE '^ *client_id: *"[^"]+"' "$CONFIG_DIR/environment.yaml" 2>/dev/null; then
    LOGIN_READY=yes
fi
if grep -qE '^ *enabled: *true' "$CONFIG_DIR/environment.yaml" 2>/dev/null; then
    LOGIN_READY=yes
fi

cat <<SUMMARY

  Portal        http://$SERVER_HOST:8080
  VPN           $SERVER_HOST:1194/udp
  Config        $CONFIG_DIR/environment.yaml
  Manage        sudo -u $DVARPALA_USER $DVARPALA_DIR/bin/dvarpala-cli \\
                  --config $CONFIG_DIR/environment.yaml user list

SUMMARY

if [[ "$LOGIN_READY" == "yes" ]]; then
    log "Installation complete"
    cat <<SUMMARY
  Next:
    1. Issue profiles:  dvarpala-cli vpn issue --user someone@example.com
    2. Register resources and grant group permissions.

SUMMARY
    exit 0
fi

warn "Installed, but NOBODY CAN SIGN IN YET."
cat <<SUMMARY
  Everything is running, and a person who connects will reach the login page
  and nothing else - which is all they will ever reach until a sign-in method
  is configured. Do not issue VPN profiles before this is done.

  Choose one, in $CONFIG_DIR/environment.yaml:

    Codes by email  - works over plain HTTP, so it needs nothing else.
                      Set auth.otp.enabled: true and fill in auth.smtp,
                      putting the password in AUTH_SMTP_PASSWORD rather than
                      the file. Then: systemctl restart dvarpala
                      Check it:  dvarpala-cli mail test you@your-domain

    Google          - needs a domain name and HTTPS. Google refuses plain
                      HTTP and refuses IP addresses, so it cannot work
                      against $SERVER_HOST as things stand.

  Then re-check with:  journalctl -u dvarpala | grep -i enabled

SUMMARY
exit 1
