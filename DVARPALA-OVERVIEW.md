# Dvarpala — End-to-End System Overview

> **Subject:** the Dvarpala repository at commit `cff0373` ("Adding rules to allow
> Oauth") on `main`.
> **Written:** August 2026, by reading that commit — source, scripts, the
> requirement PDF and git history.
>
> **Verification level:** both Go modules have been **compiled** (Go 1.26.5), the
> server binary **run**, and several inferences **empirically tested** — see §7,
> §8.7, §9.6. No database, VPN server or cloud account was available, so claims
> requiring live infrastructure remain marked `[INFERRED]`.

---

## Read this first: what has changed since this review

This document is a **point-in-time audit of `cff0373` on `main`**, and it is left
that way deliberately — its purpose is to describe the code as the team wrote it,
so the findings can be checked against what was actually there.

Work since then, on the unmerged branch `dev/foundation`, has fixed several of the
problems below. **The sections describing them have not been rewritten.** Where
this table says *fixed*, read the section as history rather than as the current
state of that branch.

| § | Finding | Status on `dev/foundation` | Commit |
|---|---|---|---|
| §8.7 | Installer fetched from `friggalabs`, a 404 — and the pipeline still exited 0 | Fixed: correct URL, and failure now stops the install | `f794b08` |
| §6.2 | `environment.yaml` used `${VAR}` placeholders Viper does not expand, so config silently loaded empty | Fixed: literal values | `266e3b8` |
| §9.10 | Firewall used `MARK`, which is **non-terminating** — packets fell through and the walled garden did not hold | Fixed: replaced with an ipset + `DVARPALA` chain that terminates | `7363c00` |
| §9.4 | One shared client certificate, so every VPN client was anonymous and interchangeable | Fixed: per-user certificates, email as common name | `8c1ed27` |
| §9.1 | The captive portal was written but nothing invoked it | Fixed: connect/disconnect hooks now call the application | `5c3cb53` |
| §10 | The installer provisioned a plain VPN and never installed Dvarpala itself | Fixed: one script, bare Ubuntu to working system; tested on a clean VM | `15aaed1`, `53c9bbd` |
| §9.8 | `admin.ovpn` — containing a private key — was served world-readable over plain HTTP for 120s | Fixed: copied over the existing SSH session, shredded server-side | `26542e9` |
| §8.1 | Every endpoint returned a placeholder string | Partly fixed: users and VPN access are real; **groups and resources are still placeholders** | `c988c5e` |
| §7.4 | No Go tests | Partly fixed: 14 tests covering sessions, providers and proxy trust — coverage is still thin | `565da0d`, `c5cc210` |

One defect in the table was **introduced during this work, not found by it**: Gin
trusts `X-Forwarded-For` from any peer by default, which let a client bind its
session to another client's tunnel address. Demonstrated, fixed in `c5cc210`, and
covered by regression tests.

**Still true as written**, and the main things left: no certificate revocation
list (§9.7), so a revoked certificate still completes a handshake; revocation
takes roughly four minutes rather than being immediate; there is no admin web
console, so user management needs SSH; the cloud provisioning path (§10) has
**never been run against a real account**; and the 20 empty template files (§8.4)
are still empty.

---

## At a glance

**Dvarpala is a well-designed Zero Trust VPN that has been specified three times,
partially built twice, and never assembled.** The specification is thorough, the
installer is production-shaped code that works, and the VPN it deploys is real —
but ordinary. The Zero Trust layer that makes Dvarpala *Dvarpala* exists as design
artefacts in three separate files, **none of which execute.**

| Component | Size | State |
|---|---|---|
| **Cloud installer** (AWS/GCP/Azure) | ~4,600 lines Go | ✅ **Works** — clean abstraction, real three-cloud parity |
| **VPN it deploys** (tunnel, PKI, certs) | — | ✅ **Works** — but a plain full-tunnel VPN |
| **Captive portal UI** | ~1,400 lines HTML/JS | ✅ **Exists** — polished; served by nothing in the live path |
| **Database schema** | 14 tables | ✅ **Well designed** — never created on any deployment |
| **Go application** | 1,608 lines | ⚠️ **Compiles and starts**; every endpoint returns a placeholder string |
| **Captive portal / two-step auth** | 3 designs | ❌ **Written, never executed** — two are refinements of one model (§9.10); the third is the branch's (§12.1) |
| **Admin dashboard & user portal** | 20 template files | ❌ **All 0 bytes** |
| **Tests** | 12 files | ❌ **All stubs — no Go test coverage** (one shell harness exists, §7.4) |

**The five findings that matter most**

1. The captive portal — the entire product differentiator — is **fully written in a
   script that nothing ever invokes** (§9.1). What deploys is an ordinary VPN that
   grants full access on connect (§9.2). The repository holds **three complete
   designs for how access should be gated, none of which executes** (§9.3, §9.10,
   §12.1) — successive refinements, not alternatives. (Distinct from the three
   *web-server implementations* in §8.6.)
2. **Nothing in the system can report failure.** The OAuth firewall step fetches a
   404 and reports success; `/health` returns a hardcoded "healthy"; config loading
   fills in blanks silently. Every checkpoint is green on a broken install (§11.3).
3. **34% of tracked files are empty** and 79% of Go files are bare stubs, because
   the whole directory tree was generated from the spec up front (§2.1).
4. **Every user shares one identity** (`CN=admin`), and `admin.ovpn` — containing
   the private key — is published over plain HTTP for 120 seconds during install
   (§9.7, §9.8).
5. The documented developer workflow **cannot work**: `make build`, `make dev` and
   `make install` all fail, for four independent reasons (§6.5, §7.5).

**Verified by execution, not just read:** both Go modules compiled, the server
binary run as systemd runs it, Viper's config behaviour reproduced, GORM's schema
parsed, the `friggalabs` URL tested, the unmerged branch built. See §13.1.

### Where to start

| If you are… | Read |
|---|---|
| **Taking over development** | This page → §1 (what it is) → §2 (why the repo misleads) → §3 (concepts to learn) → then §8–§10 |
| **The original developer, checking accuracy** | §13.1 (what was tested) → §13.2 (confidence and limits) → then any section; every claim carries a `file:line` |
| **Deciding what to do next** | This page → §9.1–9.2 → §12.4 (the branch recommendation) → Open Questions |
| **In a hurry** | This page and §13.3 |

### Contents

- [0. How to read this document](#0-how-to-read-this-document) — evidence tags
- [1. What Dvarpala is](#1-what-dvarpala-is) — the two-step model, public positioning
- [2. Three generations](#2-critical-context-this-repository-has-three-generations) — **why the repo misleads**
- [3. Prerequisites](#3-prerequisites--what-to-understand-before-reading-the-code) — concepts to learn first
- [4. Timeline](#4-development-timeline-and-current-state) — branches, dormancy
- [5. The data model](#5-the-data-model) — 14 tables, 4 defects
- [6. Configuration](#6-configuration) — the `${VAR}` failure, broken Makefile
- [7. Build/test status](#7-build-vet-and-test-status--measured) — measured
- [8. The Go control plane](#8-the-go-control-plane--what-actually-runs) — **three different servers**
- [9. VPN & captive portal](#9-the-vpn-and-captive-portal-data-plane) — **designed vs deployed**
- [10. The cloud installer](#10-the-cloud-installer) — the part that works
- [11. Operations & integration](#11-operations-surface-and-platform-integration) — monitoring, standalone status
- [12. The unmerged branch](#12-the-unmerged-branch-installerrestructure) — adopt or discard
- [13. Consolidation](#13-consolidation) — corrections, confidence, summary
- [Open questions](#open-questions-for-the-developers) — 5 resolved, 25 need a human

---

## 0. How to read this document

This document was produced by reading the repository directly — source files, shell
scripts, the requirement PDF, and git history — not by summarising the existing
README. Where the existing documentation and the actual code disagree, **the code
wins and the disagreement is recorded**, because those disagreements are the most
useful thing a newcomer can be told.

### 0.1 Evidence tags

Every substantive claim carries one of these tags so a reviewer can check it quickly:

| Tag | Meaning |
|---|---|
| `[VERIFIED: path:line]` | Read in the source at that location. Reproducible. |
| `[VERIFIED BY EXECUTION]` | Compiled and/or run in a test harness. The observed output is quoted. |
| `[SPEC-ONLY]` | Described in the requirement doc / README / context docs, but **no implementing code found**. Intent, not reality. |
| `[SCAFFOLD]` | A file or package exists at this path, but it is empty or a bare `package x` declaration. Structure without behaviour. |
| `[INFERRED]` | A conclusion drawn from evidence rather than read directly. The reasoning is stated so it can be challenged. |
| `[OPEN]` | Could not be determined from the repository. Needs a human answer. Collected in §Open Questions. |

### 0.2 What this document is not

It does not describe a running deployment. No VPN server, database or cloud
environment was contacted. The Go module *was* compiled and two specific behaviours
were reproduced in an isolated test harness (§7); everything else is source
reading. Where runtime behaviour matters and could not be tested, this document
states what the code *would* do and marks it `[INFERRED]`.

---

## 1. What Dvarpala is

### 1.1 In one paragraph

**Dvarpala** (द्वारपाल — Sanskrit for "gatekeeper" or "door guardian") is a
**Zero Trust VPN gateway** built on OpenVPN. Its distinguishing idea is that
connecting to the VPN does not, by itself, grant you network access. A user
connects with shared temporary credentials and lands in a **walled garden** where
the only thing reachable is an authentication portal. Only after the user proves
identity through a corporate OAuth provider **and** is confirmed to exist as an
active user in Dvarpala's own database does the firewall open up and grant real
network access. Access is bound to the session — disconnect, and the user must
re-authenticate. `[VERIFIED: README.md:5-14]` `[VERIFIED: Requirement Doc.pdf p.1-3]`

The intended role is to be **the single front door to all organisational
resources** — dashboards, VMs, databases, internal services — with access
granted per user-group rather than per user. `[VERIFIED: Requirement Doc.pdf p.1]`

### 1.2 The two-step access model

This is the core concept. Everything else in the system exists to serve it.

```
                    STEP 1                              STEP 2
   ┌──────────┐   temp creds    ┌──────────────┐  OAuth + DB check  ┌─────────────┐
   │  User's  │ ──────────────► │   CAPTIVE    │ ─────────────────► │    FULL     │
   │  OpenVPN │   (shared,      │   PORTAL     │                    │   ACCESS    │
   │  client  │    static)      │   network    │                    │   network   │
   └──────────┘                 └──────────────┘                    └─────────────┘
                                       │                                   │
                          reachable: ONLY the                  reachable: routes pushed
                          Dvarpala auth portal                 per the user's group
                          everything else DROPped              permissions
                                                                           │
                                                            on disconnect: revoked,
                                                            must re-authenticate
```

The two "factors" are deliberately of different kinds `[VERIFIED: Requirement Doc.pdf p.3]`:

1. **OpenVPN certificate** — device-level trust (you have the `.ovpn` file).
2. **OAuth validation** — identity, via a company Google/Microsoft/GitHub account.
3. **Database authorisation** — the user must be *explicitly added to Dvarpala*.
   A valid company Google account is not sufficient on its own.
4. **Session management** — time-bound, with automatic expiry.

> **What "full access" means differs by design.** Both server designs in this
> repository implement the two steps above; they differ only in what the second step
> grants — the whole internet through the tunnel (§9.3), or only the company's
> protected resources with everything else going direct (§9.10, split tunnelling).
> The gate is the same; the size of the opening is not.

Point 3 is the crux of the design: **OAuth proves who you are; the Dvarpala
database decides whether you are allowed in.** Passing OAuth while absent from the
database is an explicit failure case that gets audit-logged as an unauthorised
attempt. `[VERIFIED: Requirement Doc.pdf p.13]`

### 1.3 Intended user journey

**One-time, per device** `[VERIFIED: Requirement Doc.pdf p.7]`:
1. An admin issues the user a `.ovpn` file.
2. The user installs an OpenVPN client and imports the file.

**Every session** `[VERIFIED: Requirement Doc.pdf p.7-8]`:
1. User clicks Connect. The client authenticates with **static shared credentials** —
   username `temp_user`, password `temp_portal_access` — plus the embedded certificate.
2. The server grants captive-portal-only access and the user's browser is directed
   to the Dvarpala portal.
3. The user picks an OAuth provider and completes login with their company account.
4. Dvarpala validates in two stages — email domain is on the allow-list, **and** the
   user exists in the database with status `active` and has group assignments.
5. On success, the authenticated session is written to Redis, the client's network
   access is upgraded, and routes are pushed according to the user's group permissions.
6. On failure, the user stays in the captive portal. Repeated failures trigger
   progressive IP blocking (3 strikes → 20-minute iptables DROP). `[SPEC-ONLY — Requirement Doc.pdf p.14]`

> ⚠️ **The published numbers are wrong.** The requirement doc says captive
> `10.8.0.x` / full `10.8.1.x`; the README says `192.168.100.0/24` / `10.8.0.0/24`;
> the config files say `192.168.100.0/24` / `10.0.0.0/8`. **None matches what is
> deployed.** The live cloud installer uses **captive `172.30.100.0/24`, full
> `172.30.8.0/21`** — see §9.2. The three documented schemes belong to the
> superseded provisioning path or to nothing at all. *(Resolved — OPEN-1, OPEN-9.)*

### 1.4 How it is publicly positioned today

`[VERIFIED: https://frigga.cloud/product/dvarpala, fetched Aug 2026]` The public
product page markets Dvarpala as an **"identity-first VPN and access control
platform"**, status **"In Development"**, with an early-access waitlist and an
**open-core** model (Free / Team $8 per user per month / Enterprise custom).

Its headline promise is *"Remove someone from your IdP — their VPN access
disappears instantly."*

**This is a meaningfully different emphasis from the repository.** The public
framing is *IdP lifecycle sync* — access derives from, and is revoked by, the
identity provider. The repository implements *DB authorisation* — the Dvarpala
database is the authority, and a user must be explicitly added to it (§1.2 point 3).
Those are complementary but not the same mechanism, and nothing in the code
synchronises with an IdP's user directory. `[INFERRED — no directory-sync code found
at cff0373]`

Mapping the advertised features (16 at the time of writing) against the repository:

| Advertised | State in this repository |
|---|---|
| Identity-first VPN via SSO | Partially — OAuth login modelled and configured; **Okta not supported**, only Google/Microsoft/GitHub/GitLab `[VERIFIED: models/oauth_provider.go:60-98]` |
| Multi-cloud connectivity (AWS/GCP/Azure) | **Strongest match** — all three clouds implemented in the installer (§2.2) |
| Role-based access control | Schema present (`groups`, `group_permissions`); enforcement code is stubs `[VERIFIED]` |
| Full audit trail | `audit_logs` table exists; no writer code found `[VERIFIED: §5.3]` |
| Session management, idle timeout | Config + schema present (`AUTH_SESSION_DURATION`, `sessions.last_used_at`) |
| Instant access revocation | `vpn_configs.status='revoked'`, `expires_at` in schema; no revocation code found |
| **MFA (TOTP / WebAuthn)** | ❌ No implementation, no schema, no dependency |
| **Geo & device policies** | ❌ Not present (`ip_whitelists` is IP-based only) |
| **Split tunnelling** | ⚠️ Absent from `internal/` and from the deployed config — but a **designed implementation exists** in the unexecuted selective-blocking subsystem (§9.10) |
| **Suspicious activity alerting** | ❌ Not present |
| **Sensitive service classification** | Partially — `resources.type` exists, but has no prod/staging distinction |
| **Automated user lifecycle** | ❌ No IdP directory sync |
| **JIT access · Zero lingering credentials** | ❌ No implementation; no time-bounded grant mechanism beyond session expiry |
| **Open-core architecture** | ✅ Accurate — the repository is publicly readable |

`[INFERRED]` The product page describes the intended commercial product; the
repository at `cff0373` is an earlier stage of it. Given the page says "In
Development", this is expected rather than a contradiction — but a newcomer should
not assume the advertised feature list reflects existing code. **Roughly half of
the marketed capabilities have no counterpart here**, and the page is edited over
time: it listed 12 features when this review began and 16 shortly after.

### 1.5 Who it is for

Named target audiences, each mapped to a class of resource `[VERIFIED: Requirement Doc.pdf p.2]`:
Developers (dev environments/tools), DevOps (infrastructure), Finance (financial
dashboards), Management (analytics), IT Administrators (system administration).

The public page names a compatible but broader set: IT & security teams preparing
for SOC 2 / ISO 27001, platform engineers running multi-cloud, and fast-hiring
startups. `[VERIFIED: frigga.cloud product page]`

This matters architecturally: it is why access control is **group-based and
hierarchical** rather than per-user, and why `resources` are typed
(`dashboard` / `vm` / `database` / `service`) — the route-generation logic branches
on resource type. `[VERIFIED: Requirement Doc.pdf p.37]`

---

## 2. Critical context: this repository has three generations

Understanding this is a **prerequisite** for reading the codebase without being
misled. Three different design documents sit in the repository root, and **two of
them describe a system that does not exist here.**

| Generation | Artefact | Stack described | Relationship to the code |
|---|---|---|---|
| **1. Original spec** | `Requirement Doc.pdf` (68 pp.) | **Python / Flask**, SQLAlchemy, Alembic, `src/` layout | The authoritative statement of *intent*. Its code samples are Flask and do not apply. |
| **2. Restated spec** | `dvarpala-context.md` | **Python / Flask**, SQLAlchemy, `src/` layout | A condensed restatement of generation 1. **Describes a stack the repo does not use.** |
| **3. Actual build** | `dvarpala-go-context.md` | **Go**, Gin, GORM, Viper, Cobra, `internal/` layout | Matches the real repository layout. This is the design the code follows. |

Evidence:
- `dvarpala-context.md:12` states `**Backend**: Python 3.9+ with Flask`; `:121-141`
  prescribes a `src/` tree with SQLAlchemy models. **No `src/` directory exists** and
  the only `.py` file in the repository is an empty placeholder. `[VERIFIED]`
- `dvarpala-go-context.md:8-27` states Go 1.21+, Gin, GORM, Viper, Cobra — which
  matches `go.mod:1-8` (`module dvarpala`, `go 1.23.0`, gin, cobra, viper). `[VERIFIED: go.mod:1-8]`

> **Practical rule for a new developer:** treat `Requirement Doc.pdf` as the
> product requirements, `dvarpala-go-context.md` as the architectural intent, and
> **ignore `dvarpala-context.md` entirely** — it is a stale Flask-era artefact that
> will actively mislead you. `[INFERRED — from the stack mismatch above]`

### 2.1 Why the repository looks bigger than it is

The requirement doc contains, on pages 16–22, a **complete prescribed directory
tree**, and on page 61 the literal shell commands to create it:

```bash
mkdir -p src/{core,models,auth/oauth,vpn,access_control,api/v1,web/{auth,admin,user}...}
find src -type d -exec touch {}/__init__.py \;
```
`[VERIFIED: Requirement Doc.pdf p.61]`

Someone performed the Go equivalent of exactly this: **the whole tree was created
up front from the specification, and then only a fraction of it was filled in.**
This single fact explains the shape of the repository.

**Measured at `cff0373`:**

| Measure | Count |
|---|---|
| Tracked files | 337 |
| **Files that are completely empty (0 bytes)** | **113 (34%)** |
| Go files under `internal/`, `cmd/`, `pkg/` | 135 |
| …of which are bare 2-line `package x` stubs | **~100** |
| Total Go LOC in `internal/` | **1,608** |
| Total Go LOC in `scripts/installation/` (separate module) | **~4,600** |

Entire subsystems exist as directory structure and nothing else `[VERIFIED]`:
- **All** of `deployments/terraform/*.tf` (8 files) — empty
- **All** of `deployments/kubernetes/*` (12 files) — empty
- **All** of `api/openapi/*` (5 files), `api/proto/`, `api/graphql/` — empty
- **All** of `configs/openvpn/*` templates and auth-scripts (5 files) — empty
- Both root `docker-compose.yml` and `docker-compose.prod.yml` — empty
- 16 of 21 files under `docs/` — empty
- **All 12 files under `test/`** — 2-line stubs; all 3 fixtures empty (§7.4)
- `internal/services/` (8 files), `internal/utils/` (7), `internal/auth/` (most),
  `internal/vpn/` (all), `internal/web/` handlers (all), `internal/api/v1/`
  handlers (all) — **all bare `package` declarations**

> **The directory tree describes an *aspiration*, not an implementation.** A newcomer navigating by folder names will conclude the system is
> far more built than it is. §8 gives the honest map of what actually runs.

### 2.2 Where the real code is

Counter-intuitively, the substantial working code is **not** in `internal/`. It is in
`scripts/installation/` — a **separate Go module** (its own `go.mod`) containing a
multi-cloud installer, at roughly **three times the line count** of the entire
`internal/` tree. `[VERIFIED: scripts/installation/go.mod exists; LOC counts above]`

| Component | LOC | What it is |
|---|---|---|
| `scripts/installation/installer/cloud-installer.go` | 1,486 | Installer orchestration |
| `scripts/installation/providers/aws.go` | 812 | AWS provisioning |
| `scripts/installation/providers/azure.go` | 740 | Azure provisioning |
| `scripts/installation/providers/gcp.go` | 724 | GCP provisioning |
| `scripts/installation/installer/api/resource-management-api.go` | 576 | Control plane for selective blocking (§9.10) — **does not compile** |
| `scripts/installation/installer/databaseInstaller.go` | 432 | Database bootstrap |
| `scripts/installation/installer/cloud-setup-server.sh` | 1,436 | Server-side setup — **dead code, never invoked (§9.1)** |
| `scripts/installation/installer/` selective-blocking subsystem | ~2,410 | A second access-gating design with its own schema, control-plane API, VPC discovery and test harness — **also never invoked (§9.10)**. Includes the `resource-management-api.go` above. |

Plus ~32 shell scripts that perform the actual OpenVPN, nginx, iptables and
captive-portal work.

> **Caution when sizing this repository by line count.** Of the components above,
> `cloud-setup-server.sh` (1,436), the selective-blocking subsystem (~2,410) and
> `databaseInstaller.go` (432) — over 4,200 lines — are never executed. Volume here
> is a poor proxy for capability.

`[INFERRED]` **Development effort went into "how do we stand this system up on a
cloud VM", not into "the Dvarpala application server".** The product, as it exists
today, is closer to *an installer that provisions an OAuth-gated OpenVPN server*
than to *a Go web application*. §6.5 and §7.5 strengthen this reading considerably:
the entire documented developer workflow for the Go app is broken as committed. §9 and §10 confirm it: the installer is the working artefact, and what
it deploys is not the Dvarpala application.

---

## 3. Prerequisites — what to understand before reading the code

A checklist of concepts the codebase assumes. If a term here is unfamiliar, learn
it before reading the corresponding section, or that code will be opaque.

### 3.1 Essential — the VPN and captive portal (§9), the core of the product

| Concept | Why it matters here |
|---|---|
| **OpenVPN server/client model** | The entire data plane. Know `server.conf`, `.ovpn` client profiles, and that a client gets a virtual IP on a `tun` interface. |
| **`client-connect` / `client-disconnect` scripts** | OpenVPN's hook mechanism — how Dvarpala runs custom logic at connect/disconnect time. `[VERIFIED: dvarpala-context.md:292-293]` |
| **`auth-user-pass-verify`** | The OpenVPN hook that delegates username/password checking to an external script. This is where the `temp_user` credential is validated. |
| **CCD (client-config-dir)** | Per-client OpenVPN config. The mechanism for giving one client a different IP/routes than another — i.e. how "promotion" to full access is implemented. |
| **easy-rsa / PKI** | Certificate generation for the server and clients. `[VERIFIED: Requirement Doc.pdf p.57]` |
| **iptables — chains, DROP/ACCEPT, NAT/MASQUERADE** | **The actual enforcement mechanism.** The walled garden is firewall rules, not OpenVPN. Understand `INPUT`/`FORWARD`/`POSTROUTING`. |
| **Captive portal pattern** | The general technique (as in hotel wifi): allow DNS + one host, block everything else, redirect HTTP. |
| **OAuth 2.0 authorisation-code flow** | `state` parameter, redirect URI, code-for-token exchange, userinfo endpoint. |

### 3.2 Essential — the Go application and its data model (§5–§8)

| Concept | Why |
|---|---|
| **Go project layout** (`cmd/`, `internal/`, `pkg/`) | `internal/` is compiler-enforced private. Explains the structure. |
| **GORM** | The ORM. Struct tags define schema; `AutoMigrate` creates tables. |
| **Gin** | HTTP router — route groups and middleware. |
| **Viper** | Config: YAML files + env var overrides. Explains the config precedence rules — and §6.3, where they go wrong. |
| **Cobra** | The CLI framework behind `dvarpala-cli`. |
| **Redis as a session store with TTL** | Sessions are the bridge between the web app and the VPN auth script. `SETEX auth:<client_ip>` is the key pattern. `[VERIFIED: Requirement Doc.pdf p.13]` |

### 3.3 Needed for the cloud installer (§10)

Cloud provider SDK concepts for **AWS, Azure and GCP** — VPCs/subnets, security
groups/firewall rules, VM instance creation, SSH key injection, object storage
buckets, and service-account/credential handling. The installer implements the same
provisioning flow three times, once per cloud.


---

## 4. Development timeline and current state

`[VERIFIED: git log]`

| Branch | Tip | Last commit | Position |
|---|---|---|---|
| `main` | `cff0373` | **2025-06-19** | The baseline for this document. |
| `origin/CustomInstallation` | `bcf2755` | 2025-06-19 | Fully merged into `main`; nothing unique. |
| `origin/installer/restructure` | `2e6d5f6` | **2025-06-23** | **16 commits ahead, never merged.** |

All work is by one person, committing under two author strings (Veer Shrivastav /
Veer Shubhranshu Shrivastav). The most recent commit on any
branch is **23 June 2025** — the project has been **dormant for approximately 14
months**. `[VERIFIED: git log --all]`

The unmerged `installer/restructure` branch is a substantial refactor
(**+4,110 / −6,743 lines**) that moves `scripts/installation/` → `installation/`,
replaces the three per-cloud provider files with a `base_provider.go` factory
(666 lines), and extracts SQL into numbered migrations and seeds.
`[VERIFIED: git diff --stat main origin/installer/restructure]`

`[INFERRED]` Reading the commit messages on `main` in order — "Debugging issue of
installation part 1", "Fixing VM creation for GCP", "Fixing ssh to VM", "Fixing
nginx installation last step", "Installation complete" — the final phase of active
work was **getting the cloud installation to succeed end-to-end**, followed
immediately by a **code-quality refactor** ("Code review done till VM creation",
"Major refactoring", "standardizing the codebase") that was never finished or
merged. §12 assesses that branch in detail, including whether it is worth adopting.

---

## 5. The data model

This is the most complete part of the Go application, and the best place to start
reading code. **The data model is the domain model**: it tells you what the system
believes the world is made of.

### 5.1 How the schema is created — GORM `AutoMigrate`, not SQL

`internal/database/migrations/` contains six numbered `.sql` files
(`001_initial_schema.sql` … `006_add_audit_logs.sql`). **All six are 0 bytes.**
`[VERIFIED]`

The schema is instead created from Go struct definitions by GORM's `AutoMigrate`,
which reflects over the structs and issues the DDL:

```go
func AutoMigrate(db *gorm.DB) error {
    return db.AutoMigrate(
        &models.User{}, &models.Group{}, &models.Resource{}, &models.AuditLog{},
        &models.VPNSession{}, &models.VPNConfig{}, &models.NetworkRoute{},
        &models.Session{}, &models.OAuthState{}, &models.OAuthProvider{},
        &models.IPWhitelist{},
        &models.UserGroup{}, &models.GroupPermission{}, &models.GroupNetworkRoute{},
    )
}
```
`[VERIFIED: internal/database/migrate.go:10-36]`

**Consequence for a new developer:** to change the schema you edit a Go struct, not
a `.sql` file. There is no migration history and no rollback — `AutoMigrate` only
adds columns/indexes, it never drops or alters them destructively. The only
teardown is `DropAllTables()`, which drops everything in reverse dependency order.
`[VERIFIED: internal/database/migrate.go:39-56]`

> Note: the unmerged `installer/restructure` branch **reintroduces real SQL
> migrations** (`001_create_users_table.sql` etc., 34–51 lines each) and seed files.
> That is one of the substantive things that branch changes. `[VERIFIED: git diff --stat]`

### 5.2 Entity relationship map

14 tables from 15 model structs (`GroupNetworkRoute` is declared inside
`network_route.go`). `[VERIFIED: internal/database/models/]`

```
                          ┌──────────────┐
                          │    users     │
                          └──────┬───────┘
        ┌────────────┬───────────┼────────────┬──────────────┐
        │            │           │            │              │
   user_groups  vpn_sessions vpn_configs  sessions       audit_logs
   (M:N)        (1:N)        (1:N)        (1:N web)      (1:N)
        │
        ▼
   ┌──────────┐  parent_id ──┐
   │  groups  │◄─────────────┘  (self-referencing hierarchy)
   └────┬─────┘
        │
        ├── group_permissions ───► resources     (M:N + permission_type)
        │
        └── group_network_routes ► network_routes (M:N)

   Standalone:  oauth_states (CSRF tokens) · oauth_providers (provider config)
                ip_whitelists (→ optional user / group)
```

### 5.3 What each table is for

**Identity and access control**

| Table | Purpose | Key detail |
|---|---|---|
| `users` | Person with VPN access | `email` unique; `status` ∈ `active`/`inactive`/`suspended`. **`status` is the kill switch** — the DB check in the auth flow tests it. `[VERIFIED: models/user.go:11-15,24,33]` |
| `groups` | Collection of users | `parent_id` self-reference gives hierarchy. `[VERIFIED: models/group.go:21,34-37]` |
| `user_groups` | M:N users ↔ groups | Composite PK, `ON DELETE CASCADE`. `[VERIFIED: models/user_group.go:11-14,24-27]` |
| `resources` | A protected thing | Typed `dashboard`/`vm`/`database`/`service`. Carries **both** `url` (for dashboards/services) and `ip_address`+`port` (for VMs/DBs). `[VERIFIED: models/resource.go:11-16,30-36]` |
| `group_permissions` | M:N groups ↔ resources | **PK is `(group_id, resource_id, permission_type)`** — a group can hold several permission types on one resource. Types: `read`/`write`/`admin`/`ssh`/`full`. `[VERIFIED: models/group_permission.go:11-17]` |

**VPN**

| Table | Purpose | Key detail |
|---|---|---|
| `vpn_sessions` | One VPN connection | `client_ip`, `connected_at`/`disconnected_at`, `bytes_in`/`bytes_out`. Status ∈ `active`/`disconnected`/`expired`. `[VERIFIED: models/vpn_session.go:11-15,26-44]` |
| `vpn_configs` | A user's `.ovpn` profile | **Stores `client_cert`, `client_key`, `ca_cert` and the full `config_data` as text columns in Postgres.** Has `expires_at` (temporary certs) and status `active`/`inactive`/`revoked`. `[VERIFIED: models/vpn_config.go:29-44]` |
| `network_routes` | A pushable route | `destination` CIDR, `gateway`, `priority`, and `route_type` ∈ `captive_portal`/`full_access`/`restricted`. `[VERIFIED: models/network_route.go:11-15,26-35]` |
| `group_network_routes` | M:N groups ↔ routes | How "routes are pushed per group" is meant to be driven. `[VERIFIED: models/network_route.go:63-79]` |

> **`vpn_configs` is where the two-step model touches the database.** The
> `expires_at` + `status='revoked'` fields are the schema-level expression of
> "temporary certificate, revoked on disconnect" from §1.2. **No code ever writes
> them.** `models.VPNConfig{}` appears **only** in `migrate.go` (table creation) —
> nothing else in the repository references it, not even the dev seed script.
> `[VERIFIED: grep]` The schema anticipates a lifecycle nothing implements.

**Authentication and security**

| Table | Purpose | Key detail |
|---|---|---|
| `sessions` | **Web** login session (distinct from VPN session) | `session_id` unique, `expires_at`, `last_used_at` for idle timeout. `[VERIFIED: models/session.go:14,26-29]` |
| `oauth_states` | OAuth CSRF `state` tokens | Short-lived, single-use via `used bool`, bound to `user_ip` + `user_agent`. Good practice. `[VERIFIED: models/oauth_state.go:14-29]` |
| `oauth_providers` | OAuth provider config **in the DB** | Client ID/secret, auth/token/userinfo URLs, scopes, enable flag, display order. `[VERIFIED: models/oauth_provider.go:13-49]` |
| `ip_whitelists` | IP-based restriction | Scoped `user`/`group`/`global`, with CIDR and `expires_at`. `[VERIFIED: models/ip_whitelist.go:11-44]` |
| `audit_logs` | Immutable action trail | `action`, `resource_type`/`resource_id`, `ip_address`, `user_agent`, and `details` as **`jsonb`**. `user_id` nullable for system actions. `[VERIFIED: models/audit_log.go:14-36]` |

`models/oauth_provider.go:68-101` also ships a `GetDefaultProviderConfig()` helper
with the real endpoint URLs for Google, Microsoft, GitHub and GitLab pre-filled —
genuinely useful, and the only place all four providers are fully specified.
`[VERIFIED]`

### 5.4 Defects and inconsistencies in the model layer

These are the things worth raising with the original developer.

**(a) `BaseModel` is dead code.** `models/base.go` defines a `BaseModel` with
`ID`/`CreatedAt`/`UpdatedAt`/`DeletedAt` for embedding. **Exactly one of the 15
models embeds it** (`OAuthProvider`). The other 14 hand-copy the same four fields.
`[VERIFIED: base.go:10-22 vs. all other model files]`
The `TableNamer` interface in `base.go:25-27` is likewise never referenced.

**(b) `Permission` is deprecated but still wired in — and has no table.**
`models/permission.go:20` says *"This model is deprecated in favor of
GroupPermission junction table"*. But:
- `Group.Permissions []Permission` with `many2many:group_permissions` `[VERIFIED: models/group.go:44]`
- `Resource.Permissions []Permission` with `many2many:group_permissions` `[VERIFIED: models/resource.go:52]`
- **`&models.Permission{}` is absent from `AutoMigrate`** `[VERIFIED: migrate.go:10-36]`

`[VERIFIED BY EXECUTION — see §7.3]` GORM's own schema parser was run against these
structs. **Three mutually incompatible definitions of the table
`group_permissions` exist simultaneously:**

| Declared by | Resolves to table | With columns |
|---|---|---|
| `Group.Permissions` many2many | `group_permissions` | `group_id`, **`permission_id`** |
| `Resource.Permissions` many2many | `group_permissions` | **`resource_id`**, **`permission_id`** |
| `GroupPermission` model | `group_permissions` | `group_id`, `resource_id`, **`permission_type`** |

Both `many2many` relations resolve to a `permission_id` foreign key pointing at
table `permissions` — **which `AutoMigrate` never creates**, because
`&models.Permission{}` is absent from the migration list.

✅ **Corrected by running it.** An earlier draft of this document predicted that
`AutoMigrate` would produce a malformed table accreting all four columns with a
foreign key to the non-existent `permissions` table. **That did not happen.**
Against PostgreSQL 15.19, `AutoMigrate` produced exactly what `GroupPermission`
declares `[VERIFIED BY EXECUTION, Aug 2026]`:

```
                      Table "public.group_permissions"
     Column      |           Type           | Nullable
-----------------+--------------------------+----------
 group_id        | bigint                   | not null
 resource_id     | bigint                   | not null
 permission_type | character varying(20)    | not null
 created_at      | timestamp with time zone |
 updated_at      | timestamp with time zone |
Indexes:
    PRIMARY KEY, btree (group_id, resource_id, permission_type)
Foreign-key constraints:
    -> groups(id) ON DELETE CASCADE ; -> resources(id) ON DELETE CASCADE
```

No `permission_id` column, no broken foreign key, and 14 tables created cleanly.
GORM evidently resolves the collision in favour of the explicitly-migrated model.

**So the deprecated `Permission` model is dead weight, not a defect.** Deleting
`models/permission.go` and the two `Permissions []Permission` fields is still worth
doing for clarity, but it is housekeeping rather than a fix. The schema is correct
as it stands.

**(c) `database-readme.md` does not match the code.** It enumerates 14 tables
including a legacy `permissions` table, and **omits `oauth_providers`**
`[VERIFIED: database-readme.md:19-39]`. The real `AutoMigrate` set is the inverse:
`oauth_providers` **is** created, `permissions` is **not**.

**(d) Seed data does not match the README.**

| | README claims `[VERIFIED: README.md:196-211]` | Code actually creates `[VERIFIED: internal/database/init.go:33-105]` |
|---|---|---|
| Groups | `system_admins`, `vpn_users`, `guests` | `system_admins`, `vpn_users` — **no `guests`** |
| Resources | System Administration, User Dashboard, **VPN Server** | "System Dashboard" (`/admin`), "User Portal" (`/portal`) — **no VPN Server** |
| Routes | captive portal DNS, internal `10.0.0.0/8`, internet `0.0.0.0/0` | **one** route only: "Captive Portal" → `8.8.8.8/32` |

The single seeded route being `8.8.8.8/32` (Google DNS) is consistent with the
captive-portal design — DNS must work inside the walled garden — but the internal
and internet routes the README promises are simply not created.

✅ **Confirmed by running it** `[VERIFIED BY EXECUTION, Aug 2026]`. After a first
successful start against PostgreSQL 15.19, the database contains exactly:

```
groups:          system_admins, vpn_users
resources:       System Dashboard, User Portal
network_routes:  Captive Portal -> 8.8.8.8/32
```

Two, two and one — not the three, three and three the README advertises.

---

## 6. Configuration

### 6.1 Shape

One `Config` struct with eight sections — Server, Database, Redis, Auth, OAuth,
OpenVPN, Security, Logging. `[VERIFIED: internal/config/config.go:11-92]`
The VPN-relevant ones:

```go
type AuthConfig struct {
    SessionDuration      int      // seconds; default 28800 = 8h
    CaptivePortalTimeout int      // seconds; default 1800 = 30m
    JWTSecret            string
    AllowedDomains       []string // email domain allow-list
}
type OpenVPNNetworks struct {
    CaptivePortal string   // CIDR
    FullAccess    string   // CIDR
}
type SecurityConfig struct {
    FailedLoginThreshold int  // default 5
    IPBlockDuration      int  // seconds; default 1800
    BcryptCost           int
}
```
`[VERIFIED: config.go:47-86]`

`AllowedDomains` is the email-domain gate from §1.2 step 4; `FailedLoginThreshold`
and `IPBlockDuration` are the progressive-blocking policy. The config surface
matches the design intent closely — this part is well built.

The three sibling files `internal/config/auth.go`, `database.go`, `redis.go` are
comment-only stubs; `oauth.go` contains one real type, `GitLabProvider` (GitLab
needs an extra `URL` field for self-hosted instances). `[VERIFIED]`

### 6.2 How values are resolved

`config.Load(path)` does: bind ~30 env vars → read the YAML file → `Unmarshal` →
post-process two special cases. `[VERIFIED: config.go:94-114]`

Precedence is **environment variable > YAML file**, because `viper.BindEnv` takes
priority over file values.

Two things need special handling after unmarshalling `[VERIFIED: config.go:178-197]`:
- `AUTH_ALLOWED_DOMAINS` — comma-separated string split into `[]string`.
- `SERVER_READ_TIMEOUT` / `SERVER_WRITE_TIMEOUT` — parsed into `time.Duration`.

### 6.3 ⚠️ The YAML file uses a syntax Viper does not support

`configs/environment.yaml` is written entirely in `${VAR:default}` form:

```yaml
server:
  port: ${SERVER_PORT:8080}
database:
  host: ${DB_HOST:localhost}
  port: ${DB_PORT:5432}
```
`[VERIFIED: configs/environment.yaml:5-19]`

**Viper does not perform environment-variable interpolation in config files.**
There is no `${}` expansion step; the value is read as the *literal string*
`"${SERVER_PORT:8080}"`.

`[VERIFIED BY EXECUTION — see §7.2]` This was tested directly against Viper v1.16.0
using this repository's actual `environment.yaml`:

```
viper.Get("server.port")   = "${SERVER_PORT:8080}"      ← literal string, not 8080
viper.Get("database.host") = "${DB_HOST:localhost}"

With NO env vars exported, Unmarshal fails with 14 errors (every int and duration);
the first four:
  * cannot parse 'server.port' as int:   parsing "${SERVER_PORT:8080}": invalid syntax
  * cannot parse 'database.port' as int: parsing "${DB_PORT:5432}":     invalid syntax
  * error decoding 'server.read_timeout':  invalid duration "${SERVER_READ_TIMEOUT:30s}"
  * error decoding 'server.write_timeout': invalid duration "${SERVER_WRITE_TIMEOUT:30s}"

With env vars exported, it succeeds — and the YAML values are never used.
```

So **`config.Load()` returns an error and the server cannot start** unless the
environment supplies every numeric and duration setting. The YAML file's apparent
defaults are inert in all cases.

There is a subtler hazard: **only the typed fields fail loudly.** `string` fields
have no parse step, so an unset `DB_HOST` yields the literal value
`"${DB_HOST:localhost}"` with **no error at all** — it would surface later as a
baffling DNS or connection failure. The same applies to `DB_NAME`, `DB_USER`,
`DB_PASSWORD`, `AUTH_JWT_SECRET` and every OAuth client ID and secret.

The intended fix is one of: (i) run the file through `envsubst` before Viper reads
it, (ii) drop the placeholders and put real defaults in the YAML with
`viper.SetDefault()`, or (iii) delete the YAML and rely purely on env vars.

### 6.4 `.env` is never loaded by the application

`.env.example` instructs *"Copy this file to `.env` and update the values"*
`[VERIFIED: .env.example:2]`, and `.gitignore` excludes `.env` `[VERIFIED: .gitignore:26-29]`.

But **no Go code reads a `.env` file.** There is no `godotenv` (or equivalent)
dependency in `go.mod`, and no reference to `.env` anywhere in the Go sources.
`[VERIFIED: grep across *.go; go.mod:1-70]`

So creating a `.env` file has **no effect** unless something external exports it
into the process environment (a shell `set -a; source .env`, a systemd
`EnvironmentFile=`, or Docker `env_file:`). **Nothing does** — the installer passes
the `.env` file to `--config` instead, which fails differently and silently (§8.7).
*(Resolved — OPEN-6.)*

Combined with §6.3, this is the practical trap: the documented setup path
(`cp .env.example .env`, edit, `make dev`) **cannot work**, because nothing loads
`.env` and the YAML defaults are inert.

### 6.5 ⚠️ The Makefile references files that do not exist

`CONFIG_FILE = configs/environments/development.yaml` `[VERIFIED: Makefile:6]`

**That path does not exist.** There is no `configs/environments/` directory; the
real file is `configs/environment.yaml` (singular, no subdirectory).
`[VERIFIED: ls configs/]`

Every target that guards on it — `install`, `install-fresh`, `install-dev` — will
print `❌ Configuration file not found` and exit 1. `dev`, `create-test-users` and
`seed-data` pass the non-existent path straight to the binary. `[VERIFIED: Makefile:41-77,108-117]`

`README.md:225` repeats the same wrong path in its configuration instructions.
`[VERIFIED]`

Independently, `make build` cannot succeed either:

```make
go build -o $(BINARY_DIR)/install ./scripts/installation/install.go
```
`[VERIFIED: Makefile:23]` — **`scripts/installation/install.go` does not exist**
(the only top-level Go file there is `launcher.go`). `[VERIFIED: ls scripts/installation/*.go]`
And because `scripts/installation/` is a *separate Go module*, building it from the
root module would fail regardless.

`[INFERRED]` **Since `install`, `install-fresh`, `install-dev` and `dev` all depend
on `build`, the entire documented developer workflow in the README is broken as
committed.** This strongly suggests the Go application has not been run from this
repository in its current state, and that all real deployment happens through the
installer path (§10).

### 6.6 Every documented network range is stale

| Source | Captive portal | Full access |
|---|---|---|
| `Requirement Doc.pdf` p.7-8 | `10.8.0.x` | `10.8.1.x` |
| `README.md:185, 200-201` | `192.168.100.0/24` | `10.8.0.0/24` |
| `.env.example:56-57` + `configs/environment.yaml:53-55` | `192.168.100.0/24` | `10.0.0.0/8` |
| **What the live installer actually deploys (§9.2)** | **`172.30.100.0/24`** | **`172.30.8.0/21`** |

`[VERIFIED]` Three documented schemes, no two alike — and none matches what is
deployed. Any configuration written from these files describes a system that no
longer exists.

### 6.7 OpenVPN templates are empty

`configs/openvpn/` contains `server.conf.template`, `client.ovpn.template`, and
`auth-scripts/{dvarpala-auth.py, client-connect.sh, client-disconnect.sh}` —
**all five are 0 bytes.** `[VERIFIED]`

The OpenVPN configuration and hook scripts described throughout the documentation
are therefore **not** in `configs/`. They are generated by the installer — §9.2
locates them as heredoc strings inside the three provider Go files.

---

## 7. Build, vet and test status — measured

Run with **Go 1.26.5** at commit `cff0373` (`go.mod` declares `go 1.23.0`,
`toolchain go1.24.4`). Dependencies resolved cleanly: `go mod download` exit 0.

### 7.1 What compiles

| Target | Result |
|---|---|
| `./cmd/dvarpala-server` | ✅ **OK** |
| `./cmd/dvarpala-cli` | ✅ **OK** |
| `./cmd/dvarpala-worker` | ✅ **OK** |
| `./cmd/openvpn-auth` | ✅ **OK** |
| `./internal/...` | ✅ **OK** (all packages) |
| `./pkg/...` | ✅ **OK** |
| **`go build ./...`** | ❌ **FAILS** |

`go build ./...` fails in exactly three packages, all for the same trivial reason —
two files in one directory each declaring `func main()`:

```
scripts/monitoring/vpn-status.go:4:6: main redeclared in this block
    scripts/monitoring/health-check.go:4:6: other declaration of main
scripts/migration/seed.go:4:6:      main redeclared (vs migrate.go)
tools/generators/model.go:4:6:      main redeclared (vs api.go)
```

Every one of those six files is a 5-line stub whose body is `// TODO: Implement …`.
`[VERIFIED]` `go vet ./...` fails on the same three packages and reports nothing else.

**The Go application itself is structurally sound.** Only the module-wide command
was broken; each real package always compiled on its own. The breakage was confined
to placeholder tooling and was a ~2-minute fix — deleting the six stubs, since
nothing imports, builds or references them.

✅ **Since confirmed:** with those six files removed, `go build ./...`,
`go vet ./...` and `go test ./...` all pass, and the server starts, connects to
PostgreSQL and Redis, creates its 14 tables and serves its routes.
`[VERIFIED BY EXECUTION, Aug 2026]`

### 7.2 Viper `${VAR:default}` — tested

Confirmed exactly as described in §6.3. Test harness: an isolated module using
Viper v1.16.0 and a verbatim copy of `configs/environment.yaml`. Result: literal
placeholder strings; `Unmarshal` fails on all `int` and `time.Duration` fields when
env vars are absent; succeeds and ignores the YAML entirely when they are present.

### 7.3 `group_permissions` collision — tested

Confirmed as described in §5.4(b). Test harness: GORM v1.30.0 `schema.Parse` run
against verbatim copies of all 15 model files. Three different column sets resolve
to the same table name; both `many2many` relations reference the never-migrated
`permissions` table.

### 7.4 Tests

**The Go test suite is empty.** All 12 files under `test/` are 2-line package stubs,
and all three JSON fixtures are empty. `[VERIFIED]`

```
test/unit/{auth,config,group_service,user_service,vpn_service}_test.go   2 lines each
test/integration/{api,auth_flow,database,vpn_flow}_test.go              2 lines each
test/e2e/{admin_flow,user_journey}_test.go                              2 lines each
test/mocks/{database,oauth,redis}.go                                    2 lines each
test/fixtures/{users,groups,resources}.json                             0 bytes
```

`go test ./...` reports `FAIL` overall — not from failing assertions, but because
the three packages in §7.1 do not build. There is **zero Go test coverage**.

One real test suite does exist, outside the Go tooling:
`installer/test-selective-blocking.sh` (487 lines) is an integration harness for the
selective-blocking subsystem (§9.10), with ~27 assertions covering the API, the
database and the firewall rules. `[VERIFIED]` It is the only executable test in the
repository — and it targets the one subsystem that is never installed.

### 7.5 Summary of the developer workflow, as committed

| README / Makefile command | Actual result |
|---|---|
| `make deps` | ✅ works |
| `make build` | ❌ fails — `scripts/installation/install.go` does not exist (§6.5) |
| `make install` / `install-fresh` / `install-dev` | ❌ fails — depends on `build`; config path wrong |
| `make dev` | ❌ fails — depends on `build` |
| `make test` | ❌ fails — 3 packages don't build; no Go tests exist anyway |
| `go build ./cmd/...` | ✅ **works** — this is the way in |

---

## 8. The Go control plane — what actually runs

This section maps what actually executes. The finding is structural, and was not
visible from the repository layout alone:

> **There are three separate implementations of "dvarpala-server", and the one in
> `cmd/` is not the one that gets deployed.**

### 8.1 The four binaries in `cmd/`

| Binary | Lines | State |
|---|---|---|
| `dvarpala-server` | 59 | **Real skeleton** — boots, wires everything, serves stub routes |
| `dvarpala-cli` | 55 | **Shell only** — Cobra scaffold, 4 subcommands, none does anything |
| `openvpn-auth` | 21 | **Real but half** — see §8.5 |
| `dvarpala-worker` | 6 | **Empty** — `func main() { // TODO }` |

`dvarpala-cli` registers `user`, `group`, `vpn` and `admin` commands, but each is a
bare `&cobra.Command{Use:..., Short:...}` with **no `Run` function and no
subcommands** `[VERIFIED: cmd/dvarpala-cli/main.go:29-52]`. Running
`dvarpala-cli user create` prints help and exits. Every CLI capability the README
advertises is absent.

### 8.2 Server startup — this part is genuinely well built

`[VERIFIED: cmd/dvarpala-server/main.go:18-57; internal/app/app.go:22-55]`

```
flag -config  →  config.Load()
              →  database.NewConnection()      (Postgres via GORM)
              →  database.InitializeDatabase() (AutoMigrate + seed, §5)
              →  redis.NewClient()             (ping-tested)
              →  gin.New() + Logger + Recovery
              →  setupRoutes()
              →  http.Server with graceful shutdown on SIGINT/SIGTERM (30s timeout)
```

Proper error wrapping, connection-pool tuning (`MaxOpenConns`, `MaxIdleConns`,
1-hour `ConnMaxLifetime`), signal handling. If the rest of the application were
finished, this would be a sound foundation.

### 8.3 Every route is a placeholder

The complete API surface is six endpoints — `POST /auth/validate`, `GET`/`POST
/users`, `GET`/`POST /groups`, `GET /vpn/status` — each returning a hardcoded
string, e.g. `{"message": "list users endpoint"}`. The `db`, `redis` and `cfg`
parameters are passed into `SetupRoutes` and never referenced: no query, no auth, no
validation. `[VERIFIED: internal/api/v1/router.go:11-49]`

The three web routes (`/`, `/login`, `/dashboard`) render `index.html`,
`login.html` and `dashboard.html`. **None of those files exists**, and
`LoadHTMLGlob()` is never called anywhere, so Gin has no templates registered —
either fault alone would panic on the first request.
`[VERIFIED: internal/web/router.go:11-33; ls web/templates/; grep across *.go]`

### 8.4 The admin UI and user portal do not exist

Every template the design calls for is 0 bytes `[VERIFIED]`: all 12 admin templates
(users/groups/resources/permissions/audit), all 3 user templates, all 3 layouts,
all 5 components, and **all 5 CSS files**.

Every template the design calls for is 0 bytes, including the four under
`web/templates/auth/`. The only real front-end assets are the captive portal —
`captive-portal.html` (219), `auth-success.html` (376), `auth-error.html` (394),
`captive-portal.js` (314), `portal-config.json` (94).

The Go application would serve the **static** files — `web/router.go:13` registers
`r.Static("/static", "./web/static")`, so `captive-portal.js` is reachable — but it
registers **no HTML templates at all**, so the portal pages themselves are not
served by it. In practice they are deployed to the VPN server and served by a
different process entirely (§8.6).

### 8.5 `openvpn-auth` implements step 1 only

`[VERIFIED: cmd/openvpn-auth/main.go:7-22]`

```go
username := os.Getenv("username")
password := os.Getenv("password")
_ = os.Getenv("untrusted_ip") // clientIP - not used in this basic version

if username == "temp_user" && password == "temp_portal_access" {
    os.Exit(0) // Success - allow captive portal access
} else {
    os.Exit(1)
}
```

This is the OpenVPN `auth-user-pass-verify` hook. It checks the shared password and
nothing else — **no Redis lookup, no session validation, no per-client state.** The
client IP is explicitly discarded. The two-step promotion described in §1.2 has no
implementation here; the code comments say "will be enhanced later".

### 8.6 ⚠️ Three different `dvarpala-server` implementations

| # | Where | Stack | State store | Deployed by |
|---|---|---|---|---|
| **1** | `cmd/dvarpala-server/main.go` (this repo) | Gin | Postgres + Redis | the **cloud** installer |
| **2** | heredoc in `scripts/provisioning/setup-server.sh:784-910` | Gin + JWT | **in-memory map** | nobody (superseded) |
| **3** | heredoc in `scripts/provisioning/setup-server.sh:1015-1305` | stdlib `net/http` | **in-memory map** | the **provisioning** installer |

`[VERIFIED]` Implementations 2 and 3 are **Go programs written inline inside a shell
script** and compiled on the target VM (`go build -o bin/dvarpala-server
template-main.go` at `setup-server.sh:1328`).

**Implementation 3 is the one that answers the captive portal's requests.** It
registers exactly the routes the portal JS calls — `/auth/google`,
`/auth/microsoft`, `/auth/github`, `/auth/gitlab`, `/auth/success`, `/auth/error`,
`/api/internal/check-auth/`, `/api/internal/clear-auth/`, `/api/internal/auth-status`
`[VERIFIED: setup-server.sh:1204-1303]`. That resolves the mystery in §8.3: the
portal was never written against the repo's Go app.

But it is explicitly a **demo**:
- Authentication state is `authenticatedUsers map[string]time.Time` guarded by a
  mutex — **in-memory, not Redis, not Postgres.** Lost on restart; unscalable beyond
  one process. `[VERIFIED: setup-server.sh:1031-1033]`
- `/auth/success` renders hardcoded `Username: "demo-user"`, `Provider: "Demo
  Provider"`. `[VERIFIED: setup-server.sh:1231-1239]`
- `/api/internal/auth-status` — the endpoint the portal polls — **always returns
  `{"authenticated": false}`** under the comment `// Demo: simulate authentication
  check`. `[VERIFIED: setup-server.sh:1286-1298]`
- Implementation 2's OAuth handler carries `// In production, implement proper OAuth
  redirect / For now, simulate OAuth process`. `[VERIFIED: setup-server.sh:~908]`

**So on the older provisioning path, OAuth is simulated end to end.** The portal
renders, the buttons work, and nothing is actually verified.

### 8.7 `cloud-setup-server.sh` — dead code, but instructive

`installer/cloud-setup-server.sh` (1,436 lines) is the most complete written
expression of the intended server: it builds all four repository binaries, installs
and configures PostgreSQL and Redis, and registers systemd units.

**Nothing invokes it.** `[VERIFIED: grep across all *.go and *.sh]` The only
mentions anywhere are in `scripts/installation/README.md`, which claims it runs
"automatically via user-data". No Go code references it. What actually gets deployed
is in §9.2.

> *An earlier draft treated this script as the live installation path. It is not.*

It is kept here for one reason: **whoever revives it will hit two bugs immediately**,
both reproduced by running the binary.

| Bug | Evidence | Fix |
|---|---|---|
| Line 877 calls `dvarpala-server --config … --migrate`; no such flag exists | `flag provided but not defined: -migrate` | add the flag, or drop the step |
| systemd passes `--config …/.env`, but Viper reads `.env` as flat keys (`DB_HOST`) that never match the nested `Config` struct (`database.host`) — producing an all-empty config **with no error** | `failed to connect to: user= password= database= sslmode=` / `lookup port=0`, then `log.Fatalf` → systemd restarts every 10s, forever | emit real YAML, or bind the flat keys |

The two halves were never tested together — the variable names could not match:

| Installer's `.env` writes | Code binds |
|---|---|
| `REDIS_HOST`, `REDIS_PORT` | `REDIS_ADDR` |
| `JWT_SECRET` | `AUTH_JWT_SECRET` |
| `CAPTIVE_PORTAL_NETWORK` | `OPENVPN_CAPTIVE_PORTAL_NETWORK` |
| `SERVER_HOST` | *(not bound at all)* |

`[VERIFIED: cloud-setup-server.sh:833-864 vs internal/config/config.go:117-175]`

### 8.8 Stub inventory

`[VERIFIED]` Of **135** Go files under `internal/`, `cmd/` and `pkg/`:

| | Count |
|---|---|
| Bare stubs (≤4 lines, `package x` + comment) | **106 (79%)** |
| Files containing real code | **29 (21%)** |

The 29 real files are almost entirely the 15 models, the database layer, config,
Redis connection, the two routers and the four `main.go`s. **Every business-logic
package — `services`, `auth`, `vpn`, `access`, `utils`, `web/admin`, `web/user`,
`api/v1/*` handlers, `api/middleware` — is 100% stubs.**

The Redis layer is connection code plus two generic helpers (`SetWithExpiry`,
`GetValue`) `[VERIFIED: internal/redis/redis.go, cache.go]`. `session.go` is a stub.
**The `auth:<client_ip>` session bridge that the whole two-step design depends on is
not implemented anywhere in Go.**

### 8.9 Answering "what runs today"

| Component | Provisioning path (older) | Cloud path (newer) |
|---|---|---|
| Captive portal UI | ✅ served (real templates) | ✅ files copied |
| Portal backend | ✅ demo server, simulated OAuth | ❌ crash-loops |
| OAuth verification | ❌ simulated | ❌ never starts |
| Postgres / Redis | ❌ not used by the demo server | ✅ installed, never reached |
| VPN + firewall | see §9 | see §9 |

`[INFERRED]` **Neither deployment path currently performs real OAuth
authentication.** The older one simulates it; the newer one cannot boot.
§9 examines the VPN layer beneath — and finds that the walled garden is not
deployed either: what demonstrably works is the tunnel, the PKI and the portal's
appearance.

---

## 9. The VPN and captive-portal data plane

This is the layer the product's value rests on. The finding is stark:

> **The captive portal exists as a design, written down in detail, in a script that
> is never executed. What the installer actually deploys is an ordinary
> full-tunnel VPN with no captive portal, no two-step authentication, and no
> Dvarpala application.**

### 9.1 Two data planes: one designed, one deployed

| | **Designed** | **Deployed** |
|---|---|---|
| Defined in | `installer/cloud-setup-server.sh` (1,436 lines) | `providers/{gcp,aws,azure}.go` — inline SSH step list |
| Invoked by | **nothing** `[VERIFIED]` | `InstallDvarpalaDirectly()` `[VERIFIED: gcp.go:310-400]` |
| Captive portal | yes — full mechanism | **no** |
| Dvarpala app | built + systemd units | **not installed at all** |

The VM's actual startup script is *minimal* — `apt-get install curl wget openssh-server`,
start SSH, drop a marker file. Nothing more. `[VERIFIED: gcp.go:262-294]` All real
work happens over SSH from the operator's machine.

### 9.2 What is actually deployed

`[VERIFIED: gcp.go:477-504; identical in aws.go:606, azure.go:510]`

```
port 1194
proto udp
dev tun
server 172.30.100.0 255.255.255.0
push "redirect-gateway def1 bypass-dhcp"     ← FULL internet, immediately
push "dhcp-option DNS 8.8.8.8"
tls-auth ta.key 0
cipher AES-256-GCM
```

Read that against §1.2. The deployed config:

- **pushes the default gateway to every client on connect** — full tunnel, full access, no gate
- has **no `auth-user-pass-verify`** — the OpenVPN auth hook is absent
- has **no `client-connect` / `client-disconnect`** — no per-client logic
- has **no `client-config-dir`** — no per-client routing
- has **no `script-security`** — hooks could not run even if configured

`[VERIFIED]` The strings `auth-user-pass-verify`, `client-connect` and
`client-config-dir` **do not appear in any of the three provider files.**

The live installation step list also contains **no `go build`, no database schema
initialisation, no systemd unit for `dvarpala-server`.** It installs nginx,
PostgreSQL, Redis, OpenVPN, Go, generates certificates, writes the config above,
starts OpenVPN, attempts the OAuth firewall step, configures an nginx status page,
and hands over `admin.ovpn`. **PostgreSQL and Redis are installed and then never
used.** `[VERIFIED: gcp.go:324-346]`

`[INFERRED]` The deployed artefact today is *a correctly configured, ordinary
OpenVPN server on cloud infrastructure*, with Frigga branding and IP conventions.
Everything that makes Dvarpala *Dvarpala* is either dead code or unbuilt.

### 9.3 The designed mechanism — worth understanding, since it is the blueprint

The dead script contains a coherent and mostly sound design.

**Server config** `[VERIFIED: cloud-setup-server.sh:595-631]` — the inverse of the
deployed one: the default-gateway push is deliberately commented out, only a host
route to the portal is pushed, and all three hooks are wired with `script-security 3`.

**`client-connect.sh`** `[VERIFIED: cloud-setup-server.sh:638-682]` — runs on every
connection and decides that client's routes:

```bash
if [ -f "$AUTH_STATUS_FILE-$common_name" ] && [ "$AUTH_STATUS" = "authenticated" ]; then
    echo 'push "route 172.30.8.0 255.255.248.0"' >  $1   # internal network
    echo 'push "route 0.0.0.0 128.0.0.0"'        >> $1   # split default route…
    echo 'push "route 128.0.0.0 128.0.0.0"'      >> $1   # …= full internet
else
    echo 'push "route 172.30.100.1 255.255.255.255"' > $1  # portal only
fi
```

**`client-disconnect.sh`** `[VERIFIED: cloud-setup-server.sh:686-711]` deletes the
status file, forcing re-authentication next time — exactly as §1.2 promises.

> **The client stays in `172.30.100.0/24` throughout.** "Full access network
> `172.30.8.0/21`" is a *destination* route the client gains, not a new address it
> receives. This corrects the requirement doc's description of the client moving
> from `10.8.0.x` to `10.8.1.x` — there is only one client pool. `[VERIFIED]`

**A fourth state mechanism.** Authentication state is a **flat file**:
`/var/lib/dvarpala/auth/dvarpala-auth-status-<common_name>` containing the literal
text `authenticated`, written by `/opt/dvarpala/bin/mark-user-authenticated`.
`[VERIFIED: cloud-setup-server.sh:925-960]`

The system now has four incompatible session stores across its variants — Redis
`auth:<client_ip>` (designed, never built), an in-memory Go map (demo server), and
this flat file (designed hooks). None of them interoperate.

**Promotion requires a reconnect.** The script's own comment concedes it:
`# Optional: Trigger OpenVPN to refresh user routing (requires reconnection for now)`.
Routes are only evaluated at connect time, so a user must disconnect and reconnect
after authenticating. Nothing automates this. `[VERIFIED: cloud-setup-server.sh:951]`

### 9.4 The firewall design — the strongest engineering in the repository

`scripts/installation/iptables-oauth-rules.sh` (179 lines) is genuinely good work
`[VERIFIED]`:

```
mangle PREROUTING : -s 172.30.100.0/24 -j MARK --set-mark 0x100   ← "unauthenticated"

filter FORWARD    : --mark 0x100 -j DVARPALA_AUTH    (portal :8080/:443, DNS)
                    --mark 0x100 -j DVARPALA_OAUTH   (OAuth providers)
                    --mark 0x100 -j LOG  [DVARPALA-BLOCKED]
                    --mark 0x100 -j DROP             ← everything else

nat PREROUTING    : --mark 0x100 --dport 80  -j DNAT --to 172.30.100.1:8080
                    --mark 0x100 --dport 443 -j DNAT --to 172.30.100.1:8080
```

Promotion is intended to be a single rule inserted ahead of the marking rule
`[VERIFIED: iptables-oauth-rules.sh:173]`:

```bash
# /usr/local/bin/dvarpala-auth-user <client-ip>
iptables -t mangle -I PREROUTING -s $CLIENT_IP -j MARK --set-mark 0x0
```

⚠️ **This rule does not do what it appears to do — see §9.5(e).**

The OAuth allow-list is thorough — Microsoft/Azure AD, Google, GitHub, GitLab, plus
CDNs and OCSP responders, by both hostname and IP range. Someone thought carefully
about what a login flow actually touches.

### 9.5 Five defects in that firewall design

**(a) HTTPS DNAT swallows the OAuth traffic it is meant to permit.** `nat
PREROUTING` runs *before* `filter FORWARD`. Every marked packet to port 443 —
including traffic to `accounts.google.com` — is rewritten to `172.30.100.1:8080`
**before** the `DVARPALA_OAUTH` ACCEPT rules are ever consulted. The comment says
"except for allowed domains", but no such exception is implemented.
`[INFERRED — high confidence from rule ordering; needs a live host to confirm]`
As written, **OAuth cannot complete**: every HTTPS request lands on the portal.

**(b) Hostname rules are resolved once, at insertion time.** `iptables -d
login.microsoftonline.com` stores whatever IPs DNS returned at that moment; it does
not match by name at runtime. For CDN-fronted OAuth endpoints these go stale almost
immediately. Only the CIDR rules do durable work.

**(c) Nothing ever calls `dvarpala-auth-user`.** `[VERIFIED: grep]` The only
references are its own definition and the closing help text. The firewall half of
promotion has no trigger.

**(d) The two enforcement mechanisms are keyed differently and never synchronised.**
Route promotion keys on **certificate common name** (a file); firewall promotion
keys on **client IP** (an iptables rule). Nothing keeps them consistent, and a
client could plausibly hold routes without firewall permission or the reverse.

**(e) ⚠️ The promotion rule is a no-op — `MARK` is a non-terminating target.**
This is the most serious defect in the design, because it disables the mechanism
even in the case where everything else works.

```bash
# rule inserted by dvarpala-auth-user, at the top of the chain
-t mangle -I PREROUTING -s <client-ip>       -j MARK --set-mark 0x0    # "authenticated"
# the original marking rule, still present below it
-t mangle -A PREROUTING -s 172.30.100.0/24   -j MARK --set-mark 0x100  # "unauthenticated"
```
`[VERIFIED: iptables-oauth-rules.sh:22, 173]`

Unlike `ACCEPT` or `DROP`, **`MARK` does not stop traversal of the chain.** After
the first rule sets the mark to `0x0`, the packet continues to the second rule —
whose source match still succeeds, because the client's address never leaves
`172.30.100.0/24` (§9.3) — and is **immediately re-marked `0x100`**. It then meets
the `FORWARD` chain still flagged as unauthenticated and is dropped.

`[INFERRED — from documented iptables semantics; would be settled by one live test]`
**Promotion via the firewall could never have worked**, independently of §9.5(c)
(nothing calls the script anyway). A correct version needs a terminating construct —
for example an early `RETURN` in a dedicated chain, or an `ipset` of authenticated
addresses that the marking rule excludes with `-m set ! --match-set`.

### 9.6 The one live firewall step is broken three ways

All three providers run `[VERIFIED: gcp.go:343, aws.go:461, azure.go:365]`:

```bash
sudo bash -c 'curl -fsSL https://raw.githubusercontent.com/friggalabs/dvarpala/main/scripts/installation/configure-oauth-firewall.sh | bash'
```

1. **The GitHub org is wrong.** `[VERIFIED BY EXECUTION, Aug 2026]`

   ```
   raw.githubusercontent.com/friggalabs/dvarpala/...    -> HTTP 404
   raw.githubusercontent.com/frigga-cloud/dvarpala/...  -> HTTP 200

   $ curl -fsSL https://raw.githubusercontent.com/friggalabs/dvarpala/... | bash
   curl: (56) The requested URL returned error: 404
   PIPELINE_EXIT=0          <-- reports SUCCESS
   ```

   Both orgs exist on GitHub; `friggalabs` simply does not host this repository.
   `curl -f` fails silently, `bash` receives empty input and **exits 0**, so the
   installer prints `Completed: Configuring OAuth firewall rules` having done
   nothing. **The OAuth allow-list has therefore never been applied on any
   deployment, with no error surfaced.**

   Fix: correct the org in `providers/{gcp,aws,azure}.go` — *and* set
   `pipefail` (or `scp` the file over the SSH session the installer already
   holds), so a future fetch failure cannot pass silently.

   **Corroboration:** `cloud-setup-server.sh` uses the **correct** org in its own
   usage comment (`:5`) and in its `git clone` (`:457`) — `frigga-cloud`. The wrong
   spelling appears only in the three provider files. `[VERIFIED]` That asymmetry
   is independent evidence that `friggalabs` is a typo rather than a second,
   deliberately-used organisation.
2. **It fetches from `main` at install time**, not from the operator's checkout, so
   installations are not reproducible against a pinned revision.
3. **If it *does* run, it aborts.** `configure-oauth-firewall.sh:50` executes
   `bash /opt/dvarpala/scripts/iptables-oauth-rules.sh`, but **nothing ever copies
   that file to the VM** — the live step list never places it, and
   `cloud-setup-server.sh` (which creates `/opt/dvarpala/scripts/`) is dead code.
   Under `set -euo pipefail` the script exits non-zero, which propagates through
   `executeSSHCommand` and **fails the whole installation** at that step.
   `[VERIFIED: configure-oauth-firewall.sh:1-50; grep for the file's creation]`

Either way, the OAuth allow-list is never applied. And note the consequence if it
*were*: the DNAT rules would redirect all web traffic to `172.30.100.1:8080`, where
**nothing is listening**, since no portal server is deployed (§9.2).

`configure-oauth-firewall.sh` also writes `ccd/DEFAULT` with
`ifconfig-push 172.30.100.2 255.255.255.0` — **the same fixed IP for every client**
— into `/etc/openvpn/server/ccd/`, while the designed server config reads
`/etc/openvpn/ccd/`. Two bugs in four lines. `[VERIFIED: configure-oauth-firewall.sh:129-146]`

### 9.7 Certificates — one identity for everybody

The live path builds exactly **two** certificates: `server` and `admin`
`[VERIFIED: gcp.go:333-337]`. There is **no per-user certificate generation
anywhere in the cloud path.**

Consequences:
- Every user connects with the same `admin.ovpn`, so every client presents
  `common_name = admin`.
- The designed `client-connect` keys authentication state on `$common_name` — so
  **one user authenticating would grant full access to every connected client.**
- The config sets no `duplicate-cn`, so OpenVPN disconnects the previous holder of
  a common name. **Effectively one concurrent user.** `[INFERRED — standard OpenVPN
  behaviour]`

> Note: the selective-blocking design (§9.10) avoids this entirely by setting
> `username-as-common-name`, deriving identity from the login rather than the
> certificate. The fix already exists in the repository — in the subsystem that is
> never installed.

The live path uses **easy-rsa 3** syntax correctly (`./easyrsa init-pki`,
`--batch build-ca nopass`, `build-server-full`, `build-client-full`) against Ubuntu
22.04, which all three providers launch `[VERIFIED: gcp.go:196, aws.go:379, azure.go:259]`.

By contrast the dead `cloud-setup-server.sh` uses **easy-rsa 2** syntax
(`./clean-all`, `./build-ca`, `./build-key-server`, `./build-dh`, `export KEY_*`),
removed from easy-rsa years before Ubuntu 22.04. With `set -euo pipefail` it would
abort at step 9 of 20. `[VERIFIED: cloud-setup-server.sh:557-581]` Another
indication that script has not been run recently, if ever.

### 9.8 ⚠️ `admin.ovpn` is published on the public internet during install

`[VERIFIED: gcp.go:~390-400]`

```bash
sudo cp .../certs/admin.ovpn /var/www/html/admin.ovpn && sudo chmod 644 ...
# then, backgrounded:
sleep 120 && sudo rm -f /var/www/html/admin.ovpn
```

The file is served by nginx over **plain HTTP on the VM's public IP for two
minutes**, with no authentication. `admin.ovpn` embeds the CA certificate, the
client certificate **and the client private key** — i.e. complete VPN credentials.
Anyone who requests `http://<public-ip>/admin.ovpn` inside that window obtains
permanent access, and (given §9.7) that is the *only* identity the system has.

The authors were aware — the code prints a "SECURITY NOTE" — but the mitigation is
a 120-second timer, not access control. The installer already holds an SSH session
to the VM; `scp` over that channel would remove the exposure entirely.

**The same file is then written to the operator's own machine world-readable:**

```go
ovpnPath := filepath.Join(config.OutputDirectory, "admin.ovpn")
return os.WriteFile(ovpnPath, []byte(ovpnTemplate), 0644)
```
`[VERIFIED: cloud-installer.go:1284]` — mode `0644`, private key included, in
`./dvarpala-deployment/`. Any other account on that machine can read it. Should be
`0600`, alongside the same fix for `installation-config.json` (§10.7a).

### 9.9 The deployed credentials do not match any document

The dead script's `create_auth_script()` writes a **bash** `openvpn-auth` that
checks `username = "portal"` and `password = "access"`
`[VERIFIED: cloud-setup-server.sh:889-916]` — not `temp_user` /
`temp_portal_access`, which is what `README.md`, `INSTALLATION.md`,
`dvarpala-context.md`, `check-status.sh` and the repo's own Go binary all use.

Worse, it writes to `/opt/dvarpala/bin/openvpn-auth` — **the exact path the compiled
Go binary was written to at step 6** (`DVARPALA_DIR="/opt/dvarpala"`). Step 14
therefore **overwrites the Go `openvpn-auth` binary with a bash script.**
`[VERIFIED: cloud-setup-server.sh:18, 478, 889]` The `cmd/openvpn-auth` program is
compiled and then discarded.

**And the shipped client profile expects those credentials.** The `.ovpn` the live
installer generates and hands to the operator contains
`[VERIFIED: gcp.go:547-576; same in aws.go, azure.go]`:

```
# Auto-open captive portal after connection
up "… open http://172.30.100.1:8080 …"

# Initial captive portal access credentials
# Username: portal, Password: access (for initial connection only)
auth-user-pass
<inline credentials: portal / access>
```

So the **client** is configured for a captive-portal flow — it will prompt for or
supply `portal`/`access` and auto-open the portal URL — while the **server** it
connects to has no `auth-user-pass-verify` and nothing listening as a portal (§9.2).
The credential pair is therefore not confined to the dead script: it ships to every
user. The client profile and the server configuration were written against
different designs.

### 9.10 A second, unmentioned design: selective blocking

Alongside the captive portal there is a **complete and internally coherent
alternative architecture** in `scripts/installation/installer/`, ~2,410 lines,
which no other document in this repository mentions:

| File | Lines | What it is |
|---|---|---|
| `selective-blocking-integration.sh` | 538 | Installer integration — creates the schema, the API service, the cron job |
| `test-selective-blocking.sh` | 487 | Test harness |
| `api/resource-management-api.go` | 576 | **The control plane** — REST API over the blocked-resource tables |
| `scripts/vpc-discovery.sh` | 356 | Enumerates cloud resources and populates the database |
| `openvpn-selective-config.sh` | 316 | OpenVPN config generator for this model |
| `schema/selective-blocking.sql` | 137 | `blocked_resources` + `vpc_resources` tables |

Its header states the intent plainly `[VERIFIED: selective-blocking-integration.sh:4]`:

> *"This replaces the captive portal approach with selective VPN blocking"*

**It inverts the model.** Instead of blocking everything and opening up after
authentication, the tunnel carries only what it must:

- `push "redirect-gateway def1 bypass-dhcp"` is **commented out**
  `[VERIFIED: openvpn-selective-config.sh:40]`
- clients get `route-nopull` — *"Don't accept any server routes"*
  `[VERIFIED: openvpn-selective-config.sh:102]`
- routes are pushed per client, derived from rows in `blocked_resources`

That is **split tunnelling** — a feature §1.4 records as absent from the product.
It is absent from `internal/` and from the deployed configuration, but a designed
implementation of it does exist here.

The subsystem is administrable by design: the API exposes
`GET`/`POST`/`DELETE /api/resources`, `GET /api/vpc-resources` and
`POST /api/discover-vpc-resources`, and applies changes by shelling out to
`update-iptables-blocking.sh` and `update-dns-blocking.sh`
`[VERIFIED: resource-management-api.go:478-483, 556-566]`. `vpc-discovery.sh` is
installed as an hourly cron job to keep the resource inventory current
`[VERIFIED: selective-blocking-integration.sh:516]`.

**Its schema is more advanced than the main application's** in two respects
`[VERIFIED: schema/selective-blocking.sql]`. It defines four tables —
`blocked_resources`, `vpc_resources`, `user_auth_status` and `resource_audit_log`,
plus `api_tokens` — and `user_auth_status` tracks real per-user session state
(`is_authenticated`, `auth_method` ∈ `oauth`/`saml`/`local`, `session_token`,
`expires_at`, `client_ip`, `vpn_ip`). It anticipates SAML, which nothing else here
does.

**It also solves the shared-identity problem of §9.7 without per-user certificates.**
The OpenVPN config sets `username-as-common-name`
`[VERIFIED: openvpn-selective-config.sh:77]`, so each client's identity comes from
their login name rather than the certificate. That single directive removes the
"everyone is `CN=admin`" defect — for this design only.

**Its gap: blocking is global, not per-team.** `blocked_resources` has no user,
group or role column `[VERIFIED: schema/selective-blocking.sql:5-18]`. The model is
*authenticated vs not*, so it does not express "Engineering may reach Grafana,
Finance may not" — which the main schema's `groups` / `group_permissions` tables do
(§5.3). Adopting this design would mean joining the two.

⚠️ **Its schema cannot be created.** The very first table carries an invalid
constraint `[VERIFIED: schema/selective-blocking.sql:16]`:

```sql
CREATE TABLE IF NOT EXISTS blocked_resources (
    ...
    UNIQUE(resource_type, resource_value) WHERE is_active = true   -- rejected
);
```

PostgreSQL's `UNIQUE` **table constraint** accepts no `WHERE` clause — partial
uniqueness requires a separate `CREATE UNIQUE INDEX`. The author knew that idiom and
used it correctly six times on lines 77-82; it is misapplied only here.

`[INFERRED — needs a live PostgreSQL to confirm the exact error]` `CREATE TABLE`
therefore fails, and with it every later statement that depends on
`blocked_resources` — its indexes, the seeded default resources, and the
`get_blocked_ips()` function the routing script calls. The installer runs
`psql -d dvarpala -f …selective-blocking.sql` **without `ON_ERROR_STOP`**
`[VERIFIED: selective-blocking-integration.sh:175]`, so `psql` exits 0 and the step
reports success. **Another silent failure (§11.3): the subsystem could not work even
if it were invoked.**

⚠️ **It ships a hardcoded default admin API key.** The installer seeds
`dvarpala-default-admin-key-2024` into `api_tokens` and prints it on completion
`[VERIFIED: selective-blocking-integration.sh:170, 533]`. Every install would share
the same control-plane credential, and it is published in a public repository
(§9.6). The column is named `token_hash` but the literal key is inserted — tokens
are stored unhashed.

⚠️ **Its schema is not valid PostgreSQL, and fails on the first statement.**
`blocked_resources` — the subsystem's central table — declares
`[VERIFIED: schema/selective-blocking.sql:16; duplicated at selective-blocking-integration.sh:60]`:

```sql
CREATE TABLE IF NOT EXISTS blocked_resources (
    ...
    UNIQUE(resource_type, resource_value) WHERE is_active = true   -- ← line 16
);
```

**A table-level `UNIQUE` constraint accepts no `WHERE` clause.** Partial uniqueness
requires a separate `CREATE UNIQUE INDEX … WHERE`. So the first `CREATE TABLE` in
the file aborts and `blocked_resources` is never created — which cascades: the
`ON CONFLICT (resource_type, resource_value)` seed at `:88` then has no constraint
to match, and `get_blocked_ips()` / `get_blocked_domains()` (`:106`) query a table
that does not exist.

It fails **silently**, and is a clean instance of §11.3. The script sets neither
`set -e` nor `ON_ERROR_STOP`, so `psql -f` prints the error and exits 0 — and the
very next line logs `"Database schema for selective blocking created"`
`[VERIFIED: selective-blocking-integration.sh:175-177]`.

`[INFERRED — from PostgreSQL's documented grammar; no live database was available]`
This is also **evidence the design has never been run**: the first `psql` invocation
of an end-to-end attempt would have failed.

**Like everything else in §9, it is never invoked.** `integrate_selective_blocking()`
is called only from the bottom of its own file (`:527`); nothing outside the
subsystem references it. `[VERIFIED: grep across all *.go and *.sh]`

**It does *not* replace the captive portal — it extends it.** Despite the header
quoted above, its own `client-connect-selective.sh` retains the walled garden as the
unauthenticated state `[VERIFIED: openvpn-selective-config.sh:95-105]`:

```bash
if [[ ! -f "$AUTH_STATUS_FILE" ]]; then
    # "CAPTIVE PORTAL MODE"
    echo "route 172.30.100.1 255.255.255.255"   # portal only
    echo "route-nopull"                          # accept no other routes
    exit 0
fi
# authenticated → push routes for protected resources
```

So the two-step model of §1.2 is preserved intact. **The only thing that changes is
what "full access" means after authentication:**

| | Post-authentication routing |
|---|---|
| Captive portal design (§9.3) | `redirect-gateway def1` — *all* client traffic through the VPN |
| Selective blocking (here) | routes pushed *only* for protected resources; other traffic goes direct |

`[INFERRED]` These are therefore **not competing architectures** but successive
refinements of one. The repository holds three unexecuted **access-gating** designs —
§9.3, this one, and the branch's (§12.1); these are separate from the three
*web-server implementations* catalogued in §8.6. Choosing between the first two is a
narrow question ("full tunnel or split tunnel?"), not an architectural fork.

⚠️ **The authentication flag moved to a worse location.** This design keys on
`/tmp/dvarpala-auth-status-<username>` `[VERIFIED: openvpn-selective-config.sh:95]`,
where §9.3 used `/var/lib/dvarpala/auth/`. `/tmp` is cleared on reboot and is
world-writable, so **any local account on the VPN server could create that file for
any username** and grant itself authenticated routing. The `user_auth_status` table
that should hold this state is never written to by anything. `[VERIFIED: grep]`

### 9.11 Data-plane verdict

| Capability | Designed | Deployed |
|---|---|---|
| Encrypted tunnel | ✅ | ✅ **works** |
| Certificate PKI | ✅ | ✅ **works** (2 certs only) |
| Cloud infrastructure | ✅ | ✅ **works** |
| Walled garden / captive portal | ✅ | ❌ absent |
| OAuth allow-listing | ✅ | ❌ never applied |
| Two-step promotion | ✅ | ❌ absent |
| Revocation on disconnect | ✅ | ❌ absent |
| Per-user identity | ❌ | ❌ absent |
| Dvarpala application | ✅ | ❌ not installed |

`[INFERRED]` **What ships today is a working cloud-provisioned OpenVPN server.**
The Zero Trust layer — the product's entire differentiator — exists only as design
artefacts, none of which execute: `cloud-setup-server.sh` and
`iptables-oauth-rules.sh` (§9.3–§9.4), the six selective-blocking files (§9.10), and
`base_provider.go` on the unmerged branch (§12.1). §10 examines the installer, which
is the part that does work.

---

## 10. The cloud installer

This is the part that works, and the largest body of real code in the repository.
It is a **separate Go module** (`module dvarpala-cloud-installer`, Go 1.19, one
dependency: `lib/pq`) that runs on the **operator's own machine** and provisions a
complete VPN server on AWS, GCP or Azure.

### 10.1 What it does

`[VERIFIED: installer/cloud-installer.go:77-190]` A single linear flow:

```
 1. load config      (JSON file │ interactive wizard │ CLI flags)
 2. generate Frigga resource names
 3. validate config
 4. create output directory
 5. install the cloud CLI if missing   (aws │ gcloud │ az — macOS/Linux/Windows)
 6. authenticate to the cloud
 7. create or reuse VPC + subnets + firewall rules
 8. create VM  ──► SSH in and run the ~17-step installation (§9.2)
 9. verify installation over HTTP
10. download admin.ovpn to the operator's machine
11. write installation-config.json + connection-info.txt
12. create object-storage bucket and upload config backup
```

Invocation `[VERIFIED: cloud-installer.go:78-83]`:

```bash
go run launcher.go                                   # interactive wizard (default)
go run launcher.go -config examples/config-gcp.json  # unattended
go run launcher.go -provider aws -region us-east-1   # from flags
```

### 10.2 Architecture — a clean four-layer design

```
launcher.go            shells out to `go run installer/*.go`
   │
cloud-installer.go     orchestration, wizard, validation, output files   (1,486)
   │
cloud_service.go       CloudService — provider-agnostic operations          (86)
   │
cloud_wrappers.go      AWSWrapper │ GCPWrapper │ AzureWrapper              (281)
   │
providers/*.go         AWSProvider │ GCPProvider │ AzureProvider    (812/724/740)
```

The seam is a four-method interface `[VERIFIED: cloud_service.go:15-20]`:

```go
type CloudProviderWrapper interface {
    SetupVPC(name string) (string, error)
    CreateVM(vpcID string, instanceConfig InstanceConfig) (*VMInfo, error)
    SetupStorage(bucketName string) error
    UploadConfig(bucketName string, data []byte, filename string) error
}
```

selected by a single factory switch `[VERIFIED: cloud_service.go:33-45]`. The
installer README's claim that this "eliminates repetitive switch statements and
follows proper OOP principles with factory patterns" is **accurate** — there is
exactly one switch, in the constructor. This is the best-structured code in the
repository.

`[INFERRED]` Note the providers do not talk to cloud SDKs — they **shell out to the
vendor CLIs** (`aws ec2 …`, `gcloud compute …`, `az vm …`). That is why step 5
installs the CLI. It trades type safety and error handling for a much smaller
dependency tree — a defensible choice for an installer, and it explains the single
`go.mod` requirement.

### 10.3 Provider parity

All three are implemented to the same shape — roughly 20 methods each, same
lifecycle `[VERIFIED]`:

| Capability | AWS | GCP | Azure |
|---|---|---|---|
| `SetupEnvironment` / `ValidateAuthentication` | ✅ | ✅ | ✅ |
| `CreateOrGetVPC` (idempotent — finds existing first) | ✅ | ✅ | ✅ |
| security groups / firewall rules | ✅ | ✅ | ✅ (NSG) |
| `CreateInstance` on **Ubuntu 22.04** | ✅ | ✅ | ✅ |
| minimal startup script / user-data / cloud-init | ✅ | ✅ | ✅ |
| `InstallDvarpalaDirectly` (the ~17 SSH steps) | ✅ | ✅ | ✅ |
| `waitForSSHAccess` / `executeSSHCommand` | ✅ | ✅ | ✅ |
| easy-rsa 3 PKI + `generateAdminOVPN` | ✅ | ✅ | ✅ |
| nginx monitoring endpoint | ✅ | ✅ | ✅ |
| object storage (S3 / GCS / Storage Account) | ✅ | ✅ | ✅ |

Genuinely equal treatment — no "AWS first, others bolted on". VPC creation is
**idempotent**: each provider looks up an existing VPC by name before creating one
`[VERIFIED: gcp.go:97, aws.go:91, azure.go:106]`.

### 10.4 Frigga conventions

Two deliberate house standards, both consistently applied `[VERIFIED: scripts/installation/README.md:33-90]`:

- **Brand IP space `172.30.0.0/26`** — chosen because "F" is the 6th letter × 5 = 30,
  inside RFC 1918, and unlikely to collide with the usual `10.x`/`192.168.x`.
- **Resource naming `friggalabs-{resource}-{suffix}`** — applied to VPC, VM, bucket
  and keypair.

> ⚠️ The suffix is **not random**: `generateRandomSuffix()` returns
> `time.Now().Unix() % 100000` `[VERIFIED: cloud-installer.go:802-804]`. Two
> installations started in the same second collide, and values repeat roughly every
> 27 hours. The README describes it as "a 5-character alphanumeric string for
> uniqueness"; it is in fact 1–5 digits derived from the clock.

### 10.5 Build status — measured

`[VERIFIED BY EXECUTION, Go 1.26.5]`

| Target | Result |
|---|---|
| `go build ./installer` (all 4 files) | ✅ **OK** |
| launcher's exact 3-file set | ✅ **OK** |
| `go build ./providers` | ✅ **OK** |
| **`go build ./...`** | ❌ **FAILS** — `installer/api` |

```
installer/api/resource-management-api.go:17:2:
    no required module provides package github.com/gorilla/mux
```

`gorilla/mux` is absent from `scripts/installation/go.mod`. **The 576-line resource
management API has never compiled in this module**, and nothing references it
`[VERIFIED: grep across the repository]`.

### 10.6 Dead code inside the installer module

| Component | Lines | Status |
|---|---|---|
| `installer/api/resource-management-api.go` | 576 | **Does not compile** (missing `gorilla/mux`) and never invoked — but *not* orphaned: it is the control plane for the selective-blocking subsystem (§9.10) |
| `installer/databaseInstaller.go` | 432 | Compiles, but `mainDatabaseInstaller()` is **never called** — the file's own closing comment says *"To use this database installer, call mainDatabaseInstaller() from elsewhere"* `[VERIFIED: databaseInstaller.go:431]`. The launcher also omits it from its build set. |
| `installer/cloud-setup-server.sh` | 1,436 | Invoked by nothing (§9.1) |

**≈2,440 lines — over a third of the installer directory — is unreachable.** Note
that `databaseInstaller.go` contains real `CREATE TABLE` DDL and seed data, and is
the only place the installer would ever populate a database. Because it is never
called, **no deployment has a Dvarpala schema** — consistent with §9.2, where the
application is never installed either.

### 10.7 Defects and security findings

**(a) ⚠️ Cloud credentials are written world-readable and uploaded to the cloud.**
`InstallationConfig.Cloud.Credentials` holds the AWS `secret_key`, the Azure
`client_secret`, or the path to a GCP service-account key
`[VERIFIED: cloud-installer.go:34-40; examples/config-aws.json]`. That whole struct
is then:

```go
configData, _ := json.MarshalIndent(config, "", "  ")
os.WriteFile(configPath, configData, 0644)     // installation-config.json
```
`[VERIFIED: cloud-installer.go:690-695]` — **mode 0644**, readable by every user on
the machine. The same JSON is uploaded to the object-storage bucket by
`UploadConfiguration()` `[VERIFIED: cloud_service.go:78-86]`.

Fix: mode `0600`, and redact `Credentials` before marshalling — the field is not
needed in either the local artefact or the backup.

**(b) The password prompt echoes.** `readPassword()` uses `fmt.Scanln`
`[VERIFIED: cloud-installer.go:792-800]`, so the AWS secret key / Azure client
secret is displayed on screen and left in terminal scrollback. `golang.org/x/term`
`ReadPassword` is the standard fix.

**(c) The default network policy is open to the world.** Every example config ships
`"allowed_ips": ["0.0.0.0/0"]` `[VERIFIED: examples/config-{aws,gcp,azure}.json]`,
which is passed to `addSecurityGroupRules` / `addSecurityRules`. Reasonable for
UDP/1194, less so for SSH and the nginx monitoring endpoint.

**(d) The `.gitignore` protects the output directory — but the installer's default
lives elsewhere.** `.gitignore` excludes `dvarpala-deployment/` and
`scripts/installation/dvarpala-deployment/` `[VERIFIED: .gitignore:47-48]`, and the
default `OutputDirectory` is `./dvarpala-deployment` — so it is covered **only when
the installer is run from the repository root or from `scripts/installation/`**.
Run from anywhere else and `installation-config.json`, containing credentials, is
not protected.

**(e) The launcher requires a Go toolchain on the operator's machine.** It shells
out to `go run` rather than being a compiled binary
`[VERIFIED: launcher.go:19-33]`. Three pre-built binaries *are* committed —
`dvarpala-installer` (3.5 MB), `test-installer`, `test-launcher` — so both
distribution models appear to have been used. Committing multi-megabyte binaries is
worth revisiting.

**(f) The launcher's file list is hand-maintained.** It names three files
explicitly; adding a fourth source file to `installer/` silently omits it from the
launcher build. `go run ./installer` would be equivalent and self-maintaining.

### 10.8 Extending it — practical notes

To **add a fourth cloud provider**, the seam is well placed:
1. Add `providers/<cloud>.go` implementing the ~20-method shape (copy `gcp.go`).
2. Add a `<Cloud>Wrapper` in `cloud_wrappers.go` satisfying `CloudProviderWrapper`.
3. Add one `case` to the factory in `cloud_service.go:33`.
4. Add a `setup<Cloud>Config()` wizard branch and a CLI installer function.

To **change what is provisioned on the VM**, edit the `steps` slice inside that
provider's `InstallDvarpalaDirectly` — but note it is duplicated three times, once
per provider, and the OpenVPN config appears three times as a heredoc string
(`getOpenVPNServerConfigCommand`). **This is the natural first refactor**, and it is
precisely what the unmerged `installer/restructure` branch attempts with its
666-line `base_provider.go` (§4, §12).

### 10.9 Installer verdict

`[INFERRED]` **The installer is production-shaped code with a clean abstraction,
genuine three-cloud parity and idempotent VPC handling.** Its weaknesses are
credential hygiene, ~2,440 lines of unreachable code, and the fact that what it
installs (§9.2) is not the product the rest of the repository describes. As a
"provision an OpenVPN server on any of three clouds" tool it is close to complete;
as a "deploy Dvarpala" tool it stops short of deploying Dvarpala.

---

## 11. Operations surface and platform integration

### 11.1 Dvarpala is completely standalone

`[VERIFIED via code0 across all 8 indexed repositories, Aug 2026]`

The organisation has eight connected repositories: `dvarpala`, `code-service`,
`cloud-mapper`, `vor-ai`, `vor-ai-frontend`, `Organisation-Service`,
`user-service-2`, `Frigga-Accounts-Hub`.

**No repository other than `dvarpala` contains a single reference to it.** Searches
for `dvarpala` returned zero matches in `vor-ai`, `vor-ai-frontend`,
`Organisation-Service`, `cloud-mapper`, `code-service`. The apparent hits in
`Frigga-Accounts-Hub` and `user-service-2` are coincidental substrings inside
`package-lock.json` dependency hashes, not code.

This matters for two reasons:

- **Nothing depends on Dvarpala.** You can change anything in this repository
  without breaking another Frigga service. That is unusually free rein.
- **Dvarpala does not use Frigga's own identity platform.** The organisation
  operates `Frigga-Accounts-Hub` and `user-service-2`, yet Dvarpala implements its
  own user table and its own OAuth provider configuration from scratch. Given the
  product page markets *"remove someone from your IdP and their VPN access
  disappears instantly"* (§1.4), integrating with the in-house accounts service
  looks like the obvious route to that feature — and it currently does not exist.
  `[INFERRED]` `[OPEN-25]`

### 11.2 What monitoring a deployed server actually has

The installer deploys one nginx site with three endpoints
`[VERIFIED: gcp.go:596-620; identical in aws.go:687, azure.go:591]`:

```nginx
location /health {
    return 200 '{"status":"healthy","timestamp":"$(date -Iseconds)"}';
}
location /installation-progress {
    return 200 '{"current_step":"Installation completed","completed_steps":9,"total_steps":9}';
}
location /installation-status {
    return 200 'Installation completed successfully';
}
```

**All three are hardcoded string literals.** They are nginx `return` directives —
they check nothing. `/health` reports healthy whether or not PostgreSQL, Redis,
OpenVPN or the application are running (and none of the latter is even installed,
§9.2). `/installation-progress` reports 9-of-9 complete from the moment nginx
starts, including during a failed install.

The `$(date -Iseconds)` inside the timestamp is written in a quoted heredoc and
served by nginx, which performs no command substitution — so the response contains
the **literal text** `$(date -Iseconds)` rather than a time. `[VERIFIED]`

### 11.3 Installation verification is cosmetic

`[VERIFIED: cloud-installer.go:854-868]` The installer's verification step is:

```go
curl -s http://<vm-ip>:8080/health
```

against the hardcoded endpoint above. It therefore **always succeeds** provided
nginx is running. The "✅ Installation verification successful" message carries no
information about whether the VPN, database, or portal work.

Combined with §9.6 (the OAuth firewall step silently no-ops), §9.10 (the
selective-blocking schema fails in `psql` and is logged as created) and §11.2, a
completely non-functional Dvarpala installation reports success at every checkpoint.
`[INFERRED]` This is the mechanism by which the gap between intent and reality could
persist unnoticed — **nothing in the system is capable of reporting failure.**

### 11.4 Port 8080 is claimed twice

nginx binds `8080` on every provider `[VERIFIED: 6 occurrences across the three
provider files]`. But `8080` is also the captive portal's port — the firewall DNATs
unauthenticated traffic to `172.30.100.1:8080` (§9.4), and the portal server binds
`SERVER_PORT` defaulting to `8080`.

`[INFERRED]` If the portal server were ever deployed alongside this nginx config,
one of the two would fail to bind. As shipped there is no conflict only because the
portal is never installed — a captive user redirected to `:8080` would reach the
**hardcoded health-check page**, not a login screen.

### 11.5 Containers and orchestration

| Artefact | State |
|---|---|
| `deployments/docker/Dockerfile` | **Real** — 41 lines, multi-stage, builds 3 of the 4 binaries |
| `deployments/docker/docker-compose.yml` / `.prod.yml` | **empty** |
| root `docker-compose.yml` / `.prod.yml` | **empty** |
| `deployments/docker/Dockerfile.auth`, `.dockerignore` | **empty** |
| `deployments/kubernetes/*` (12 files) | **all empty** |
| `deployments/terraform/*` (8 files) | **all empty** |
| `deployments/ansible/*` (2 files) | **all empty** |

The Dockerfile is competent — pinned `golang:1.21-alpine` builder, `CGO_ENABLED=0`,
slim `alpine` runtime, copies `configs/` and `web/` `[VERIFIED]`. Two problems:
it omits `dvarpala-worker`, and its `CMD ["./dvarpala-server"]` passes no `-config`,
so the binary falls back to `configs/environment.yaml` and hits the failure in §6.3.
There is also no compose file to supply PostgreSQL or Redis, so the image cannot run
usefully on its own.

`[INFERRED]` **The container and orchestration story is aspirational.** The
Dockerfile was written; nothing that would run it was.

### 11.6 Logging and audit

- **OpenVPN** logs properly — `status`, `log-append`, `verb 3` to `/var/log/openvpn/`
  `[VERIFIED: provider server.conf]`. This is the one real operational signal.
- The designed hooks log connect/disconnect events to their own files (§9.3) — dead code.
- **The `audit_logs` table is never written.** `models.AuditLog` appears only in
  `migrate.go` (table creation) and the dev seed script. No application code
  creates an audit record. `[VERIFIED: grep]` The "Full audit trail" on the product
  page (§1.4) has no implementation.
- `configs/monitoring/prometheus.yml` and `grafana-dashboard.json` are **both empty**.
- `scripts/monitoring/health-check.go` and `vpn-status.go` are the 6-line
  `// TODO: Implement` stubs that break `go build ./...` (§7.1).

### 11.7 Backup, deploy, rollback

`scripts/deployment/backup.sh`, `deploy.sh` and `rollback.sh` are **all 0 bytes**
`[VERIFIED]`.

The only backup that exists is the installer's object-storage upload — which stores
`installation-config.json` **including cloud credentials** (§10.7a) and nothing
else. No database backup, no certificate backup, no PKI backup. **If the VM is
lost, the certificate authority is lost with it**, and every issued `.ovpn` becomes
unusable. `[INFERRED]`

### 11.8 What a real operator has to work with

| Need | Available |
|---|---|
| Is the VPN up? | OpenVPN status log on the box (SSH only) |
| Is the app up? | ⚠️ `/health` — always says yes |
| Who connected, when? | OpenVPN logs only; no audit records |
| Metrics / dashboards | ❌ empty files |
| Alerting | ❌ none |
| Backup / restore | ❌ empty scripts; PKI unprotected |
| Deploy an update | ❌ empty script — re-run the installer |
| Roll back | ❌ empty script |

`[INFERRED]` **Operationally, a deployed Dvarpala server is a black box you SSH
into.** The provisioning path (`scripts/provisioning/check-status.sh`, 185 lines,
and `emergency-access.sh`, 137 lines) does contain real operator tooling — but it
belongs to the superseded generation (§8.6) and targets the old `192.168.100.0/24`
scheme, so it does not match what the cloud installer deploys.

---

## 12. The unmerged branch: `installer/restructure`

16 commits, **+4,110 / −6,743 lines**, last touched **23 June 2025** — four days after
`main` went quiet. Whether to adopt it is a decision a newcomer has to make early,
so it is assessed on its merits rather than summarised.

All findings below were produced by extracting the branch to a scratch directory
and building it. **The working tree was not modified.**

### 12.1 What it changes

**Structural**
- `scripts/installation/` → `installation/` (top-level, matching its importance)
- Deletes `cloud-installer.go` (1,486), `cloud_service.go`, `cloud_wrappers.go`,
  `launcher.go` — replaced by `main.go` (144) + `lib/cloud_lib.go` (475) +
  `lib/config_lib.go` (222)
- **Deletes `cloud-setup-server.sh` (1,436 lines)** — the dead script of §9.1
- **Deletes ~9 MB of committed binaries** (`dvarpala-installer`, `test-installer`,
  `test-launcher`)

**The refactor that matters**
- `providers/base_provider.go` (666 lines) centralises what was triplicated:
  one `InstallDvarpalaDirectly`, one `getOpenVPNServerConfigCommand`, one nginx
  config, one easy-rsa flow, one `generateAdminOVPN`. The three providers become
  thin subclasses. **This is precisely the refactor recommended in §10.8.**

**Genuinely new capability — not present on `main` at all**
- `configureCaptivePortal()` — called from `InstallDvarpalaDirectly:167`
- `Configure2StepVPNAccess()` — called from `main.go:122`, implemented by all three
  providers, delegating to the base
- `setupPostgreSQLDatabase()` — database provisioning during install
- `database/postgres_handler.go` (375) + `schema/installer.go` (160)
- **Real SQL migrations and seeds** — `001_create_users_table.sql`,
  `002_create_groups_table.sql`, `003_create_sessions_table.sql`,
  `001_default_admin_user.sql` (the empty `.sql` files of §5.1, finally written)

**Housekeeping**
- Root `go.mod` tidied — actually-used dependencies promoted to direct requires,
  ~10 unused indirect ones dropped, `go.sum` −614 lines
- `installation/go.mod` **adds `gorilla/mux`** — fixing the API that never compiled
  (§10.5, `[OPEN-21]`)
- `CONFIG_USAGE.md` (283 lines) of new documentation

`[INFERRED]` This branch is aimed squarely at the two largest problems this document
identifies: the triplicated provider logic, and the fact that the deployed system
has no captive portal and no database. **Whoever wrote it knew what was wrong.**

### 12.2 Does it build? No — three separate reasons

`[VERIFIED BY EXECUTION, Go 1.26.5, branch extracted to a scratch directory]`

**(a) `go.sum` is missing from both modules.** The branch added `*.sum` to
`.gitignore` `[VERIFIED: git diff main...restructure -- .gitignore]`:

```diff
 dvarpala-deployment/
-scripts/installation/dvarpala-deployment/
+installation/dvarpala-deployment/
+
+*.sum
```

Both `go.sum` files were consequently deleted from the branch (`go.sum` and
`scripts/installation/go.sum`, present on `main` at 59,734 and 50,508 bytes, are
**absent**). Every build fails immediately with `missing go.sum entry`.

**This is a mistake and should be reverted.** `go.sum` is the cryptographic lockfile
that pins every dependency's hash. Ignoring it removes supply-chain verification and
makes builds non-reproducible. It must be committed.

**(b) A genuine compile error in new code.** After regenerating `go.sum` with
`go mod tidy`, the installation module still fails:

```
installer/database/test_db.go:23:2: declared and not used: handler
```

`test_db.go` (61 lines) is a scratch harness containing a hardcoded password
(`dvarpala123`) and an unused variable. It looks committed by accident.

**(c) The root module's three duplicate-`main` packages are unfixed** — the same
`scripts/monitoring`, `scripts/migration` and `tools/generators` failures as `main`
(§7.1).

So: `go mod tidy` + delete `test_db.go` gets the installer module compiling. The
branch is **mid-refactor, not abandoned mid-thought** — but it was never left in a
building state.

### 12.3 What it does *not* fix

| Issue | On `main` | On the branch |
|---|---|---|
| `friggalabs` URL (§9.6) | broken | **still broken — now in 3 places**, including a `git clone https://github.com/friggalabs/dvarpala.git` |
| `push "redirect-gateway def1"` (§9.2) | full access on connect | **still present** in the base server config |
| Duplicate-`main` build failures (§7.1) | broken | still broken |
| Hardcoded nginx health endpoints (§11.2) | cosmetic | still cosmetic |

And it introduces **new instances of known bug patterns**:

- `Configure2StepVPNAccess` writes an OAuth config containing
  `"client_id": "${GOOGLE_CLIENT_ID}"` into a **quoted heredoc**, so no substitution
  occurs and the literal `${GOOGLE_CLIENT_ID}` lands in the file — the identical
  mistake to §6.3, in new code. `[VERIFIED: base_provider.go:552-567]`
- That same config sets `"allowed_domains": ["*"]`, which disables the email-domain
  allow-list that §1.2 identifies as a core control.
- It adds **2FA via the Google Authenticator PAM module**, which is a *different*
  mechanism from the OAuth captive portal — appended to `server.conf` alongside the
  still-present `redirect-gateway`. `[VERIFIED: base_provider.go:576-596]`
- `aws.go:594` uploads to a hardcoded `s3://friggalabs/dvarpala/...`, ignoring the
  configured bucket name.

### 12.4 Recommendation

`[INFERRED — this is a judgement call, offered with reasoning]`

**Adopt the branch, but as a starting point rather than a merge.**

The case for it:
- It deletes ~2,900 lines of confirmed dead weight (`cloud-setup-server.sh`, 9 MB of
  binaries) that would otherwise mislead every future reader.
- `base_provider.go` is the right structure, and re-deriving it would cost days.
- It contains the **only** attempt anywhere in the repository to wire the captive
  portal and the database into the deployed path.
- The SQL migrations are real work that does not exist elsewhere.

The case for caution:
- It does not build, and has never been demonstrated to work end to end.
- The new two-step code repeats the `${VAR}` substitution bug and weakens the domain
  allow-list, so it cannot be trusted as-written.
- 14 months of no activity means no one's memory of it is fresh.

**Suggested sequence:** revert `*.sum` and restore both `go.sum` files; delete
`test_db.go`; get it compiling; then treat `Configure2StepVPNAccess` and
`configureCaptivePortal` as *drafts to be reviewed*, not working features. The
structural work is worth keeping; the new functional code needs the same scrutiny
this document applied to `main`.

---

## 13. Consolidation

### 13.1 What was verified by execution

| Finding | Method |
|---|---|
| All four binaries + `internal/` + `pkg/` compile | `go build` |
| `go build ./...` fails in 3 stub packages | `go build`, `go vet` |
| Zero test coverage | `go test ./...` |
| Viper does not expand `${VAR:default}` | isolated harness, Viper v1.16.0 |
| `group_permissions` has three conflicting definitions | GORM `schema.Parse` on the real models |
| `--migrate` flag does not exist | ran the binary |
| `.env` → nested config yields all-empty values | ran the binary as systemd does |
| `friggalabs` URL 404s; pipeline still exits 0 | `curl`, pipeline simulation |
| No other org repo references Dvarpala | code0, all 8 repositories |
| The restructure branch does not build | extracted and built both modules |

### 13.2 Confidence statement

**High confidence** (read directly, or executed): everything in §5, §6, §7, §8,
§10, §12, and the deployed-vs-designed distinction in §9.1–9.2.

**Medium confidence** (read carefully, but not executed against live
infrastructure): the iptables rule-ordering analysis in §9.5(a), the
non-terminating-`MARK` analysis in §9.5(e), the single-concurrent-user consequence
in §9.7, the invalid partial-`UNIQUE` constraint in §9.10, and the port-8080
collision in §11.4.
Each is marked `[INFERRED]` in place and would be settled by one test deployment.

**Not assessed:** anything requiring a running VPN server, a populated database, or
a cloud account. No deployment was observed. If the team has a running instance,
the fastest way to validate this document is to check §9.2 against its
`/etc/openvpn/server/server.conf`.

### 13.3 Closing judgement

The state of the system is summarised at the top of this document (*At a glance*)
and per-layer in §9.11; it is not repeated here. What that summary cannot convey is
the **shape of the remaining work**, which is the more useful thing to say last.

**The distance from here to a working product is shorter than the volume of missing
functionality suggests.** The reason is that almost nothing left to do is unsolved:

- The captive-portal mechanism has already been designed and written in full — the
  hook scripts, the routing logic, the firewall marking scheme (§9.3, §9.4). It has
  never been *run*, but it does not need to be *invented*.
- The database schema is complete and coherent (§5), and the SQL to create it exists
  on the unmerged branch (§12.1).
- The installer that provisions everything works (§10).

What separates those pieces is wiring, not design: a script nothing calls, a URL
with the wrong org, a config file in the wrong format, a firewall rule that is never
applied. Each is small. Collectively they are why nothing works end to end.

The two genuinely unsolved problems are **per-user identity** — the system issues
one certificate for everyone (§9.7), already solved in the unexecuted §9.10 design —
and **deciding which server implementation is the real one** (§8.6). Both are
decisions before they are code.

`[INFERRED]` A reasonable first move is not to write features, but to **make the
system capable of reporting failure** (§11.3). Every
checkpoint currently reports success on a broken install, and that single property
is what allowed the gap between design and reality to grow this wide unnoticed.

---

## Open questions for the developers

**5 resolved during this review · 32 need a human answer.** Ordered by how much the
answer would change what a newcomer should do.

### Resolved — no action needed

| # | Question and answer |
|---|---|
| **OPEN-1 / OPEN-9** | *Which network ranges are correct?* Two installer generations, each internally consistent. The **cloud installer (active)** deploys captive `172.30.100.0/24`, full `172.30.8.0/21`, VPC `172.30.0.0/26`. The older `scripts/provisioning/` path uses `192.168.100.0/24` / `10.8.0.0/24` — which is what the README and `.env.example` describe. **The docs describe the superseded scheme.** (§6.6, §9.2) |
| **OPEN-6** | *What exports the env vars in a real deployment?* **Nothing does.** systemd passes `--config .../.env`; Viper parses it as flat dotenv keys that never match the nested `Config` struct, producing an all-empty config with no error. (§6.4, §8.7) |
| **OPEN-11** | *DB authority or IdP directory?* **Answered by the team: the Dvarpala database is the authority.** Clarification from code — the DB gate applies to **promotion to full access**, not to tunnel creation; anyone with the shared credential reaches the portal by design. (§1.4, §8.5) |
| **OPEN-17** | *Is the `friggalabs` fetch URL wrong?* **Yes — verified.** `friggalabs/dvarpala` → 404; `frigga-cloud/dvarpala` → 200. The pipeline exits 0 regardless, so the OAuth firewall step has silently no-opped on every install. **One-word fix in three files.** (§9.6) |

### Direction and intent — answer these first

| # | Question | § |
|---|---|---|
| **OPEN-31** | Three complete access-gating designs exist and none executes: captive portal (§9.3), selective blocking (§9.10), the branch's two-step wiring (§12.1). They are **not** mutually exclusive — §9.10 keeps the walled garden and changes only post-authentication routing. So the question is narrow: **full tunnel or split tunnel after login?** *Evidence favours split: the product page lists "Split Tunneling" and never mentions a captive portal; §9.10 is the newest work in the tree. Unconfirmed — needs a human answer.* | §9.10 |
| **OPEN-32** | The ~2,410-line selective-blocking subsystem (with its own schema, control-plane API and VPC discovery) is documented nowhere and invoked by nothing. Superseded, or unfinished? | §9.10 |
| **OPEN-16** | The deployed OpenVPN config pushes `redirect-gateway def1` — full access on connect, no captive portal. **Was the two-step model ever running on a deployed server?** | §9.2 |
| **OPEN-15** | `cloud-setup-server.sh` (1,436 lines — the only complete captive-portal implementation) is invoked by nothing. Abandoned, or was the wiring lost? | §9.1 |
| **OPEN-3** | Is `installer/restructure` to be merged or discarded? *Assessed in §12: it does not build, but holds the only attempt to wire the portal and database into the deployed path. Recommendation — adopt as a starting point, not a merge.* | §12 |
| **OPEN-4** | Was the Go `internal/` application ever meant to be completed, or did the design shift to "installer + shell scripts" as the real product? | §2.2 |
| **OPEN-12** | Which deployment path is current — `scripts/provisioning/` (demo server, simulated OAuth) or `scripts/installation/` (cloud installer)? Is the older one dead? | §8.6 |
| **OPEN-25** | Frigga runs `Frigga-Accounts-Hub` and `user-service-2`, yet Dvarpala builds its own identity and references neither. **Is integration planned?** It is the natural route to the advertised "remove from IdP → access revoked". | §11.1 |
| **OPEN-10** | The product page promises Okta, MFA, geo/device policies, split tunnelling, IdP sync. Planned work, or built somewhere not visible here? | §1.4 |
| **OPEN-13** | Was `dvarpala-server` intended to exist three times, twice as heredocs inside a shell script? | §8.6 |

### Security — worth a decision regardless of direction

| # | Question | § |
|---|---|---|
| **OPEN-18** | Only `server` and `admin` certificates are generated, so **all clients share `CN=admin`**. Is per-user issuance planned? Without it, per-user auth state is impossible. | §9.7 |
| **OPEN-19** | `admin.ovpn` (containing the client private key) is served over plain HTTP on the public IP for 120s during install. Switch to `scp` over the SSH session already open? | §9.8 |
| **OPEN-23** | `installation-config.json` is written mode 0644 **and uploaded to object storage with cloud credentials in it**. Intended? | §10.7 |
| **OPEN-28** | Nothing backs up the certificate authority. If the VM is lost, **every issued `.ovpn` stops working**. Accepted risk? | §11.7 |
| **OPEN-20** | `frigga-cloud/dvarpala` is publicly readable. Intended (open-core), given it documents default credentials and the `admin.ovpn` window? | §9.6 |
| **OPEN-30** | The branch sets `"allowed_domains": ["*"]` and adds Google Authenticator PAM 2FA. Is PAM 2FA the intended direction — replacing or supplementing the OAuth portal? | §12.3 |

### Concrete bugs — cheap to fix once confirmed

| # | Question | § |
|---|---|---|
| **OPEN-7** | Is the `${VAR:default}` syntax in `configs/environment.yaml` a known issue, or was it believed to work? Viper never expands it. | §6.3 |
| **OPEN-8** | Delete the deprecated `Permission` model and the two `many2many:group_permissions` fields? Three definitions currently claim one table. | §5.4 |
| **OPEN-14** | `cloud-setup-server.sh:877` calls `--migrate`, a flag that does not exist. Was migration ever run on a deployed server? | §8.7 |
| **OPEN-22** | `databaseInstaller.go` (432 lines — all the DDL and seed data) is never called. **Nothing else creates the schema.** Was it meant to run during install? | §10.6 |
| **OPEN-26** | `/health` and `/installation-progress` return hardcoded success strings, so verification always passes. Was a real health check intended? | §11.2 |
| **OPEN-27** | nginx and the captive portal both bind port 8080. Which should own it? | §11.4 |
| **OPEN-36** | §9.10 keys authentication on `/tmp/dvarpala-auth-status-<username>`. `/tmp` is world-writable, so any local account could forge it; it is also cleared on reboot. Should this move to the `user_auth_status` table it already defines? | §9.10 |
| **OPEN-35** | The selective-blocking installer seeds a hardcoded control-plane API key (`dvarpala-default-admin-key-2024`), identical on every install and published in a public repo — stored unhashed in a column named `token_hash`. Placeholder, or intended default? | §9.10 |
| **OPEN-33** | `dvarpala-auth-user` sets mark `0x0`, but `MARK` is non-terminating so the next rule re-marks the packet `0x100` — firewall promotion is a no-op. Was this ever tested on a live host? | §9.5(e) |
| **OPEN-34** | The shipped `.ovpn` embeds `portal`/`access` credentials and auto-opens a captive portal that the server does not run. Should the client profile match the deployed server, or the server match the client? | §9.9 |
| **OPEN-37** | `selective-blocking.sql:16` uses `UNIQUE(...) WHERE is_active = true`, which PostgreSQL rejects in a table constraint — so `blocked_resources` is never created, and `psql` reports it only to a log nobody reads. Should this become a `CREATE UNIQUE INDEX`, and should the installer set `ON_ERROR_STOP`? | §9.10 |
| **OPEN-29** | The branch added `*.sum` to `.gitignore`, deleting both `go.sum` lockfiles — removing supply-chain verification and breaking every build. Deliberate? | §12.2 |
| **OPEN-5** | Has the Go server ever been run from this repo? `make build` and `make dev` both reference non-existent paths. | §6.5 |

### Housekeeping

| # | Question | § |
|---|---|---|
| **OPEN-2** | Is `dvarpala-context.md` (the Flask-era doc) safe to delete? It describes a stack that does not exist and will mislead newcomers. | §2 |
| **OPEN-21** | `resource-management-api.go` (576 lines) has never compiled — `gorilla/mux` missing from `go.mod`. The branch adds it, so: unfinished rather than abandoned. Is it wanted? | §10.5 |
| **OPEN-24** | Resource-name suffixes come from `time.Now().Unix() % 100000`, not randomness. Does anything depend on their uniqueness? | §10.4 |

---

*This document describes `cff0373` as of August 2026.
Sections most likely to need a developer's correction: §9.5(a), §9.5(e), §9.7,
§9.10, §11.4 — each marked `[INFERRED]` and resolvable with one test deployment.*
