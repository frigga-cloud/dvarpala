# Dvarpala — Reference for the Working Product

*Written 27 August 2026. Describes what is built and proven, not what is
planned. Where something does not work, it says so.*

---

## Contents

1. [What it is](#1-what-it-is)
2. [How access is decided](#2-how-access-is-decided)
3. [The journey, step by step](#3-the-journey-step-by-step)
4. [Architecture](#4-architecture)
5. [Data](#5-data)
6. [Signing in](#6-signing-in)
7. [The two enforcement layers](#7-the-two-enforcement-layers)
8. [Repository layout](#8-repository-layout)
9. [Installing](#9-installing)
10. [Running it day to day](#10-running-it-day-to-day)
11. [What is deliberately absent](#11-what-is-deliberately-absent)
12. [Known gaps](#12-known-gaps)

---

## 1. What it is

**Dvarpala** (Sanskrit द्वारपाल, "doorkeeper") is a Zero Trust network gateway
built on OpenVPN.

An ordinary VPN is a door: get through it and the company network is yours.
Lose a laptop and an intruder has everything on it.

Dvarpala makes the VPN a doorkeeper. Connecting gets you into an empty room
containing one sign-in page. Only after you prove who you are does it open the
specific addresses you are entitled to — and nothing else. Disconnect and it
closes behind you.

### Proven on real infrastructure

Every claim below has been demonstrated on a server built by the installer,
over the public internet, from a laptop and a phone:

- A connected but unidentified client reaches the sign-in page and nothing else
- Any `http://` request is answered by that page, whatever was asked for
- A code arrives by email and exchanges for a session
- After signing in, a granted private address is reachable
- An *ungranted* address on the same network is refused — **by the firewall**,
  not merely by withholding a route
- Every decision is recorded

### What it is not

Not a general-purpose VPN. Not a way to browse from another country. Not a
replacement for an identity provider. It decides who may reach which machines
on a private network, and enforces that.

---

## 2. How access is decided

Three separate things must be true. Passing one grants nothing.

**A certificate.** Every person has their own, issued by Dvarpala's certificate
authority, with their email address inside it. Without one, no tunnel forms at
all. This proves *a device* is permitted.

**An identity.** A six-digit code sent to that person's mailbox, exchanged for
a session. This proves *a person* is present. It is checked on every sign-in
and expires.

**An entitlement.** The account must exist in Dvarpala's own database, be
active, and belong to a group that has been granted the resource. This is the
step an identity provider cannot answer, and it is why proving your email is
not enough.

The certificate and the identity must agree: a session is pinned to the
certificate that created it, so an address handed on to a different client
cannot inherit it.

---

## 3. The journey, step by step

### Step 1 — Connect

The person imports a `.ovpn` file into any OpenVPN client and connects.
OpenVPN checks the certificate and gives them a tunnel address, for example
`172.30.100.2`.

### Step 2 — The server asks itself what this client may reach

OpenVPN runs `client-connect.sh`, which calls the internal API:

```
GET /api/internal/vpn/access/172.30.100.2?cn=sam@company.com
```

Nobody has signed in from that address, so the answer is: **nothing**.

### Step 3 — The walled garden

The client is given the **default route** — everything it sends goes to the
server.

This is deliberate and it is the whole design. Withholding a route is not a
denial: a laptop not told to send something through the tunnel sends it over
its own wifi, where the server never sees it and cannot refuse it. Taking the
default route means every packet arrives to be judged.

The firewall then permits three things and refuses the rest:

| | |
|---|---|
| The sign-in page | so there is somewhere to go |
| DNS, to this server only | so anything can be resolved at all |
| Web requests | **rewritten** to the sign-in page, whatever was asked for |

Everything else is refused — TCP with a reset, so a browser fails in a second
rather than waiting a minute and reporting that the internet is broken.

### Step 4 — Signing in

The person opens `http://signin` — a name this server's own resolver answers —
or any `http://` address at all, which is rewritten to the same page.

They enter their work email. A six-digit code is sent to it. They enter the
code.

Dvarpala then checks, in order: the email's domain is permitted; the account
exists; the account is active. Only then is a session created.

### Step 5 — The tunnel restarts, and access opens

Three seconds after signing in — long enough for the browser to receive its
cookie and the page saying it worked — the server tells OpenVPN to restart
that client.

This is not a workaround. **A connect is the only moment OpenVPN accepts new
routes for a client**; nothing can apply them to a live tunnel.

The hook runs again, and this time the answer lists the person's resources.
It pushes a route for each, and adds a `(client, destination)` pair to the
firewall for each.

The default route is **withdrawn**. Personal traffic goes back over the
person's own connection; only company addresses travel through the tunnel.

### Step 6 — Disconnect

Routes vanish with the tunnel. Firewall entries are removed, and connections
already open are killed. The session survives two minutes — because a
reconnect is how access is applied, and deleting on disconnect would make
access unreachable — then expires.

A client that connects and never signs in is disconnected after a configurable
window, thirty minutes by default.

---

## 4. Architecture

```
        the person                    the server                the network
        ──────────                    ──────────                ───────────

     OpenVPN client  ──tunnel──▶  OpenVPN
                                     │
                                     │ client-connect.sh
                                     ▼
                                  Dvarpala  ──▶ PostgreSQL   who exists,
                                     │           what may be reached,
                                     │           what happened
                                     │
                                     ├──▶ Redis        live sessions
                                     │
                                     ├──▶ iptables + ipset    enforcement
                                     │
                                     └──▶ Brevo or SMTP       sign-in codes
```

### The pieces

**OpenVPN** carries the tunnel and nothing else. It does not decide access; it
asks.

**`client-connect.sh`** runs on every connection. It asks Dvarpala what this
client may reach, writes the routes OpenVPN should push, and tells the firewall
which destinations to open. If Dvarpala cannot be reached it **fails closed** —
the client gets the sign-in page and nothing more. Failing open would hand the
network to everyone whenever the management server was down.

**Dvarpala** is one Go binary: the sign-in portal, the admin console, the
internal API the hooks call, and the certificate authority.

**PostgreSQL** holds people, groups, resources, permissions and the audit
trail. **Redis** holds live sessions, keyed both by browser cookie and by
tunnel address, because the VPN knows only the address.

**iptables with ipset** is the enforcement. One rule, one lookup, whatever the
number of clients.

**dnsmasq** answers DNS for clients in the walled garden, and answers `signin`
with the portal.

---

## 5. Data

Eleven tables. The five that matter:

**users** — email, name, department, status (`active` / `inactive` /
`suspended`), last login. Email is the identity everywhere: it is the
certificate's common name, the session's owner, and the audit trail's subject.

**groups** — permissions attach here, never to a person. A person inherits
what their groups have. Groups have a parent field for hierarchy, *which is
not yet honoured* (see gaps).

**resources** — a name, a type (`dashboard`, `vm`, `database`, `service`), an
IP address and a port. **The address is what matters**: a resource without one
can be granted and produces no route and no firewall entry.

**group_permissions** — which group may reach which resource, at what level
(`read`, `write`, `admin`, `ssh`, `full`). The level is recorded and audited;
enforcement today is reachability, not operation-level control.

**audit_logs** — every decision. Sign-ins, refusals, access granted and denied,
codes requested, permissions changed, emergency access. Read-only: nothing in
the application can alter or delete a record.

A user with audit history **cannot be deleted** — the database refuses.
Deactivation is the offboarding path.

---

## 6. Signing in

### Codes by email — the method in use

A six-digit code, generated with a cryptographic random source, sent to the
person's mailbox. Held in Redis, expiring in five minutes, single use.

Deliberately strict, because with codes as the only way in these limits are
the whole defence against guessing a six-digit number:

| | |
|---|---|
| One code per minute per address | a mailbox cannot be used as a weapon |
| Ten a day | |
| Five wrong attempts | then the code is destroyed |
| Compared in constant time | a wrong guess reveals nothing by how long it took |

**The portal never says whether an address exists.** An unknown address, a
blocked domain and a deactivated account produce the identical page, and no
message is sent. Otherwise the portal — reachable by anyone holding a
certificate — becomes a way to enumerate an organisation's staff. Refusals go
to the audit trail instead.

The verification is recorded against the login attempt, not handed to the
browser: knowing somebody's email is not enough to complete a sign-in as them.

### Sending the mail

Two ways, chosen by configuration:

**Brevo**, when an API key is set. No mail ports, and no judgement about where
a connection came from.

**Any SMTP server**, when a host is set. Timeouts, both styles of encrypted
connection, and a connection retry.

A caution learned the hard way: **Google Workspace refuses SMTP sign-ins from
datacenter addresses and reports it as bad credentials.** The same password was
accepted from a laptop and refused from the server minutes apart. If mail works
from your machine and not from the server, that is what has happened.

### Emergency access

If mail breaks, nobody can sign in — including whoever would repair the mail
server. `dvarpala-cli admin break-glass <email> --reason "..."` prints a
single-use link that signs that person in.

It skips proving control of a mailbox and nothing else: the account must exist
and be active, and its groups still decide what opens. It is not a privilege
escalation — anyone who can run it already holds the database credentials —
but it leaves a record saying exactly what it was.

Fifteen minutes by default, two hours maximum, a stated reason required, and
the link is spent on first use because URLs end up in logs.

---

## 7. The two enforcement layers

**Routes** tell a client where to send packets. **The firewall** decides what
the server will forward.

Both are needed, and the reason is worth stating plainly. Routes are
instructions to somebody else's machine. Adding one by hand takes a single
command — and until this was corrected, doing so reached a server that had
never been granted, and nothing recorded it.

The firewall holds `(client, destination)` pairs:

```
ACCEPT   match-set dvarpala_access src,dst
```

Nothing accepts traffic merely because it comes from somebody signed in.
Authentication decides **whether** you have destinations; the pairs decide
**which**. That is the difference between a VPN with a login page and Zero
Trust.

A separate list records who has signed in, used for one thing only: exempting
their web traffic from being rewritten to the sign-in page. It grants nothing.

### What cannot be done

**HTTPS cannot be redirected to the sign-in page.** TLS exists to prevent
exactly that, and no captive portal anywhere solves it. Those connections are
refused with a reset so the browser fails immediately.

**No operating system announces a captive portal when a VPN connects** —
verified on macOS, iOS and Android. That check runs when a *network* is joined,
and a VPN is not a network join. The sign-in address has to be communicated:
`http://signin` while in the garden, `http://172.30.100.1:8080` at any time.

---

## 8. Repository layout

```
cmd/
  dvarpala-server/     the web server
  dvarpala-cli/        the administration tool, run on the server

internal/
  app/                 wires everything together — read this first
  auth/                sessions, codes, mail, break-glass, providers
  services/            the rulebook: users, groups, resources, permissions, audit
  web/                 portal pages and the admin console
  vpn/                 certificate authority, OpenVPN management interface
  config/              settings and environment variables
  database/            tables and migrations
  redis/               connection
  api/vpnapi/          the endpoints the OpenVPN hooks call

scripts/
  openvpn/
    client-connect.sh       asks Dvarpala, pushes routes, opens the firewall
    client-disconnect.sh    revokes, kills open connections
    dvarpala-firewall.sh    the walled garden
    dvarpala-dns.conf       the resolver
  install/
    install-dvarpala.sh     bare Ubuntu to working server
    dvarpala-backup.sh      nightly, of the things that cannot be recreated
    dvarpala-cli-wrapper.sh puts dvarpala-cli on the PATH
  installation/             SEPARATE Go module: builds the cloud machine too

web/
  templates/           portal, sign-in, success, error, admin console
  static/              css and js, served locally
```

**Two Go modules.** `go build ./...` at the root does not cover
`scripts/installation`; build it separately.

Nothing on any portal page is fetched from the internet. Inside a real walled
garden a stylesheet from elsewhere would never arrive, and the first thing a
new user saw would be a broken page.

---

## 9. Installing

### On a server you already have

Ubuntu 22.04, with a public address and UDP 1194 reachable.

```bash
git clone https://github.com/frigga-cloud/dvarpala /opt/dvarpala/src
sudo /opt/dvarpala/src/scripts/install/install-dvarpala.sh \
  --source /opt/dvarpala/src --admin you@your-domain
```

About twenty steps: PostgreSQL, Redis, Go, the binaries, the certificate
authority, OpenVPN with the hooks, the firewall as a systemd unit so it
survives a reboot, the resolver, nightly backups, and the first administrator.

It is idempotent — running it twice is safe.

**It ends with a warning, not a success**, because a fresh server has no way
for anybody to sign in. Saying "complete" there is how an administrator ends
up issuing VPN profiles for a door with no handle on the inside.

### Building the machine too

For AWS, GCP or Azure, `scripts/installation` creates the network, the
instance, a fixed public address, and then runs the installer over SSH. It
needs the repository, Go, and cloud credentials on the operator's own machine.

### Then, before anybody can sign in

```bash
sudo nano /opt/dvarpala/config/environment.yaml    # auth.otp.enabled: true
                                                   # auth.brevo or auth.smtp
sudo nano /etc/dvarpala/dvarpala.env               # the key or password
sudo systemctl restart dvarpala
dvarpala-cli mail test you@your-domain
```

Credentials go in `/etc/dvarpala/dvarpala.env`, which root writes and only
Dvarpala reads. Both the server and the CLI read it, so the delivery check
tests the same credentials the server uses.

**Do not issue profiles until a real message arrives.**

---

## 10. Running it day to day

Everything is `dvarpala-cli`, on the server. Two commands orient a newcomer:

```bash
dvarpala-cli quickstart    # what to do, in the order it has to be done
dvarpala-cli check         # is this installation working, and what is wrong
```

`quickstart` reads the installation's own state and marks the steps already
done, so it is as useful half way through as it is on the first morning.

```bash
# People
dvarpala-cli user list
dvarpala-cli user create --email sam@company.com --name "Sam Patel"
dvarpala-cli user access sam@company.com        # what they may reach, and via which group
dvarpala-cli user deactivate sam@company.com    # blocks future sign-ins, kills their tunnel
dvarpala-cli user activate sam@company.com

# Groups carry the permissions
dvarpala-cli group create --name engineering --description "Engineering"
dvarpala-cli group assign --user sam@company.com --group engineering

# Resources, and who may reach them
dvarpala-cli resource create --name wiki --type service --ip 10.0.5.20 --port 80
dvarpala-cli permission grant --group engineering --resource wiki --type read

# Profiles
dvarpala-cli vpn issue --user sam@company.com --output sam.ovpn
dvarpala-cli vpn revoke sam@company.com

# Who is connected, and removing somebody
dvarpala-cli session list
dvarpala-cli session end 172.30.100.4

# What happened
dvarpala-cli audit --limit 20
dvarpala-cli audit --user sam@company.com
dvarpala-cli audit --action authentication_failed --since 2h
dvarpala-cli audit actions

# Mail, and the way back in
dvarpala-cli mail test you@your-domain
dvarpala-cli admin break-glass you@your-domain --reason "why"
```

An admin console covers the same ground at `http://172.30.100.1:8080/admin`,
reachable only through the tunnel, and only by members of `system_admins`.
Every form carries a token tied to the session, so no other website can make an
administrator's browser grant access.

### Two distinctions that matter

**Deactivating is not disconnecting.** `user deactivate` blocks future
sign-ins and closes their tunnel; a browser session already open lasts until it
expires. `session end` is the immediate removal.

**Granting is not applying.** A new permission reaches somebody on their next
connect. Their tunnel has to cycle.

### Backups

Nightly to `/var/backups/dvarpala`, kept fourteen days: the database and the
certificate authority. It verifies its own dumps and discards a corrupt one
rather than leaving something that looks like a backup.

**Those copies live on the machine they protect.** Take a copy off it. Losing
the certificate authority means every profile ever issued stops working and
every person needs a new one.

---

## 11. What is deliberately absent

**Deleting people.** The audit trail's foreign key refuses it. You should not
be able to erase somebody's history; deactivation is the offboarding path.

**Passwords.** There are none to steal, reuse or reset.

**OAuth in the working configuration.** The abstraction exists and Google is
implemented, but Google refuses plain HTTP and refuses IP addresses, so it
cannot work without a domain name and a certificate. Codes by email need
neither.

**Intercepting HTTPS.** Possible with a self-signed certificate and a
full-page browser warning. Teaching people to click past certificate warnings
is the habit that gets them phished later.

**Trusting `X-Forwarded-For`** from anyone but configured proxies. With none
set, the client address is the actual peer and cannot be forged.

---

## 12. Known gaps

Honest list. None prevents the product working; all would be noticed.

**Group hierarchy does not inherit.** Groups have a parent field and the
service supports nesting, but permissions consider direct membership only. A
member of a child group gets none of the parent's grants.

**A resource with no address is silently unroutable.** It can be created and
granted, shows in every listing, and produces no route and no firewall entry.

**No certificate revocation list.** A revoked certificate still completes a
TLS handshake; the database check refuses access, so this is a second line of
defence rather than a hole.

**Permission levels are recorded, not enforced.** `read` and `write` are
audited and shown, but enforcement is reachability.

**No TLS on the portal.** Not urgent — it is reachable only through an
encrypted tunnel and nowhere else — but it blocks OAuth and browsers grow less
tolerant each year.

**Profiles name an IP address.** A fixed address survives a restart, but
rebuilding a server leaves every issued profile pointing somewhere dead, with
no error beyond a client waiting for a host. A domain name fixes this properly.

**The cloud installer has no teardown.** It creates networks, instances and
addresses and removes none of them.

**Sessions are keyed to the tunnel address.** Nothing pins a client to the same
address across reconnects, so a new address means a lost session. The
certificate check prevents another client inheriting one.

**`main` does not contain this work.** Anything cloning the default branch gets
code with no installer in it.
