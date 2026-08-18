# Dvarpala — session handoff

Written 2026-08-17, updated 2026-08-18 after the first real cloud deployment.
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
commits ahead:     27
pushed:            NO — all 27 exist only on this laptop
working tree:      configs/environment.yaml modified, deliberately uncommitted
main branch:       still the old pre-review code
```

**⚠️ The single biggest risk: nothing is pushed.** Four days of work on one disk.
The user has been told repeatedly; it is their call. Do not push unasked.

**⚠️ `configs/environment.yaml` holds a live Google client secret** and is left
uncommitted for that reason — the repository is public. It needs rotating, and
the replacement should come from `OAUTH_GOOGLE_CLIENT_SECRET` (already bound in
`config.go`) rather than the file.

**⚠️ Second issue:** the cloud providers clone `main`, which has none of this work.
A cloud install today would fetch the old broken code. This resolves itself when
the branch merges.

### The 27 commits, grouped

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
                       mailer.go    — SMTP, plus a log fallback (debug mode only)
                       service.go   — Begin/Complete/Logout, domain allow-list,
                                      RequestCode/VerifyCode

internal/vpn/pki.go    certificate authority in pure Go. IssueClient() puts the
                       user's email in the common name — that is how the VPN
                       knows who connected.

internal/api/vpnapi/   GET  /api/internal/vpn/access/:clientip   ← the hook calls this
                       DELETE /api/internal/vpn/session/:clientip

internal/web/auth.go   captive portal handlers
internal/web/otp.go    the two-step code form (/otp/login, request, verify)
internal/web/admin.go  read-only admin console at /admin, gated on system_admins

cmd/dvarpala-cli/      admin CLI: user, group, resource, permission, vpn

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
redis-cli ping        # Redis, port 6379 — up
curl localhost:8080/api/v1/users        # the app — returns 200

limactl list          # Ubuntu VMs: "dvarpala" (running), "dvtest" (stopped)
```

**Lima, not Multipass.** Multipass could not get working networking on this Mac;
Lima worked immediately. `dvtest` is the clean VM the installer was proven on.

**Config:** `configs/environment.yaml`, literal values (no `${VAR}` — Viper does
not expand them, which was a real bug). Google OAuth client ID is in there. The
client secret was pasted into chat at one point and should be rotated.

---

## 6. Verification ritual

Run this after any change — the user explicitly asks for it:

```bash
cd /Users/abcom/dvarpala && export PATH=$PATH:/usr/local/go/bin
go build ./... && go vet ./... && go test ./internal/...
(cd scripts/installation && go build ./providers ./installer)
curl -s -o /dev/null -w '%{http_code}\n' localhost:8080/api/v1/users
bash -n scripts/install/install-dvarpala.sh
```

Last known state (18 Aug): **all pass, app returns 200.** Tests now live in
four packages — `internal/app`, `internal/auth`, `internal/services`,
`internal/web` — and the auth ones skip themselves if Redis is not running.

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

---

## 8. What is left

**Verified working:** Google login, email-code login, per-user access
enforcement, per-user certificates, one-command install, users/groups/
permissions, the audit trail, and the full connect → login → reconnect →
restricted-access → disconnect loop **on real AWS infrastructure** (§10).

**Open, most valuable first:**

| Item | Note |
|---|---|
| Push the branch | 27 commits on one laptop. Still the highest risk. |
| **Internal VPN API is reachable by any VPN client** | `/api/internal/vpn/access/:ip` and `/session/:ip` are served on `:8080` with no restriction, though the package comment says "never beyond localhost". Any certificate holder can read anyone's access map and disconnect anyone. **Confirmed by curl on a live server.** Fix: localhost-only middleware on that route group. |
| Installer step 15 swallows every error | `>/dev/null 2>&1 \|\| true` on all three admin-creation commands, then prints a green tick. Cost an hour on 18 Aug. |
| Login-machinery failures are not audited | Unknown provider, bad state token and exchange failures all return from `Complete()` without a record. Only success and denial are logged. |
| Reconnect required after login | Routes are decided only at connect time. `openvpn.management` (localhost:7505) is already written into the generated config but nothing enables or uses it — that is the fix. |
| Portal offers four hardcoded OAuth buttons | Three do nothing, and it does not offer OTP. `authSvc.Providers()` exists for rendering the real list. |
| SMTP never tested | Only the debug-mode log fallback has been exercised. SES is the natural choice on AWS. |
| Certificate CN is not cross-checked | Access keys on the Redis session, not on the certificate identity. Defensible (two independent factors) but should be a decision. |
| DNS pushed is `8.8.8.8` | Cannot resolve internal names, so resources only work by IP. Should be the customer's own resolver. |
| Session keyed to the tunnel address | Nothing pins a client to the same address across reconnects; a new address means a lost session. |
| Certificate revocation list | A revoked cert still completes a handshake; the database check refuses access, so it is a second line of defence, not a hole. |
| Microsoft / GitHub / GitLab OAuth | ~40 lines each, the abstraction is ready. Microsoft matters most for enterprise. |
| 20 template files | All 0 bytes. |

**Roughly 85%** toward something a customer could run.

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

- Tunnelblick warns *"public IP did not change"* — correct. There is no
  `redirect-gateway`; only the portal is routed. It complains because you fixed
  the thing the old installer got wrong.
- Tunnelblick warns about DNS — correct. Unauthenticated clients are pushed no
  DNS at all, and reach the portal by address.
- The portal's CSS loads from a CDN and works — because that traffic never
  enters the tunnel, so the firewall never sees it.

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
