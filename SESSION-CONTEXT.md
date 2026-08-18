# Dvarpala — session handoff

Written 2026-08-17, to carry context into a fresh chat. Read this first, then
`DVARPALA-OVERVIEW.md` for the deep technical audit.

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
commits ahead:     22
pushed:            NO — all 22 exist only on this laptop
working tree:      clean
main branch:       still the old pre-review code
```

**⚠️ The single biggest risk: nothing is pushed.** Three days of work on one disk.
The user has been told; it is their call.

**⚠️ Second issue:** the cloud providers clone `main`, which has none of this work.
A cloud install today would fetch the old broken code. This resolves itself when
the branch merges.

### The 22 commits, grouped

| Stage | What it did | Commits |
|---|---|---|
| **Make it run** | Removed duplicate `main()` stubs; fixed a 404 installer URL; replaced `${VAR}` config placeholders Viper never expanded | `f72bb54` `f794b08` `266e3b8` |
| **The rulebook** | Service layer: users, groups, resources, permissions; CLI commands; the "what may this email reach" query | `c988c5e` `fde1b3e` `93c90a4` `92fe2fe` |
| **Login** | Redis session store, OAuth provider abstraction, dev provider, first tests, captive portal served, Google login | `565da0d` `00a0a0c` `c0cf2bf` |
| **The gate** | VPN access API, connect/disconnect hooks, ipset firewall, per-user certificates, disconnect grace period | `5c3cb53` `cf237de` `7363c00` `8c1ed27` |
| **Install anywhere** | One script, bare Ubuntu → working system; providers now call it; dead code removed | `15aaed1` `53c9bbd` `11ea003` |
| **Security** | Don't trust `X-Forwarded-For`; stop publishing `admin.ovpn` on a public web root | `c5cc210` `26542e9` |
| **Docs** | The system review | (latest) |

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
                       service.go   — Begin/Complete/Logout, domain allow-list

internal/vpn/pki.go    certificate authority in pure Go. IssueClient() puts the
                       user's email in the common name — that is how the VPN
                       knows who connected.

internal/api/vpnapi/   GET  /api/internal/vpn/access/:clientip   ← the hook calls this
                       DELETE /api/internal/vpn/session/:clientip

internal/web/auth.go   captive portal handlers

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

Last known state: **all pass, 0 failing tests, app returns 200.**

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
- **`.gitignore` needs leading slashes** for binaries (`/dvarpala-cli`), or it
  ignores the `cmd/dvarpala-cli/` source directory too.

---

## 8. What is left

**Verified working:** Google login, per-user access enforcement, per-user
certificates, one-command install, users/groups/permissions, the full
connect → login → reconnect → restricted-access → disconnect loop on real
infrastructure.

**Not done:**

| Item | Note |
|---|---|
| Push the branch | 22 commits on one laptop. Highest priority. |
| Cloud path | `launcher.go` never run against a real AWS/GCP/Azure account |
| Certificate revocation list | A revoked cert still completes a TLS handshake; access is refused by the database check, so it is a second line of defence, not a hole |
| Instant revocation | ~4 minutes today; needs the OpenVPN management interface |
| Admin web console | User management needs SSH today; the user has asked why more than once — they want a portal |
| Group/resource API endpoints | Still placeholder strings. The CLI works. |
| Microsoft / GitHub / GitLab OAuth | ~40 lines each, the abstraction is ready |
| 20 template files | All 0 bytes |

**Roughly 80%** toward something a customer could run.

---

## 9. Where the last conversation stopped

The user asked *"HOW make it run, like how did we test that, and URL from what"* —
wanting the concrete mechanics behind the "make it run" stage. Two of three
answers were delivered:

1. **The URL** — `friggalabs` (does not exist, 404) vs `frigga-cloud` (correct).
   Diff shown, present in all three provider files.
2. **Why it went unnoticed** — the `curl | bash` exit-code trap above. The live
   demonstration hung on a network call and was killed.
3. **How we tested** — not yet answered. This is the open thread.

The answer to (3), for reference: the app was built and run locally against real
PostgreSQL and Redis; `AutoMigrate` created 14 tables which were inspected
directly; the full VPN loop was exercised against a real OpenVPN server on the
`dvarpala` Lima VM; and the installer was run start-to-finish on the clean
`dvtest` VM, ending with a successful handshake showing
`cn=richa.t@frigga.cloud`.
