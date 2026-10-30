# Security audit — 2026-10-30

## Scope and method

Report-only static review of the local working tree. Branch reference read directly from `.git/refs/heads/main`: `9e507b1b4988cd66992fee81207cef72be4865ac`. This identifies the branch tip, not proof that every local file matches that commit. No source changes, builds, tests, live customer probes, deployment, staging, commit or push were performed. No customer identifiers or secret values are included in this report.

Reviewed API routing/authentication, enrollment and heartbeat authorization, update trust, web probes, SQLite initialization, notification dispatch, install scripts and container configuration. Severity reflects plausible deployment conditions, not confirmed exploitation. No dependency/CVE scan, complete repository secret/history scan, runtime isolation assessment or penetration test was performed. No claim of overall security or absence of other critical/high vulnerabilities is made.

## Findings

| ID | Severity | Finding |
| --- | --- | --- |
| S01 | High | Predictable privileged bootstrap account |
| S02 | High | Optional transport protection and untrusted initial update trust |
| S03 | Medium | DNS validation is not bound to the web probe connection |
| S04 | Medium | Excessive privileges and broad default network exposure |
| S05 | Medium | Unbounded persistent username failure tracking |
| S06 | Low | SQLite connection-specific settings not applied to every connection |
| S07 | Low | Notification dispatch lacks bounded queue/cooldown and 429 backoff |

### S01 — Predictable privileged bootstrap account

Evidence: `backend/internal/database/db.go:230–257` initializes the operator with `admin/admin` and restores that password on startup while mandatory password change remains outstanding. `backend/internal/api/auth.go`, `Login` and `ChangePassword`, permits the holder of this known current password to select a new password. `AuthMiddleware` restricts pre-change access, but does not establish the identity of the first operator.

Condition/impact: an attacker able to reach a fresh or unfinished installation can win the onboarding race, set their own password, and obtain normal operator access. Five-attempt throttling does not mitigate a known correct password. This is not an authentication bypass on an already initialized deployment with a changed password.

Recommendation: locally delivered random one-time bootstrap credential, restricted onboarding exposure and no predictable-password restoration. Validate the full first-run/restart path before changing installation behavior.

### S02 — Optional transport protection and untrusted initial update trust

Evidence: `backend/cmd/server/main.go`, HTTP/HTTPS startup, binds all interfaces and serves HTTP unless TLS is configured. `backend/cmd/agent/main.go:355` enforces HTTPS only through the optional flag or `REQUIRE_HTTPS`. `autoEnroll` sends enrollment credentials and accepts the returned token/public key. At lines 298 onward it saves the returned signing key without comparing an existing pin. `performUpgrade:656–668` also obtains a missing pin from the instruction or signer endpoint. SHA-256 and Ed25519 checks are present, but the first trusted public key can arrive through the same unauthenticated transport. Generated installation scripts also download executable content using the derived server URL.

Condition/impact: on a deployment using HTTP, a network observer can capture bearer/enrollment credentials. An active intermediary during bootstrap or key recovery can substitute the initial signing key and sign a malicious replacement executable; impact inherits agent service privileges. Re-enrollment key replacement also warrants explicit rotation authentication. Existing correctly pinned keys protect normal signed update verification; this does not protect plaintext credential transport. A publicly available *public* key is not itself a vulnerability.

Recommendation: authenticated TLS with certificate validation for the complete provisioning/report/update lifecycle; convey the signing pin through an independently authenticated installation path, reject unexpected replacements, and make rotation explicit. Do not bypass certificate validation or open management interfaces to implement this.

### S03 — DNS validation is not bound to the web probe connection

Evidence: `backend/internal/monitor/poller.go`, `isDisallowedHostOrIP` resolves a hostname with `net.LookupIP`, while `checkWebsiteReachability` then uses a separate default HTTP transport resolution. Lookup failure currently returns false (not disallowed). Redirects recheck destination names but also use separate resolution. Link-local, multicast and unspecified destinations are filtered, but there is no deployment-specific destination/port allowlist or validated-address dialer.

Condition/impact: a malicious DNS authority for a configured web target can return a permitted address during validation and a forbidden destination during connection. This may bypass intended metadata/link-local restrictions. Creating a target ordinarily requires operator access; compromise of DNS for an existing target also matters. Exploit success depends on resolver timing, destination protocol and available responses; arbitrary internal compromise or metadata credential theft was not demonstrated. Internal IP monitoring is intentional, so simply banning all private ranges is inappropriate.

Recommendation: resolve and validate immediately in a dedicated dial path, connect only to those approved addresses, retain hostname TLS verification, and apply the same policy to redirects and proxies. Define customer-specific authorized destinations without widening network access.

### S04 — Excessive privileges and broad default network exposure

Evidence: `docker-compose.yml:16–23` grants `NET_ADMIN` and `NET_RAW`; `Dockerfile` has no non-root `USER`. `install.sh:139` configures `User=root`, and lines 161–162 add an unrestricted UFW allow rule for TCP 3800 when UFW exists. Compose publishes `3800:3800`, and the server binds `:port`.

Impact: application compromise inherits avoidable local/network administration capabilities. Installation may expand panel reachability beyond the operator's intended collector sources. Existing network segmentation may reduce exposure; running as root is an impact multiplier, not proof of remote code execution.

Recommendation: remove unneeded network administration capability, scope binding and existing access policy to authorized sources, and migrate to a dedicated account after checking filesystem, ping and update requirements. Do not add new capabilities or firewall exceptions just to make telemetry work. These defaults conflict with the governing no-additional-attack-surface policy.

### S05 — Unbounded persistent username failure tracking

Evidence: `backend/internal/api/auth.go`, `Login`, creates `userAttempts[input.Username]` for every failed username. Hourly cleanup in `init` removes entries only from `loginAttempts`, not `userAttempts`. Body size is bounded, but username length and total username map cardinality are not explicitly bounded. Session map creation is also not capped, although sessions do expire.

Condition/impact: unauthenticated callers using multiple source addresses or spreading attempts over IP lockout windows can continually add distinct usernames; these entries do not expire from the map. Memory exhaustion is plausible over sustained use. IP lockouts slow a single source, so immediate exhaustion from a handful of requests is not claimed.

Recommendation: bounded expiring rate-limit state for both IP and username, explicit username length bounds, and bounded concurrent expensive login processing. Preserve username-based protection against source rotation.

### S06 — SQLite connection settings are not guaranteed across the pool

Evidence: `backend/internal/database/db.go:40–63` executes `busy_timeout`, `foreign_keys` and other PRAGMAs through `db.Exec` before configuring a 25-connection pool with connection recycling. Connection-local settings are not guaranteed on subsequently opened connections. WAL journal mode is persistent and should not be confused with those settings.

Impact: inconsistent lock wait or foreign-key enforcement can cause availability/integrity issues under concurrency. Pool size alone does not prove corruption or constitute a remotely exploitable vulnerability.

Recommendation: initialize required PRAGMAs for every connection using supported driver configuration. Assess write coordination separately. Do not blindly reduce the whole pool to one connection without checking nested queries/transactions for deadlocks.

### S07 — Notification resource limits and retry policy

Evidence: `backend/internal/incident/classifier.go`, `DispatchAlert:306–375`, starts per-channel goroutines for every dispatch. The reviewed notification implementation has no `Retry-After`/429 retry path, bounded dispatch queue, or general three-minute repeated-alert cooldown. ICMP's three scheduled failure confirmation is present in `poller.go`; it is not an outbound notification rate limiter.

Impact: repeated incidents can create alert fatigue and downstream throttling, with dropped/delayed notifications. Sustained event volume can grow in-flight work. This is an availability/operational finding; it does not establish credential compromise.

Recommendation: bounded dispatch, explicit per-device/channel suppression preserving incident records and recovery semantics, and bounded retries respecting provider responses. Do not add an unbounded retry loop.

## Controls observed and limits

- Protected operator API group uses bearer sessions and mandatory bootstrap password change. Enrollment requires a fleet key for a new device; registered agents require their existing device token for replacement. No unauthenticated registered-agent takeover was established by this review.
- Password verification uses the database password helper, and parameterized queries appear in the examined authentication paths. This is not an exhaustive SQL injection review.
- Signed agent updates require a checksum and Ed25519 verification. Trust bootstrap remains the separate S02 concern.
- Registered ICMP inventory has target validation, one OS ping per scheduled check, a 60-second minimum cadence, three completed negative observations before an incident, and no TCP/ARP scanning fallback. Local command failures are unknown, not proof of device power loss. No new NAS/ESXi management protocol was observed in this path.
- The targeted frontend sink search found no `v-html`, `innerHTML`, `insertAdjacentHTML` or `eval(` matches in `frontend/src`; this does not certify absence of all XSS.
- Token generation uses 24 bytes for operator sessions and 32 bytes for enrollment tokens in the reviewed code. The prior audit's claim of 48-byte agent tokens and constant-time comparison is unsupported by these paths: enrollment uses string equality, heartbeat uses a parameterized database lookup. Ignored random-read errors are additional hardening debt, not proof of predictable tokens in normal operation.
- The static `JWT_SECRET` Compose default is apparently unrelated to this in-memory session implementation; it is not evidence of forgeable JWT authentication.

## Priority

Address S01 and S02 before exposing a fresh deployment to untrusted networks. Then address web egress binding (S03), deployment least privilege/reachability (S04), and bounded authentication state (S05). Treat S06 and S07 as availability hardening. Remediation remains unimplemented in this report-only audit. Live customer configuration, actual TLS termination and effective firewall rules remain unverified.
