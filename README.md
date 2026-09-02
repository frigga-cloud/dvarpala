# Dvarpala

**Dvarpala** (Sanskrit द्वारपाल, "doorkeeper") is a Zero Trust network gateway
built on OpenVPN.

An ordinary VPN is a door: get through it and the company network is yours.
Lose a laptop and an intruder has everything on it.

Dvarpala makes the VPN a doorkeeper. Connecting gets you into an empty room
containing one sign-in page. Only after you prove who you are does it open the
specific addresses you are entitled to — and nothing else. Disconnect and it
closes behind you.

---

## How it works

```
connect            a tunnel forms; nothing is reachable but a sign-in page
sign in            a six-digit code, sent to your work email
                   the tunnel restarts, and your permissions are applied
after that         only the addresses you were granted come through the VPN
                   everything else goes over your own connection, as before
disconnect         access closes behind you
```

Three separate things must be true before anything opens. A **certificate**,
so the device is permitted. An **identity**, proved by a code sent to your
mailbox. And an **entitlement** — the account must exist here, be active, and
belong to a group that has been granted the resource. Passing one grants
nothing.

Two layers enforce it. **Routes** tell a laptop where to send packets;
**a firewall** decides what the server will actually forward, as
`(who, where)` pairs. Routes alone are only instructions to somebody else's
machine — adding one by hand takes a single command — so the firewall is what
makes the answer real.

---

## Documentation

**[new-dvarpala.md](new-dvarpala.md)** — the reference. What is built and
proven, how to install it, how to run it, and what does not work yet. Start
there.

Everything else at the top level of this repository predates the current
system and contradicts it in places. Treat it as history.

---

## Installing

Two ways, described in full in
**[new-dvarpala.md §9](new-dvarpala.md#9-installing)**.

**You already have a server** — Ubuntu 22.04, reachable, with UDP 1194 open:

```bash
# ON THE SERVER
sudo apt-get update && sudo apt-get install -y git
sudo git clone https://github.com/frigga-cloud/dvarpala /opt/dvarpala/src
sudo /opt/dvarpala/src/scripts/install/install-dvarpala.sh \
  --source /opt/dvarpala/src --admin you@your-domain
```

**Or build the server too**, on AWS, GCP or Azure — needs this repository, Go,
and cloud credentials on your own machine:

```bash
# ON YOUR OWN COMPUTER
cd scripts/installation
go run launcher.go --provider aws --region <region>
```

Either way the install **ends with a warning, not a success**: a fresh server
has no way for anybody to sign in until a mail service is configured. The
reference explains that step, and `dvarpala-cli quickstart` on the server
prints what remains in the order it has to be done.

---

## Running it

Everything is `dvarpala-cli`, on the server:

```bash
dvarpala-cli quickstart              # what to do, in order, and what is left
dvarpala-cli check                   # is this installation working
dvarpala-cli user access sam@co.com  # what somebody may reach, and via which group
dvarpala-cli session list            # who is connected right now
dvarpala-cli audit --limit 20        # what has happened
```

There is also an admin console at `http://172.30.100.1:8080/admin`, reachable
only through the tunnel and only by members of `system_admins`.

---

## Development

Go 1.23, PostgreSQL 15, Redis.

```bash
make deps        # fetch dependencies
make build       # build both binaries
make test        # run the tests
make fmt         # format
make help        # the full list
```

Two Go modules: the root, and `scripts/installation` for the cloud installers.
`go build ./...` at the root does **not** cover the second one.

```
cmd/          dvarpala-server (the web server), dvarpala-cli (administration)
internal/     app, auth, services, web, vpn, config, database, redis
scripts/      openvpn (the hooks and firewall), install, installation
web/          templates and static files, served locally
```

`internal/app/app.go` shows how the pieces connect. Then
`scripts/openvpn/client-connect.sh`, which is where access is actually decided.

---

## Where it stands

The whole journey has been demonstrated on a server the installer built by
itself: connect, walled garden, a code by email, sign in, reach a granted
private address, and be refused an ungranted one on the same network — refused
by the firewall, not merely left unrouted.

What is not finished is listed honestly in
**[new-dvarpala.md §12](new-dvarpala.md#12-known-gaps)**. The short version:
group hierarchy does not inherit, permission levels are recorded rather than
enforced, there is no certificate revocation list, and profiles name an IP
address rather than a domain.

---

## Licence

See [LICENSE](LICENSE).
