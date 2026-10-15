# GNNwatchtower Security Re-audit

Date: 2026-10-15 (Europe/Istanbul). Application version: 0.4.3.

Target: the current local working-tree files. The local `main` ref contains `e81a31627c52b7c0ed05638534b9438d24eb3054`; this is a reference marker, not a claim that the working tree is clean or identical to that commit. Git status could not run because the configured macOS Command Line Tools installation is missing.

Scope: Go API/authentication/database, Linux and Windows agents, monitoring/discovery/notifications/backups, Vue source, dependency manifests, Docker and installation/update scripts. This is a report-only static review. No application source, dependency manifest, configuration, or previous audit was edited. No live appliance was contacted, enrollment attempted, alert sent, exploit executed, or application test/build run. The report follows the English documentation requirement in `.agents/AGENTS.md`.

## Assessment

The previous remediation work introduced useful controls, but several security outcomes remain open. The most urgent defect is that public installation scripts expose the new shared enrollment secret. A network client can retrieve that secret and replace an existing device's agent token. Default HTTP communication also permits an on-path attacker to supply privileged agent updates, and the initial administrator account remains claimable using its known password.

The review records **3 High, 8 Medium, and 1 Low** findings. These are contextual severity assessments, not calculated CVSS scores. Deployment-dependent findings state their prerequisites; none is presented as a dynamically reproduced exploit.

| ID | Severity | Finding | Required access or condition |
|---|---|---|---|
| R01 | High | Public installation scripts disclose the fleet enrollment key | Network access; a device identifier for takeover |
| R02 | High | Known bootstrap password permits first-claim administrator takeover | Access before legitimate setup completes |
| R03 | High | Default HTTP permits replacement of privileged agent updates | Ability to alter agent HTTP traffic |
| R04 | Medium | Forwarded headers bypass the login attempt limiter | Direct backend access or unsanitized proxy headers |
| R05 | Medium | Long passwords silently downgrade from bcrypt to SHA-256 | Affected password plus later hash disclosure |
| R06 | Medium | Enrollment accepts URLs that become server-side monitoring requests | Enrollment key, obtainable through R01 |
| R07 | Medium | Stored settings become executable installer syntax | Settings-write access and subsequent elevated installation |
| R08 | Medium | Password change/logout do not revoke existing sessions | Previously obtained bearer token |
| R09 | Medium | Large CIDRs can exhaust memory before scanner limits apply | Operator session; accidental input also sufficient |
| R10 | Medium | Agent text is inserted unescaped into alert email HTML | Agent token and enabled Brevo alerts |
| R11 | Medium | Windows secret files rely on inherited ACLs | Local user allowed to read the installation directory |
| R12 | Low | Secret-bearing backups have permissive default file modes | Local filesystem access in an unprotected deployment |

## Comparison with the previous report

Baseline: [security-audit-2026-10-15.md](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/.memory-bank/audits/security-audit-2026-10-15.md).

| Previous finding | Observed improvement | Current disposition |
|---|---|---|
| 1. Unauthenticated enrollment/heartbeat | Missing or invalid heartbeat tokens now receive 401; a shared enrollment key is checked | Partially addressed. Public installer disclosure defeats enrollment protection; R01 |
| 2. Default administrator credentials | Password change is enforced by middleware and the UI, with an eight-byte backend minimum | Partially addressed. Anyone knowing `admin/admin` can claim initial setup; R02 |
| 3. HTTP transport | Native HTTPS can be enabled with TLS environment variables | Partially addressed. HTTP remains the default; R03. Ordinary Go HTTPS clients do verify certificates |
| 4. Fast password hashing | bcrypt cost 12 is used normally; legacy hashes migrate on successful login | Partially addressed. bcrypt errors fall back to the old hash; R05. Unused legacy accounts remain unmigrated |
| 5. SSRF | Website checks now reject selected metadata/link-local destinations and inspect redirects | Partially addressed. Enrollment-to-monitoring boundary remains open, and enforcement is inconsistent; R06 |
| 6. Missing request/rate limits | API body ceiling is 2 MiB; login/enrollment/report have smaller caps; login lockout exists | Body-size issue addressed in inspected routes. Login limiter is bypassable; R04. Valid-agent reporting still has no frequency quota |

The historical report `audit-dda0140.md` is not evidence of current security. Its claims of mandatory update checksums, strict origin pinning, and globally protected secrets are not supported by the current code. A SQLite pool size greater than one is not, by itself, proof of a security vulnerability or database corruption; no such finding is carried forward here.

## Detailed findings

### R01 — Public installation scripts disclose the fleet enrollment key

**Severity: High. Confidence: high from source.**

Both installation routes are registered before `AuthMiddleware`. Their handlers read `agent_enrollment_key` from settings and embed its full value in the returned bash/PowerShell command. An unauthenticated caller can obtain it and satisfy the new enrollment check. Enrollment then matches a caller-supplied UUID, IP, or hostname against existing devices, overwrites the selected device's token, and returns the replacement token. Knowledge of one identifier is sufficient for device impersonation; new unauthorized enrollment does not require an existing identifier.

This permits telemetry/status/incident manipulation and invalidates the genuine device's credential. Removing the old heartbeat fallback was correct, but does not close this alternate path. The enrollment check also fails open if the stored key is empty or its query fails.

Evidence: [public routes](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/api/router.go:50), [Linux secret interpolation](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/api/handlers.go:1248), [Windows secret interpolation](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/api/handlers.go:1317), [key validation and re-enrollment](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/api/handlers.go:873).

Remediation: require authenticated authorization for secret-bearing installer generation; use expiring, device-scoped provisioning credentials; require the current device credential or explicit operator approval before replacing an enrolled identity. Fail closed on absent enrollment configuration. After closing disclosure paths, rotate the shared key and credentials for potentially affected devices.

### R02 — Known bootstrap password permits first-claim administrator takeover

**Severity: High. Confidence: high from source.**

An empty user database still receives `admin/admin`. The new mandatory-change control blocks general APIs, but permits the password-change endpoint. An attacker reaching a fresh appliance before its operator can log in with the public default, set an attacker-selected password, and obtain an unrestricted administrator session. The same risk applies to an existing installation that still uses the default password. Once a legitimate unique password has been established, this specific bootstrap path no longer applies.

Evidence: [account initialization](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/database/db.go:227), [password change and session unlock](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/api/auth.go:211), [mandatory-change exception](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/api/auth.go:270).

Remediation: require a unique bootstrap secret provisioned locally, an administrator-supplied initial credential, or a setup flow restricted to a locally authenticated operator. Mandatory password change should supplement that identity check.

### R03 — Default HTTP permits replacement of privileged agent updates

**Severity: High. Confidence: high from source; requires an on-path attacker.**

HTTPS is now available, but the server uses HTTP unless TLS settings are supplied; the shipped installer and README continue to use HTTP. The agent trusts an upgrade instruction returned by its heartbeat request, downloads an executable, and replaces/restarts itself. An attacker able to modify HTTP traffic can insert an upgrade instruction and substitute the executable without waiting for a legitimate queued update. The checksum comes through the same unauthenticated channel, so it can also be replaced. An empty checksum skips validation entirely.

The URL guard compares a string prefix instead of parsed origins, and the download client follows redirects. These weaken the stated appliance-pinning guarantee. The installed Linux service runs without a non-root `User` setting; the Windows service uses its default service identity, normally LocalSystem. Successful replacement therefore inherits those privileges.

Evidence: [optional TLS/default HTTP](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/cmd/server/main.go:168), [documented agent installation](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/README.md:85), [instruction handling](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/cmd/agent/main.go:478), [download-origin check](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/cmd/agent/main.go:516), [optional checksum and replacement](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/cmd/agent/main.go:574), [Linux service](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/cmd/agent/service_nonwindows.go:108), [Windows service](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/cmd/agent/service_windows.go:212).

Correction to the prior report: the agent uses normal Go HTTP clients and does not set `InsecureSkipVerify`. With a properly trusted HTTPS URL, standard certificate verification is present. The defect is insecure defaults and update authenticity, not universal certificate-verification bypass.

A related deployment gap remains: generated scripts infer HTTPS only from `Request.TLS`. A TLS-terminating proxy with an HTTP backend therefore generates HTTP agent URLs. Evidence: [script origin construction](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/api/handlers.go:1240).

Remediation: require verified HTTPS for provisioning, heartbeat and updates; use a canonical configured external HTTPS URL; reject missing/malformed digests; authenticate update manifests with a provisioned signing key. Compare parsed scheme/host/port and validate every redirect. Provision CA trust for self-signed deployments rather than disabling verification.

### R04 — Forwarded headers bypass the login attempt limiter

**Severity: Medium. Confidence: high from source and pinned dependency behavior.**

The login limiter indexes attempts by `c.ClientIP()`. Router initialization never restricts trusted proxies. Gin v1.9.1 trusts forwarding headers from all peers by default, so a directly connected client can rotate a valid `X-Forwarded-For`/`X-Real-IP` value to obtain a fresh attempt bucket. The client can also submit attempts attributed to another address. The risk is reduced when an inaccessible backend sits behind a proxy that overwrites these headers, but that is not enforced by the application.

Evidence: [limiter key/check](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/api/auth.go:65), [failed-attempt update](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/api/auth.go:105), [router initialization](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/api/router.go:25). Dependency semantics were checked against [Gin v1.9.1 source](https://raw.githubusercontent.com/gin-gonic/gin/v1.9.1/gin.go).

Remediation: disable forwarded-header trust in direct deployments or configure only actual proxy CIDRs. Add account/global limits alongside source-IP limits and bound concurrent expensive password verification.

### R05 — Long passwords silently downgrade from bcrypt to SHA-256

**Severity: Medium. Confidence: high from source and pinned dependency behavior.**

`HashPassword` attempts bcrypt cost 12 but returns the legacy fixed-salt SHA-256 value on any bcrypt error. The change-password handler checks only the minimum length. Go bcrypt rejects passwords longer than 72 bytes; a long passphrase, including a shorter character count with multibyte text, therefore receives weaker storage without a warning. Login migration repeats the same fallback for affected legacy passwords. Exploitation requires later access to the stored hash and a guessable password; long random passwords are not assumed to be easily crackable.

Evidence: [fallback](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/database/db.go:248), [password validation](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/api/auth.go:194), [legacy migration](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/api/auth.go:127). The 72-byte limit is documented in [Go bcrypt](https://pkg.go.dev/golang.org/x/crypto/bcrypt#GenerateFromPassword) and present in the cached v0.23.0 source.

Remediation: return hashing errors and never generate a fast fallback hash. Clearly reject unsupported byte lengths or use a suitable password KDF supporting the intended input length. Keep legacy verification only for migration, and establish a policy for dormant legacy accounts.

### R06 — Enrollment URLs become server-side monitoring requests

**Severity: Medium. Confidence: high for the request path; response-body disclosure is not established.**

Enrollment's `local_ip` field is not validated as an IP literal. A value containing an HTTP URL passes the raw loopback-string checks and is saved as the device address. The poller classifies any address starting with HTTP(S) as a website and repeatedly sends GET requests, even though enrollment assigned a Linux/Windows device type. A network attacker can combine this with R01 to make the appliance contact loopback or other internal services. A legitimate but compromised enrollment credential remains sufficient after public disclosure is fixed.

The existing metadata guard is a useful improvement but is incomplete: DNS is resolved once for validation and again by the default transport; the manually stripped colon can corrupt an already parsed IPv6 hostname; and HTTP service checks use a separate client without the website guard. These are residual paths requiring connection-time policy enforcement. No metadata credential exfiltration was demonstrated; website response bodies are discarded. Intended administrator-configured private-network monitoring is not itself classified as a vulnerability.

Evidence: [unvalidated enrollment address](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/api/handlers.go:887), [persistence](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/api/handlers.go:954), [HTTP classification](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/monitor/poller.go:132), [host validation and HTTP request](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/monitor/poller.go:191), [separate service client](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/monitor/poller.go:385).

Remediation: accept only validated IP literals in agent enrollment; reserve URL-monitoring configuration for authenticated operators. Apply one outbound policy to all request paths and redirects, validate the actual dialed address, and preserve explicitly authorized private-network monitoring.

### R07 — Stored enrollment settings become executable installer syntax

**Severity: Medium. Confidence: high from source; requires settings access and subsequent operator execution.**

`PUT /api/settings` accepts arbitrary strings for `agent_enrollment_key`. The script generators insert this stored value into double-quoted bash and PowerShell arguments. Double quotes allow command/subexpression expansion; storing executable syntax in the key turns it into code when an endpoint administrator runs the generated installer. The frontend PowerShell command uses the same unsanitized setting. This crosses the boundary from web configuration rights to endpoint root/administrator execution. It is not unauthenticated command execution on the monitoring server.

Evidence: [arbitrary settings values](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/api/handlers.go:706), [bash argument construction](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/api/handlers.go:1253), [bash execution line](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/api/handlers.go:1298), [PowerShell construction](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/api/handlers.go:1322), [frontend command](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/frontend/src/components/AgentModal.vue:179).

Remediation: enforce a fixed enrollment-key alphabet/length and encode all generated literals for the destination shell. Prefer a protected provisioning data file over embedding values in executable syntax. Validate the canonical server origin as well, because the request Host is also interpolated into scripts.

### R08 — Password changes and logout do not revoke existing sessions

**Severity: Medium. Confidence: high from source.**

Sessions last seven days. Changing a password updates the database hash and clears only the current session's mandatory-change flag; other sessions remain in memory. Logout only removes the browser's local storage value. Middleware checks the in-memory token and expiration without a credential-version or revocation check. A previously stolen bearer token can remain useful after the operator changes their password or logs out, until expiration or process restart.

Evidence: [session lifetime](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/api/auth.go:144), [password-change session handling](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/api/auth.go:217), [session acceptance](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/api/auth.go:255), [client-only logout](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/frontend/src/services/api.js:55).

Remediation: revoke account sessions on password change, implement server-side logout, and rotate any session intentionally retained after credential changes.

### R09 — Large CIDRs can exhaust memory before scanner limits apply

**Severity: Medium. Confidence: high from source.**

The authenticated discovery handler forwards a parsed but otherwise unrestricted CIDR to the scanner. The scanner allocates a slice entry and IP copy for every address before applying its 64-worker concurrency bound. A sufficiently large IPv4 range or an IPv6 /64 can exhaust memory before network probing begins. Request cancellation is not passed into that expansion. This is both an authenticated denial-of-service path and an accidental operator-input risk; the worker cap does not bound the preceding allocation.

Evidence: [discovery handler](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/api/handlers.go:458), [unbounded expansion](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/discovery/scanner.go:18), [later worker limit](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/discovery/scanner.go:43).

Remediation: calculate and reject excessive host counts before allocation, validate supported address families and permitted scan scope, iterate lazily, limit simultaneous jobs, and honor cancellation.

### R10 — Agent text is inserted unescaped into alert email HTML

**Severity: Medium. Confidence: high from source; requires enabled email alerts.**

A valid agent can supply its shutdown reason. That text is passed to the alert dispatcher and interpolated directly into an HTML email with `fmt.Sprintf`. Device names and other fields are also interpolated. An agent can therefore place misleading HTML links/images or content into messages sent by the trusted monitoring sender. R01 makes obtaining an agent token easier, but this remains a separate trust-boundary issue for compromised legitimate agents. Browser JavaScript execution in email clients is not claimed.

Evidence: [agent-supplied shutdown detail](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/api/handlers.go:1013), [email dispatch](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/incident/classifier.go:189), [unescaped HTML formatting](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/incident/classifier.go:270).

Remediation: use `html/template` with text values or escape all interpolated fields. Apply context-appropriate escaping to Telegram HTML as well, including the assembled table.

### R11 — Windows secret files rely on inherited ACLs

**Severity: Medium, deployment-dependent. Confidence: high for missing ACL enforcement; actual host exposure unverified.**

The agent saves both `agent.key` and `enroll.key` with mode `0600` under its Program Files directory. No explicit restrictive Windows DACL is established by the reviewed code. Go's Unix-style permission bits do not enforce owner-only Windows reads, so inherited directory access controls decide who can read these credentials. Where local nonadministrator users inherit read access, they can obtain the agent token and the shared enrollment credential. Production Windows ACLs were not inspected.

Evidence: [secret-file writes](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/cmd/agent/main.go:129), [Windows storage paths](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/cmd/agent/service_windows.go:384). Platform semantics: [Go filesystem permission documentation](https://pkg.go.dev/os#Chmod).

Remediation: create and maintain explicit DACLs limited to SYSTEM, Administrators and the intended service identity. Check existing files during upgrades; prefer device-scoped enrollment credentials and remove provisioning secrets when no longer required.

### R12 — Secret-bearing backups have permissive application defaults

**Severity: Low, deployment-dependent. Confidence: high for file modes; actual filesystem exposure unverified.**

Backup exports include all settings and device tokens. The application creates the backup directory with `0755` and automatic JSON backups with `0644`. With traversable parent directories and a normal umask, direct-binary deployments can expose integration credentials and agent keys to other local users. The standard installer mitigates this by creating protected installation/data/backup directories; the finding is not universal and does not imply Docker volumes are remotely accessible.

Evidence: [settings export](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/backup/manager.go:43), [device-token export](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/backup/manager.go:72), [backup permissions](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/backup/manager.go:255), [installer protection](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/install.sh:30).

Remediation: enforce restrictive backup directory/file permissions in the application, including existing paths, and handle permission/write failures. Treat exported JSON as a credential-bearing artifact.

## Dependency advisory spot check

Actual lockfile versions were read rather than inferred from package.json ranges: Axios 1.20.0, Vue 3.5.43, Vite 5.4.21, esbuild 0.21.5, Rollup 4.64.5, PostCSS 8.5.29. This was a selected primary-source advisory check, not a complete SCA scan, SBOM review, or Go call-graph vulnerability analysis.

| Component | Verified advisory | Applicability to this repository |
|---|---|---|
| Vite 5.4.21 | [GHSA-fx2h-pf6j-xcff](https://github.com/vitejs/vite/security/advisories/GHSA-fx2h-pf6j-xcff), published 2026-06-01; affected range includes <=6.4.2 | Conditional development-server file disclosure on Windows/NTFS or short-name configurations when explicitly exposed to the network. The inspected configuration does not explicitly expose `server.host`; the production embedded SPA does not run Vite |
| Vite 5.4.21 | [GHSA-rq7h-c2jc-7f22](https://github.com/vitejs/vite/security/advisories/GHSA-rq7h-c2jc-7f22), published 2026-10-06; affected range includes <=6.4.3 | Requires an exposed non-Windows dev server and a particular imported-path collision. The required collision was not established; affected-version match is not proof of application exploitability |
| esbuild 0.21.5 | [GHSA-67mh-4wv8-2f99](https://github.com/evanw/esbuild/security/advisories/GHSA-67mh-4wv8-2f99); affected <=0.24.2, patched >=0.25.0 | Concerns esbuild's own development server. No invocation of that server was identified in repository source/scripts; not counted as a confirmed production vulnerability |

Vite advisories [GHSA-p9ff-h696-f583](https://github.com/vitejs/vite/security/advisories/GHSA-p9ff-h696-f583) and [GHSA-v2wj-q39q-566r](https://github.com/vitejs/vite/security/advisories/GHSA-v2wj-q39q-566r) were checked and their stated version ranges do not include locked Vite 5.4.21. They were not counted against it. Upgrade development tooling to a supported patched line after compatibility review; the October advisory lists 6.4.4 as a patched release on that branch.

## Additional observations and explicit exclusions

- Heartbeat authentication now rejects missing/unknown tokens. Valid token holders can still submit unlimited-frequency metrics; request-size caps do not bound total database growth. Add per-device frequency limits and retention controls. This was not load-tested.
- SQL parameter binding is used in the inspected API paths. No confirmed SQL-injection path was found. Vue source predominantly uses escaped template bindings; no confirmed DOM-XSS exploit was established. These observations are not a comprehensive proof of absence.
- The `JWT_SECRET` configuration is not consumed by the current opaque in-memory session implementation. Its fixed Compose fallback is therefore not evidence of JWT forgery.
- Root service privileges, container capabilities, mutable installation branches and build-tool versions merit hardening, but no additional standalone exploit or container escape is asserted from those facts alone.
- `.gitignore` does not establish that secrets are absent from history. Git history/index and live secret stores were not audited; no historical-secret clearance is claimed.

## Evidence and validation limits

Source analysis was independently divided across API authentication, agent distribution, and monitoring/data paths, then reconciled against current code. Existing auth, database, monitoring and notification tests were inspected. They were not executed: Go/npm/scanner executables were not available on the shell PATH, and no dependencies or toolchains were installed. In particular, current tests do not establish protection against public enrollment-key disclosure, proxy-header rotation, bcrypt failure fallback, stale sessions, or oversized CIDRs.

External requests were limited to public maintainer documentation and advisories; private source, credentials and runtime data were not uploaded. Live reverse-proxy settings, Windows ACLs, filesystem ownership, container images, compiled binaries and externally deployed behavior remain unverified.

A content fingerprint was recorded for 61 source, configuration, manifest and frontend artifact files and compared again after report creation. The source-manifest SHA-256 is `473d7217f4679da5ec84c435f32b7d02fddfd52cb6eb71c360fe7626e8a04615` (sorted relative-path/hash pairs encoded as JSON). This identifies the inspected local snapshot and supports checking for concurrent file changes; it is not a Git-cleanliness assertion.

## Recommended order

1. Close enrollment-secret disclosure and uncontrolled re-enrollment; replace shared long-lived provisioning credentials and rotate exposed secrets.
2. Protect initial administrator setup and require authenticated transport/update provenance.
3. Fix proxy trust, hashing error handling, installer literal encoding, session revocation and enrollment input validation.
4. Bound discovery/ingestion resources, escape alert content, enforce secret-file permissions, and review development dependencies.

No remediation was applied during this audit.
