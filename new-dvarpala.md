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
| Mail services | so the code that was just sent can be read |
| Web requests | **rewritten** to the sign-in page, whatever was asked for |

That third one is not a convenience. The code arrives by email, and if a
person's mail client cannot reach its server they cannot read it — the only
way in is blocked by the thing they are signing in to. The allowed list is in
`scripts/openvpn/dvarpala-firewall.sh` and covers Google, Microsoft and Apple
by default; add whatever your people actually use, and keep it short, because
every entry is a small hole in the garden.

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

There are two ways in, and they suit different people. Read the first
paragraph of each and pick one; do not follow both.

Every command below says which machine to type it on. That distinction
matters more than anything else in this section.

Anything in angle brackets is something you substitute:

| | |
|---|---|
| `<the-address>` | the server's public address, e.g. `65.1.124.239` |
| `<your-key.pem>` | the file that lets you on to the server — explained below |
| `you@your-domain` | your own work email address |

Type the value, not the brackets.

---

### Path A — you already have a server

**For:** anyone whose organisation makes its own machines. Most people.

**You need, before starting:**

- A machine running **Ubuntu 22.04**, reachable from the internet
- **UDP port 1194** open to it, and SSH
- To be able to `ssh` into it and run `sudo`

Dvarpala does not create the machine on this path, and does not open the
port. If you cannot already reach the machine with `ssh`, stop here — nothing
below will work.

**ON THE SERVER**, over SSH:

```bash
sudo apt-get update && sudo apt-get install -y git
sudo git clone --branch install-v3 \
  https://github.com/frigga-cloud/dvarpala /opt/dvarpala/src
sudo /opt/dvarpala/src/scripts/install/install-dvarpala.sh \
  --source /opt/dvarpala/src \
  --admin you@your-domain
```

**`--branch install-v3` is not optional today.** Without it git takes the
repository's default branch, `main`, which does not contain this work at all —
there is no `scripts/install/` there, so the next line fails on a file that
does not exist. When this work merges into `main`, drop the `--branch` line
and the command becomes the ordinary one. Nothing else in this section
changes.

`--admin` is the first administrator's email. It is created for you, and it is
the account you will sign in as.

About twenty minutes. It installs PostgreSQL, Redis, Go, the Dvarpala
binaries, a certificate authority, OpenVPN with the hooks, the walled-garden
firewall as a systemd unit so it survives a reboot, the resolver, and nightly
backups.

Running it twice is safe.

**It ends with a warning, not a success.** That is correct: a fresh server has
no way for anybody to sign in. Continue to "Making it possible to sign in"
below.

---

### Path B — build the server as well

**For:** a proof of concept, or an organisation happy to hand over cloud
credentials.

**You need, before starting, ON YOUR OWN COMPUTER:**

- This repository, cloned
- **Go** installed
- The **AWS CLI**, configured with `aws configure`, using credentials that may
  create networks and instances

**ON YOUR OWN COMPUTER:**

```bash
cd dvarpala/scripts/installation
go run launcher.go --provider aws --region ap-south-1
```

It asks a few questions. Two answers matter:

- **Region** — type it. Pressing enter takes the default, which may not be
  the one you passed on the command line.
- **Instance type** — the smallest offered is enough.

Leave any key and secret prompts **blank**; anything typed there overrides the
credentials `aws configure` already set up.

It creates a network, a security group, an instance and a fixed public
address, then runs the same installer as Path A over SSH.

**What it leaves you**, in `./dvarpala-deployment`:

| | |
|---|---|
| `<name>-keypair.pem` | the SSH key for the server. **The only copy.** |
| `admin.ovpn` | the first administrator's VPN profile |
| `connection-info.txt` | the server's address, and the steps from here |

Both files hold private keys. Do not commit them or send them by email.

---

### Getting on to the server

Everything after this point is typed on the server, so both paths meet here.

#### What a key file is

Servers do not ask for a password. They ask for a **key file** — a small file
on your own computer that proves you are allowed on. It usually ends `.pem`.

It comes in a pair. The server keeps one half; you keep the other. Anybody
holding your half can get on to that server as you, so it is never sent by
email, never committed, and never shared.

**Where yours is:**

- **Path B** — the installer created it and saved it in
  `./dvarpala-deployment/`, next to where you ran the command. It is the file
  ending `-keypair.pem`. **This is the only copy in existence**; if you lose
  it you cannot get back on to that server.
- **Path A** — whoever made the machine created it. It is the same file you
  already use to reach that server. If somebody else made it, ask them for it.

To find it:

```bash
ls ~/dvarpala/scripts/installation/dvarpala-deployment/*.pem
```

Wherever this document writes `<your-key.pem>`, put that file's path.

#### Connecting

**ON YOUR OWN COMPUTER:**

```bash
ssh -i <your-key.pem> ubuntu@<the-address>
```

Three parts:

| | |
|---|---|
| `-i <your-key.pem>` | the key file above |
| `ubuntu` | the user account. Always `ubuntu` on an Ubuntu image |
| `<the-address>` | the server's public address |

The address is printed at the end of the install and again in
`connection-info.txt`.

A worked example — yours will differ:

```bash
ssh -i ./dvarpala-deployment/friggalabs-vm-y66am-keypair.pem ubuntu@65.1.124.239
```

**If it says "Permission denied (publickey)"** the key is wrong, or you are
running the command on the server instead of on your own computer.

**If it says the identity file is not accessible** the path is wrong — run the
`ls` above and copy the path it prints exactly.

Your prompt changes to something like `ubuntu@ip-172-30-0-16` once you are on
the server. If it still shows your own computer's name, the commands below
will not do what you expect.

Worth doing once, so the key path stops mattering. **ON YOUR OWN COMPUTER**,
add this to `~/.ssh/config`, creating the file if it does not exist:

```
Host dvarpala
    HostName <the-address>
    User ubuntu
    IdentityFile /full/path/to/<name>-keypair.pem
```

Then it is just `ssh dvarpala`. Nothing depends on this; it only saves typing.

---

### Making it possible to sign in

Required on both paths. Until this is done, anybody who connects reaches the
sign-in page and nothing else.

**ON THE SERVER:**

**1. Turn on codes, and say who sends them.**

```bash
sudo nano /opt/dvarpala/config/environment.yaml
```

Under `auth:`, set:

```yaml
  otp:
    enabled: true
```

Then choose **one** way of sending the codes, and fill in that block only.

Dvarpala does not send email itself. It hands each message to a service that
does, and this is where you say which.

#### Either — a transactional email service

Brevo, SendGrid, Postmark, Amazon SES and others exist to send email from
servers. Dvarpala speaks to Brevo directly.

**What you need first:** an account at `brevo.com`, and the address you want
codes to come from verified with them. Then, in their dashboard under
**SMTP & API**, create an **API key** — a long string beginning `xkeysib-`.
Note this is *not* the SMTP key on the same page.

```yaml
  brevo:
    api_key: ""                    # leave empty - it goes in step 2
    from: noreply@your-domain      # must be verified with Brevo
    from_name: "Your Company"      # what recipients see as the sender
```

Best for most deployments: it needs no mail ports, and it does not care that
the connection comes from a server.

#### Or — a mail server you already have

Any ordinary mail server: your own, or a provider's.

**What you need first:** its address, the login it expects, and a password or
application key for that login. Your email provider's help pages call this
"SMTP settings".

```yaml
  smtp:
    host: smtp.your-provider       # e.g. smtp.gmail.com
    port: 587                      # 587 usually; 465 for some providers
    username: <the login>          # often not the same as the from address
    password: ""                   # leave empty - it goes in step 2
    from: noreply@your-domain      # what recipients see
    from_name: "Your Company"
```

Two cautions. Some providers use a login that cannot be sent from — Brevo's
SMTP login ends `@smtp-brevo.com` — which is why `username` and `from` are
separate settings.

And a mail service built for people may refuse a sign-in from a server and
report it as a wrong password. Google Workspace does this. If the same
credentials work from your laptop and not from the server, that is what has
happened; use a transactional service instead.

Save with `Ctrl+O`, `Enter`, `Ctrl+X`.

**2. Put the credential in the credentials file**, not the settings file. That
file is read by several programs and ends up in backups.

```bash
sudo nano /etc/dvarpala/dvarpala.env
```

Remove the `#` from the line you need and put the value after the `=`:

```
BREVO_API_KEY=xkeysib-...
```

or

```
AUTH_SMTP_PASSWORD=...
```

**3. Apply it, and check.**

```bash
sudo systemctl restart dvarpala
sudo journalctl -u dvarpala -n 20 | grep -i "code sign-in"
```

You want a line naming what you configured. If it says codes are being written
to the server's own log, the credential did not reach it — check step 2.

**4. Send yourself a real message.**

```bash
dvarpala-cli mail test you@your-domain
```

**Do not go further until that email arrives.** A VPN profile issued now gets
somebody as far as the sign-in page and no further.

---

### What next

**ON THE SERVER:**

```bash
dvarpala-cli quickstart
```

Prints the remaining steps in order — resources, groups, permissions,
profiles — and marks the ones already done. Section 10 covers the same ground
in more detail.

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

# Profiles - see "Giving somebody access" below
dvarpala-cli vpn issue --user sam@company.com
dvarpala-cli vpn list
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

### The admin console

The same ground, for people who would rather click, at:

```
http://172.30.100.1:8080/admin
```

**That address, not `http://signin`.** The name is answered by this server's
own resolver, which a client is given only while it is still in the walled
garden. Once somebody signs in they keep their own resolver — their personal
traffic is theirs — so the name stops resolving at exactly the point an
administrator wants it. The address works in both states.

Reachable only through the tunnel, and only by members of `system_admins`.
Every form carries a token tied to the session, so no other website can make an
administrator's browser grant access.

### Giving somebody access

Four things, in this order. Skipping one produces something that looks right
and does nothing.

**1. Make sure they exist and are in a group that has been granted something.**

```bash
dvarpala-cli user access sam@company.com
```

If that lists no grants, a profile will get them to the sign-in page and no
further.

**2. Issue their profile.**

Two ways. The console is easier and is what you will use most of the time.

*From the admin console*, while connected to the VPN yourself:

```
http://172.30.100.1:8080/admin
```

Enter their email in the VPN section and press issue. **The file downloads to
your own computer** the way any download does — no key file, no ssh, nothing
to clean up afterwards.

*Or from your own computer*, in one command:

```bash
ssh -i <your-key.pem> ubuntu@<the-address> \
  'dvarpala-cli vpn issue --user sam@company.com --output -' > sam.ovpn
```

`--output -` writes the profile to the screen instead of to a file, so it
arrives on your machine directly. Useful for scripting, or before you have a
working profile of your own to reach the console with.

**The first profile is the exception.** Nobody can reach the console before
they have one, so the very first administrator's is fetched for you: Path B
writes it to `dvarpala-deployment/admin.ovpn`, and on Path A the command above
is the way to get it. After that, use the console.

Each person should have exactly one profile. Issuing a new one revokes the
last, so a lost laptop is handled by issuing again.

**3. Hand the file over directly.**

It contains that person's private key. Anybody holding it can open a tunnel as
them. Give it to them in person, or over something you already trust for
credentials — not email, and not a shared drive. Delete your copy afterwards.

**4. Tell them three things.**

- Install an OpenVPN client — Tunnelblick on a Mac, OpenVPN Connect elsewhere
- Import the file and connect
- Then open **http://signin**, or the address you were given if that fails

That last point matters. No operating system announces a captive portal when a
VPN connects, so nothing will prompt them. Any `http://` address is redirected
to the sign-in page, but they have to open something.

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
code with no installer in it, which is why Path A above passes
`--branch install-v3`. The cloud installer in Path B already names the same
branch internally. Both revert to the plain default the day this merges.
