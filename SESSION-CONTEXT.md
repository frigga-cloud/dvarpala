# Dvarpala — session handoff

Written 2026-08-17. Updated 2026-08-18 after the first real cloud deployment,
again after testing from a phone, on 2026-08-24 after sign-in by emailed code
was built, and on 2026-08-26 - the day the whole product ran end to end on a
server the installer built by itself.
Read this first, then `DVARPALA-OVERVIEW.md` for the deep technical audit.

---

## 1. Who you are working with

The user is **not a programmer** and has said so repeatedly. They inherited this
repository with no prior knowledge of it and are continuing its development.

**How they have asked to be worked with:**

- **Plain language.** Explain concepts, don't assume. They have said "I genuinely
  feel lost" more than once. When they ask "how does X work", they want the
  concept, not the code.
- **Commit messages: one line only.** They said this explicitly, twice.
- **Don't break what works.** Direct quote: *"be 100 percent sure it doesn't ruin
  what we've built till now."* Verify after every change — build, vet, test, and
  confirm the running app still answers.
- **No pull requests.** They declined; don't open one.
- **Don't push without being asked.** Standing preference in this session.
- **Documents:** no repetition, no "phase" framing (a third party won't know what
  a phase is), don't over-use superlatives, don't re-litigate things already
  resolved.
- When they ask "what did we achieve", they want an honest percentage and a plain
  summary — not a feature list.
- **Explain after each assertion.** Asked for directly: make a claim, then say
  what it means. Do not stack claims and explain at the end.
- **They will interrupt and redirect.** Follow the redirect immediately rather
  than finishing the previous thread. If they say something does not work, find
  out what they saw before theorising.

---

## 2. What Dvarpala is

A **Zero Trust network gateway**. Sanskrit for "doorkeeper".

An employee connects to a VPN. Instead of landing on the whole company network —
which is what an ordinary VPN gives you — they land in a **walled garden** where
only a login page is reachable. They sign in with Google. Dvarpala then checks its
own database: is this person still active, and which groups are they in? Based on
that it opens **only** the specific addresses that person is permitted to reach.
Disconnect, and it closes behind them.

Two enforcement layers, deliberately:

- **Routes** — the laptop is only told how to reach permitted addresses
- **A firewall (ipset)** — everything else is dropped

Routes alone are only a suggestion; the firewall makes it real.

**Commercial context:** the user says this is *"actually the zero trust network
access we will sell eventually"*. The marketing site promises who/what/where
visibility; the user considers that aspirational.

---

## 3. State of the repository

```
branch:            dev/foundation
commits ahead:     88
pushed:            partly — see below
working tree:      configs/environment.yaml and SESSION-CONTEXT.md, both
                   deliberately uncommitted
main branch:       still the old pre-review code
```

**⚠️ Another session works in this repository at the same time.** This is
confirmed, not suspected: on 24 Aug two commits landed mid-conversation that
this session did not make — `0e087b2` (installer asks the cloud for its public
address) and `5cf1d41` (OpenVPN management interface). Both are authored
`richa` and co-authored by another Claude.

Consequences to work with rather than fight:

- `git status` and `git log` before assuming you know the state. A file you
  did not touch may be modified; that is usually deliberate.
- Read a file immediately before writing it.
- Re-run the full verification after committing, not only before. Someone
  else's commit may have landed in between. On 24 Aug that check passed.
- Do not revert a change you cannot account for. Ask.

**The admin console with writes is now committed** (`b564b6e`) — it adds people,
creates groups and resources, grants and revokes access, and issues a `.ovpn` as
a browser download. Every form carries a CSRF token derived from the session; a
POST without it is refused (403, verified).

**⚠️ Pushing needs a pull request, and creating a branch does not.**

A ruleset named "PR Merge Only" has applied to `~ALL` branches since June 2025.
Pushing to a branch that already exists is refused; creating a new one is not.
That is why the first push of `dev/foundation` worked and every later one has
failed.

The working method has been a new branch per batch - `test/brevo` is the most
recent, and the cloud installer's `defaultRepoRef` is pointed at whichever is
current so a test box installs today's code. **That pin is a local, uncommitted
change; do not commit it.** It must return to `main` when the branch merges.

Ask an administrator to exclude `dev/**` from that ruleset, or accept pull
requests. The user has said no pull requests; those two positions have not been
reconciled.

**⚠️ `configs/environment.yaml` holds a live Google client secret** and is left
uncommitted for that reason — the repository is public. It needs rotating, and
the replacement should come from `OAUTH_GOOGLE_CLIENT_SECRET` (already bound in
`config.go`) rather than the file.

**⚠️ Second issue:** the cloud providers clone `main`, which has none of this work.
A cloud install today would fetch the old broken code. This resolves itself when
the branch merges.

### The 49 commits, grouped

| Stage | What it did | Commits |
|---|---|---|
| **Make it run** | Removed duplicate `main()` stubs; fixed a 404 installer URL; replaced `${VAR}` config placeholders Viper never expanded | `f72bb54` `f794b08` `266e3b8` |
| **The rulebook** | Service layer: users, groups, resources, permissions; CLI commands; the "what may this email reach" query | `c988c5e` `fde1b3e` `93c90a4` `92fe2fe` |
| **Login** | Redis session store, OAuth provider abstraction, dev provider, first tests, captive portal served, Google login | `565da0d` `00a0a0c` `c0cf2bf` |
| **The gate** | VPN access API, connect/disconnect hooks, ipset firewall, per-user certificates, disconnect grace period | `5c3cb53` `cf237de` `7363c00` `8c1ed27` |
| **Install anywhere** | One script, bare Ubuntu → working system; providers now call it; dead code removed | `15aaed1` `53c9bbd` `11ea003` |
| **Security** | Don't trust `X-Forwarded-For`; stop publishing `admin.ovpn` on a public web root | `c5cc210` `26542e9` |
| **Docs** | The system review | `ccd5ebc` |
| **Real deployment** (18 Aug) | Audit VPN access decisions; read-only admin console; firewall survives reboot; sign-in by emailed code | `a4e0586` `8208eed` `882da46` `1be43cf` |
| **Cleanup** (24 Aug) | Removed generated scaffolding and superseded subsystems — 363 files/11.3 MB down to 121/2.2 MB; gofmt across the tree; admin console writes | `aac3da5` `bb79bba` `b564b6e` |
| **Closing the 18 Aug findings** (24 Aug) | Internal VPN API restricted to loopback; login-machinery failures audited; installer fails loudly when the first admin cannot be created | `f091b6f` `12d2932` `15b3df2` |
| **Code sign-in made real** (24 Aug) | Portal offers it and the dead buttons are gone; pages no longer fetch anything from the internet; mailer hardened for a real server and given a connection retry; code requests audited; `mail test`; config documented | `3550131` `938681d` `ad980fc` `513f5cb` `76c0a9f` `3f882e4` `34f3a9a` |
| **Operating it** (24 Aug) | Break-glass emergency access; a readable audit trail; a session list and kill switch | `c285dcd` `08a0ebc` `4d4f004` |
| **From the other session** (24 Aug) | Installer asks the cloud for its public address; OpenVPN management interface for instant revocation and reconnect-after-login | `0e087b2` `5cf1d41` |

---

## 4. Map of the code

```
internal/services/     the rulebook — user, group, resource, permission,
                       vpnconfig, audit. services.New() is the composition root.
                       PermissionService.ResourcesForUser() is the core query.

internal/auth/         session.go   — Redis sessions, keyed BOTH by token and by
                                      client IP (the VPN looks up by IP)
                       provider.go  — the OAuth interface
                       google.go    — real Google OAuth
                       dev.go       — fake login, refuses unless mode=debug
                       otp.go       — sign-in by emailed code: rate limits,
                                      single-use verification bound to the state
                       mailer.go    — SMTP: timeouts, both encrypted-connection
                                      styles, a connection retry, and a log
                                      fallback (debug mode only)
                       breakglass.go— emergency access when mail is broken:
                                      single-use link, self-expiring, audited
                       session.go   — also Active(), which lists live sessions
                       service.go   — Begin/Complete/Logout, domain allow-list,
                                      RequestCode/VerifyCode, NewLoginAttempt,
                                      RedeemBreakGlass

internal/vpn/pki.go    certificate authority in pure Go. IssueClient() puts the
                       user's email in the common name — that is how the VPN
                       knows who connected.

internal/api/vpnapi/   GET  /api/internal/vpn/access/:clientip   ← the hook calls this
                       DELETE /api/internal/vpn/session/:clientip

internal/web/auth.go   captive portal handlers; portalView() builds the sign-in
                       page from the providers actually enabled. Also
                       /break-glass, which redeems an emergency link.
internal/web/otp.go    the code form. /otp/start is the portal's one-step entry
                       (address in, attempt created, code sent); /otp/login is
                       the generic provider entry point.
internal/web/admin.go  admin console at /admin, gated on system_admins

web/templates/         captive-portal, otp-login, auth-success, auth-error, admin
web/static/css/        portal.css — the portal styles itself. Nothing on these
                       pages is fetched from the internet, because inside a real
                       walled garden nothing would arrive.

cmd/dvarpala-cli/      user, group, resource, permission, vpn,
                       mail test        — proves delivery, names the fix on failure
                       audit            — read the trail; --user/--action/--since
                       audit actions    — what kinds of event exist
                       session list     — who is signed in, and whether the
                                          network is open to them
                       session end <ip> — the kill switch for a live session
                       admin break-glass— emergency access

scripts/openvpn/       client-connect.sh    — asks the API, writes routes, FAILS CLOSED
                       client-disconnect.sh — revokes firewall, ends session
                       dvarpala-firewall.sh — setup|allow|revoke|status

scripts/install/install-dvarpala.sh    15 steps, bare Ubuntu → working Dvarpala

scripts/installation/  SEPARATE GO MODULE. Cloud provisioning for AWS/GCP/Azure.
                       providers/dvarpala.go is shared logic (compiled, not yet
                       called — providers currently use a 2-step git-clone path).
```

**Two Go modules.** Root is `dvarpala`; `scripts/installation` is
`dvarpala-cloud-installer`. `go build ./...` at the root does not cover the
installer — build it separately.

---

## 5. Local development environment

Everything below is already installed and running on this machine.

```bash
export PATH=$PATH:/usr/local/go/bin     # Go 1.26.5 — NOT on the default PATH

pg_isready            # PostgreSQL 15, port 5432, Homebrew — up
                      # database dvarpala_dev, user/password dvarpala/dvarpala_password
redis-cli ping        # Redis, port 6379 — up
curl localhost:8080/api/v1/users        # NOT your build — this is the Lima VM's
                                        # Dvarpala. Lima forwards 8080. Run a
                                        # local build on 8099 (§6).

limactl list          # Ubuntu VMs: "dvarpala" (running), "dvtest" (running)
```

**Lima, not Multipass.** Multipass could not get working networking on this Mac;
Lima worked immediately.

- **`dvarpala`** — OpenVPN only, no application. The workbench, from before the
  installer existed.
- **`dvtest`** — a full install, now **bridged onto the LAN at `192.168.0.41`**.
  Portal at `http://192.168.0.41:8080`, VPN on `192.168.0.41:1194/udp`, Google
  credentials and `mode: debug` configured. A real phone connected to it.

**Bridged networking is set up and working — do not casually undo it.** It
needed `socket_vmnet` copied into `/opt` (Lima refuses Homebrew's path: it walks
the whole path chain demanding root ownership and rejects symlinks), a sudoers
rule from `limactl sudoers`, and `networks: - lima: bridged` appended to
`~/.lima/dvtest/lima.yaml`. Three rounds of sudo to get right.

**Your Mac cannot reach `192.168.0.41`,** and that is expected: macOS will not
route to a guest bridged onto the interface the host is using. The VM reaches
everything including the Mac, and other devices reach the VM. It affects nothing
— the Mac was never the client.

**The Mac runs Redis 8.10; Ubuntu ships 6.0.** That difference hid a
login-breaking bug for days. Assume the Mac is not representative of a customer.

**Config:** `configs/environment.yaml`, literal values (no `${VAR}` — Viper does
not expand them, which was a real bug). Google OAuth client ID is in there. The
client secret was pasted into chat at one point and should be rotated.

---

## 6. Verification ritual

Run this after any change — the user explicitly asks for it:

```bash
export PATH=$PATH:/usr/local/go/bin
go build ./... && go vet ./... && go test ./... -timeout 240s
gofmt -l . | grep -v vendor          # must print nothing
(cd scripts/installation && go build ./...)
bash -n scripts/install/install-dvarpala.sh
for f in scripts/openvpn/*.sh; do bash -n "$f"; done
```

**Run it again after committing, not only before** — another session commits
into this repository while you work (§3).

Last known state (24 Aug): **all pass, tree gofmt clean.** Tests live in six
packages — `internal/api/vpnapi`, `internal/app`, `internal/auth`,
`internal/services`, `internal/vpn`, `internal/web`. The ones needing Redis
skip themselves if it is not running.

To exercise a running server locally use port 8099 — Lima holds 8080:

```bash
SERVER_PORT=8099 AUTH_OTP_ENABLED=true go run ./cmd/dvarpala-server
curl -s -o /dev/null -w '%{http_code}\n' localhost:8099/
```

Clear leftover sessions before testing a login by hand, or a session created by
an earlier test signs the browser straight in:

```bash
redis-cli --scan --pattern 'auth:*'    | xargs -r redis-cli DEL
redis-cli --scan --pattern 'session:*' | xargs -r redis-cli DEL
dvarpala-cli session list              # should say nobody is signed in
```

---

## 7. Traps this session hit — don't repeat them

- **`curl -fsSL … | bash` returns 0 even when curl 404s.** The pipeline's exit
  status is bash's, and bash succeeds at running nothing. This is why a broken
  installer URL went unnoticed. Was mid-demonstration when this session ended.
- **`pkill -f <pattern>` matches the invoking shell itself.** Cost this session
  several self-kills, and once killed the OpenVPN *server*. Use bracket regex
  (`[o]penvpn`) or PID files.
- **iptables `MARK` is non-terminating.** Packets fall through to later rules.
  The original firewall design was broken for this reason; replaced with an ipset
  and a `DVARPALA` chain that actually terminates.
- **Gin trusts `X-Forwarded-For` from any peer by default.** Introduced a session
  spoofing hole. Fixed with `SetTrustedProxies` (empty by default) + tests.
- **OpenVPN takes ~364s to notice a vanished client** (it doubles keepalive), so
  "instant revocation" is not instant. Needs the management interface.
- **Editing the three provider files is dangerous.** They are near-identical and
  ~1000 lines each. A function-boundary script deleted the wrong functions here;
  the recovery was `git checkout HEAD` on all three and a conservative redo.
  Always `git diff -w` afterwards to separate real changes from gofmt noise.
- **Lima forwards port 8080 to the Mac.** `curl localhost:8080` reaches the
  **VM's** Dvarpala, not anything you start locally. Test a local build on
  another port (`SERVER_PORT=8099`) or you will debug the wrong process.
- **On EC2, `hostname -I` returns the private address.** The installer falls
  back to it when `--host` is omitted, and then every `.ovpn` it issues points
  somewhere unreachable. Always pass `--host <PUBLIC-IP>`.
- **A 302 from the login flow is not success.** Failures redirect to
  `/auth/error` with the reason in the query string. Print `%{redirect_url}`,
  never just the status code.
- **Untracked files are not protected by anything.** A first OTP implementation
  was deleted before it was ever committed and was unrecoverable — no stash, no
  dangling objects. Commit work in progress before restructuring around it.
- **`.gitignore` needs leading slashes** for binaries (`/dvarpala-cli`), or it
  ignores the `cmd/dvarpala-cli/` source directory too.
- **Sessions are keyed by client IP as well as by cookie.** On localhost every
  process shares `::1`, so a session created by a curl test silently logs the
  browser in — the portal jumps straight to the success page without asking
  Google. Run `redis-cli DEL "auth:::1"` before testing a login by hand.
- **Ubuntu 22.04 ships Redis 6.0, which has no `GETDEL`.** The login path used
  it, so **login was impossible on every real install** while working fine on
  the Mac's Redis 8.10. Fixed with an equivalent Lua script (`consumeStateScript`
  in `internal/auth/service.go`), committed in `8208eed`, and since proven on
  Ubuntu. Before adding a Redis command, check it exists in 6.0.
- **Building inside a VM needs `-buildvcs=false`** — the copied source is a git
  repository the VM's user cannot read, and Go refuses to stamp it.
- **A restarted server that failed to bind leaves the old one answering.** On
  24 Aug `pkill -f dvarpala-server` missed the process, the replacement exited
  with `bind: address already in use`, and half an hour went into "diagnosing"
  behaviour that was simply the previous build. It produced a confident and
  entirely wrong bug report. **After restarting, grep the log for
  `starting on port` AND for `failed to start` before testing anything.** Kill
  by port, not by pattern:
  `for pid in $(lsof -nP -iTCP:8099 -sTCP:LISTEN -t); do kill -9 $pid; done`
- **Gin prefers a literal path over a wildcard sibling**, in either registration
  order — verified. `/auth/break-glass` alongside `/auth/:provider` works. Three
  comments in this codebase claimed the opposite; corrected in `102a1b2`. The
  routes that sit outside `/auth/` do so because they are pages rather than
  providers, not because the router forces it.
- **`smtp.gmail.com` is many machines behind one name, and an individual one can
  be transiently unreachable.** A send failed with `dial tcp 192.178.211.109:587:
  i/o timeout` and an identical send succeeded seconds later. Port 587 was open
  throughout. The mailer now retries the connection three times, 300 ms apart,
  inside the same overall budget. Do not diagnose this as a blocked port without
  checking `nc -z smtp.gmail.com 587` first.
- **A retry with no pause is not a retry.** The first version retried instantly;
  against a refused connection all three attempts landed in the same instant.
  Caught by a test written to fail if the retry was ineffective.
- **`tac` does not exist on macOS** — use `tail -r`. Likewise `timeout`; use the
  tool's own timeout flag.

---

## 8. What is left

**Verified working:** Google login, **sign-in by emailed code end to end
through Google Workspace into a real mailbox** (24 Aug), emergency access when
mail fails, per-user access enforcement, per-user certificates, one-command
install, users/groups/permissions, a readable audit trail, a session list, and
the full connect → login → reconnect → restricted-access → disconnect loop
**on real AWS infrastructure** (§10).

**Open, most valuable first:**

| Item | Note |
|---|---|
| Merge to main | 88 commits. `main` still has no `scripts/install/`, so nothing outside this branch can install Dvarpala at all. Blocked on the branch rule above. |
| **Make installation easy for customers** | The agreed next piece of work. Today's install still needs the repo, Go, and AWS credentials on the operator's own machine. The README advertises a one-line `curl \| bash` install whose URL 404s, and that pipeline exits 0 when the download fails - so a customer following it gets no install and no error. |
| **Rotate the Google app password** | A Google Workspace app password for `solutions@frigga.cloud` was pasted into a chat transcript on 24 Aug to prove delivery. It still works. Revoke it at `myaccount.google.com/apppasswords`, issue a fresh one, and keep it only in `AUTH_SMTP_PASSWORD`. |
| **Mail is sent from a person's own account** | Codes currently leave as `solutions@frigga.cloud`. A `noreply@frigga.cloud` Workspace account is the production answer: today, the day that person's password changes, nobody can sign in to the VPN. |
| **Decide the split tunnel** | See §9. A product decision, then two lines. Nothing else on this list is blocked on the user like this one, and it is now the last thing between this and something a stranger could be handed. Needs a live EC2 box — it is iptables and routing, untestable locally. |
| **HTTPS and a domain name** | Google refuses `http://` and refuses IP addresses, so **Google login cannot work on any deployment without a domain and a certificate**. There is no TLS anywhere in the code — `ListenAndServe`, no cert paths in config, nothing in the installer. Also fixes the IP-churn problem below. Highest-value missing piece. |
| **AWS and GCP allocate no static IP** | Azure does (`--allocation-method Static`). Stop and start an EC2 instance and it gets a new public address, so **every issued `.ovpn` points somewhere dead** — recovery means reissuing profiles to every employee. A domain name in the profile fixes this properly, since OpenVPN resolves it on each connect. |
| **No backups** | `/opt/dvarpala/certs/ca.key` cannot be recreated. Lose it and every certificate ever issued becomes unverifiable. The database can be rebuilt by hand; the authority cannot. ~6 KB, unprotected, on one machine. |
| Resources with no IP are silently unroutable | `toRoutes` in `vpnapi` skips any grant whose resource has no `IPAddress`. It shows as granted in the CLI and the console, produces no route and no firewall entry, and says nothing. The uncommitted console warns about it at creation time; the underlying behaviour is unchanged. |
| Group hierarchy does not inherit | Groups have a `parent_id` and the group service supports nesting, but `ResourcesForUser` considers direct membership only. A member of a child group gets none of the parent's grants. The TODO is in `permission.go`. |
| Certificate CN is not cross-checked | Access keys on the Redis session, not on the certificate identity. Defensible (two independent factors) but should be a decision. |
| DNS pushed is `8.8.8.8` | Cannot resolve internal names, so resources only work by IP. Should be the customer's own resolver. |
| Session keyed to the tunnel address | Nothing pins a client to the same address across reconnects; a new address means a lost session. |
| Certificate revocation list | A revoked cert still completes a handshake; the database check refuses access, so it is a second line of defence, not a hole. |
| Microsoft / GitHub / GitLab OAuth | ~40 lines each, the abstraction is ready. Microsoft matters most for enterprise. |
| Sign-in codes depend on one mail provider | If Google Workspace is unreachable, nobody signs in. Break-glass covers an administrator; ordinary users simply wait. A Postfix relay on the appliance would hold and retry the message — worth less than it sounds, since codes expire in five minutes, but it is the standard answer. |
| Deactivating a user does not end their session | Confirmed by design and now stated in `session end --help`. `user deactivate` blocks future sign-ins; the open session runs until it expires. `session end <tunnel-ip>` is the actual removal. Consider making deactivation do both. |
| No user can ever be deleted | `audit_logs.user_id` has a foreign key with no delete behaviour, so PostgreSQL refuses. Arguably correct — a trail should not be erasable — but it means deactivation is the only offboarding path and nothing says so. |
| `permission access <email>` exits 0 for a user that does not exist | It prints the parent command's help instead of running, so a check returns success having checked nothing. |
| 20 template files | All 0 bytes. |

**Roughly 90%** toward something a customer could run - and the remaining tenth
is almost entirely about installation, not about the product. On 26 Aug the
whole journey ran on a server the installer built by itself: connect, walled
garden, a code by email, sign in, reach one granted private address and be
refused another. What is left is making that installation something a stranger
can perform.

## 9. The first real deployment — 18 August

A `t3.small` Ubuntu 22.04 instance in `eu-north-1`, built by hand (not by the
cloud installer), source `rsync`'d up, `install-dvarpala.sh` run over SSH.
**Terminated at the end of the day.** Rebuild with the steps in §10.

**Proven for the first time:**

- Bare Ubuntu → working server, all 15 steps, over the real internet
- Tunnel forms behind NAT; per-user certificate accepted
- Walled garden holds — watched the firewall's DROP counter go 0 → 5
- Promotion works — after login the same packets hit the ACCEPT rule instead
- The audit trail answered a live debugging question in one query

**Found, and only findable on real infrastructure:**

1. The internal VPN API is reachable by any VPN client (see §8)
2. The firewall did not survive reboot — **fixed**, `882da46`
3. Step 15 of the installer hides its own failures (see §8)
4. Login failures in the auth machinery leave no audit record (see §8)

**Things that look like bugs and are not:**

- Tunnelblick warns about DNS — correct. Unauthenticated clients are pushed no
  DNS at all, and reach the portal by address.
- The portal's CSS loaded from a CDN and worked — because that traffic never
  entered the tunnel, so the firewall never saw it. (Same cause as the split
  tunnel below.) **Fixed on 24 Aug**: the portal now serves its own styling, so
  it will still render once the tunnel carries everything.

### The split tunnel — a real gap, and the one decision outstanding

Tunnelblick's *"public IP did not change"* warning is **not** a false alarm, and
an earlier note in this file said it was. A phone connected on 18 Aug reached
every website on the internet while unauthenticated.

The installer records the choice deliberately:

> `# No redirect-gateway: a client gets nothing beyond the portal until the`
> `# connect hook decides otherwise.`

**The premise does not hold for a real client.** Withholding a route is not a
denial — it is an abstention. Traffic that is not routed into the tunnel leaves
over the device's own Wi-Fi, so the firewall never sees it and cannot drop it.
Every earlier test hid this because the client *was* the server, where all
traffic was already on the machine the firewall runs on.

So today Dvarpala does *"you cannot reach company resources until you
authenticate"*. The README promises *"users gain access only to a captive
portal"*. Those are different products, and the gap must be closed from one end
or the other.

**Fix:** push `redirect-gateway` always, before and after login, and let the
ipset be the only thing that decides. The firewall rules are already correct.
**But it changes what the product is** — all of an employee's traffic would go
through the company gateway, and they would reach no internet at all unless
explicitly granted. That is stricter than most people expect and is a decision
for the user, not an implementation detail.

**Still outstanding on 24 Aug.** Nothing since has changed the tunnel's shape,
so this remains the one decision blocking a customer-ready product. The two
pieces it needs are now both in place: the portal renders without the internet,
and the OpenVPN management interface (`5cf1d41`) can force the reconnect that
applies a login.

---

## 9a. Sign-in by emailed code, proven — 24 August

The decision recorded on 18 Aug was **codes only, no OAuth**. On 24 Aug that
became real. No AWS instance was involved; everything below was done locally
against the actual Google Workspace that serves `frigga.cloud`.

**How the mail provider was chosen.** `dig MX frigga.cloud` answers
`aspmx.l.google.com` — the domain is on Google Workspace. That settles it:
Dvarpala hands the message to Google, which already has the sending reputation
that gets mail into inboxes. A server sending directly from a fresh cloud IP
would be filtered as spam and, on AWS, blocked on the port outright. Look this
up rather than asking; it is one command.

Settings that work, with the password supplied only through the environment:

```yaml
auth:
  otp: { enabled: true }
  smtp:
    host: smtp.gmail.com
    port: 587
    username: solutions@frigga.cloud
    from: solutions@frigga.cloud
    from_name: Dvarpala
    # AUTH_SMTP_PASSWORD — a Google *app* password, not an account password
```

**Proven, in a browser, with a real inbox:** typed an address on the portal →
Google accepted the message in ~4.5 s → a six-digit code arrived → it was
exchanged for a session carrying `system_admins, engineering`.

**Also proven:** a wrong code is refused; a second code inside a minute is
refused; a replayed state token is refused; and an unknown address, a blocked
domain and a deactivated account all produce the *identical* "check your email"
page with **no message sent**. The portal is reachable by anyone holding a
certificate, so it must not become a way to enumerate an organisation's staff.
Refusals go to the trail instead.

### Break-glass

Codes being the only way in means a broken mail server locks out the person who
would repair it. `dvarpala-cli admin break-glass <email> --reason "..."` prints
a single-use link that signs that person in.

- Skips **one** thing: proving control of the mailbox. The account must still
  exist and be active, and its groups still decide what opens.
- Not a privilege escalation — anyone who can run it already holds the database
  credentials. What it adds is a record saying what it was.
- The link carries a **redemption code, not the session token**, because URLs
  are written into access logs. Redeeming spends it; a copy found later is
  worthless. Verified in a browser: second use refused.
- 15 minutes by default, two hours maximum, `--reason` required.
- `--client-ip <tunnel-ip>` also binds the tunnel. Without it the console opens
  and the network does not — the audit record says which (`binds_tunnel`).

### The portal is now self-contained

All three portal pages were loading Tailwind and Google Fonts from CDNs. Inside
a real walled garden those never arrive, so the first thing a new user would
have seen is an unstyled page. It only looked fine on 18 Aug because the split
tunnel meant that traffic never entered the tunnel at all — the same root cause
as §9's finding. Styling is now served from the portal itself.

The page also builds itself from `authSvc.Providers()`, so the three buttons
that led nowhere are gone.

---

## 9b. The whole thing worked - 26 August

On a server the cloud installer built unaided, in Mumbai, from a branch on
GitHub. Every step below is a thing that had never worked before that day.

    connect                     tunnel up, no access
    http://signin               the sign-in page, by a name
    enter email                 a real code, by email, through Brevo
    enter code                  signed in; tunnel restarts by itself
    ping 172.30.0.12            reachable - a private address that was granted
    ping 172.30.0.1             refused - same network, not granted

`session list` showed `SIGNED IN VIA: otp`, and the hook log showed
`pushed 1 route(s)` against the resource that had been granted. Eighteen
earlier connects had pushed none.

### What changed to get there

**The firewall decides destinations, not identity.** It held a list of who had
signed in and accepted everything from them; routes were the only thing keeping
anybody to their own resources, and a route is an instruction to the client's
own machine. Adding one by hand reached an ungranted server in three packets,
and nothing recorded it. It now holds `(client, destination)` pairs. Signing in
opens nothing by itself.

**A session is pinned to its certificate.** Sessions are keyed on tunnel
address, and OpenVPN reuses those, so for the two-minute grace window the next
client to receive an address inherited whatever the last holder earned. The
session records who signed in and OpenVPN reports whose certificate connected;
those two facts now meet.

**Mail goes through Brevo.** Google Workspace refuses an SMTP sign-in from a
datacenter address and reports it as bad credentials - proven by the same
password being accepted from a laptop and refused from the server minutes
apart. Brevo has no such heuristic. `BrevoMailer` sits alongside `SMTPMailer`;
SMTP is still the default and works with any mail server.

**Credentials live in `/etc/dvarpala/dvarpala.env`**, read by systemd and by
the CLI. A key set with `systemctl edit` reaches the service and nothing else,
so `dvarpala-cli mail test` reported no mail service on a server that was
sending mail - the one command whose purpose is to check exactly that.

**`dvarpala-cli` is on the PATH.** Every message the installer printed already
called it that; none of them worked.

**The resolver.** dnsmasq on the tunnel answers `signin` with the portal, and
DNS to anywhere else is now refused - it had been a two-way channel out of the
walled garden.

**Deactivation is reversible.** `user deactivate` had no counterpart, and
break-glass deliberately refuses an inactive account. Deactivating yourself
meant editing the database by hand.

### Decisions taken

**Administrators use the address, not the name.** `signin` is answered by this
server's resolver, which a client is given only inside the walled garden. After
signing in they keep their own resolver - their personal traffic is theirs -
so the name stops resolving at the point an administrator wants it.
`http://172.30.100.1:8080/admin` works in both states. Considered and rejected:
pushing our resolver after sign-in, which would route everybody's personal
lookups through the company server.

**A full tunnel before sign-in, a split tunnel after.** All traffic is carried
while unidentified, so the walled garden is a restriction rather than a
suggestion. Once signed in the default route is withdrawn and only the
company's addresses are carried. The reconnect that applies this is triggered
by the server, three seconds after signing in - long enough for the browser to
receive its cookie and the page saying it worked.

**HTTPS cannot be redirected to the portal, on any platform.** TLS exists to
prevent it. Blocked TCP is now refused with a reset so a browser fails in a
second rather than hanging for a minute.

**No operating system announces a captive portal when a VPN connects.** Not
macOS, not iOS, not Android - verified: the phone never made the check at all.
The check runs on joining a network, and a VPN is not a network join. The
address has to be communicated.

### The installer, which had never been run end to end

Fourteen defects found and fixed in two days, every one invisible until it met
real infrastructure: AWS errors reduced to `exit status 254`; a security group
found by a name it was never created with; racing its own boot script and
blaming apt; a VPC leaked per failed run until the account hit its limit; the
`--region` flag ignored in interactive mode; ports 8080 and 443 opened to the
internet; a successful install reported as a crash; a health check against a
port that had been closed; five minutes retrying a download that published a
private key on a public web root; the admin profile written to the wrong
directory; dnsmasq installed but never started; SSH instructions relative to
the wrong directory.

`connection-info.txt` is now seven numbered steps rather than a description of
an installation that did not exist.

---

## 10. Rebuilding the test box

Steps 1–3 are done permanently: root MFA, an IAM admin user, and the
`dvarpala-test` key pair and security group (SSH 22 + UDP 1194, both "My IP").

```bash
# launch Ubuntu 22.04 (jammy, amd64, Canonical), t3.small, 20 GB, public IP on
rsync -av --exclude .git -e "ssh -i ~/.ssh/dvarpala-test.pem" \
  ./ ubuntu@<PUBLIC-IP>:/tmp/src/
ssh -i ~/.ssh/dvarpala-test.pem ubuntu@<PUBLIC-IP> \
  "sudo mkdir -p /opt/dvarpala && sudo mv /tmp/src /opt/dvarpala/src"
ssh -i ~/.ssh/dvarpala-test.pem ubuntu@<PUBLIC-IP>
sudo /opt/dvarpala/src/scripts/install/install-dvarpala.sh \
  --source /opt/dvarpala/src --host <PUBLIC-IP> --admin richa.t@frigga.cloud
```

`--host <PUBLIC-IP>` is the one flag that silently ruins every profile if
forgotten. Terminate the instance when finished; nothing else costs money.

Do **not** use the cloud installer (`launcher.go`) yet: `aws.go:452` clones
`main` from GitHub, where none of this work exists.

---

## 11. The rest of the organisation

Checked across all 13 `frigga-cloud` repositories:

- **Nothing outside dvarpala references dvarpala.** The placeholder API
  endpoints (`/api/v1/groups`, `/vpn/status`, `/auth/validate`) have no
  consumers waiting on them, so building them out is optional work rather than
  a debt.
- **`user-service-2` is a mature NestJS service with 76 endpoints** already
  doing users, roles, policies, permission evaluation, OAuth, SSO and audit —
  including `GET internal/users/by-email`, which is exactly Dvarpala's lookup.
  Its OTP implementation is well built: `crypto.randomInt`, timing-safe
  comparison, 60-second cooldown, 10 a day, five attempts.
- **Dvarpala has independently built a parallel version of much of it**, so an
  administrator currently adds every person twice, and deactivating someone in
  one system leaves them active in the other.
- `Frigga-Accounts-Hub` is a React/Vite front end — the existing accounts UI.
- Dvarpala is the **only Go CLI in the organisation**; nothing else uses Cobra.

**Recommendation on record:** keep Dvarpala's own database authoritative for VPN
access. It is infrastructure — if it depends on another service being up, an
outage locks everyone out of the network, including whoever would go and fix
that service. But read from `user-service-2` where it is present so nobody is
entered twice. `internal/users/by-email` is the seam.
