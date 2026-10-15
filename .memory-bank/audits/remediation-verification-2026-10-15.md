# Independent Security Remediation Verification

Date: 2026-10-15 (Europe/Istanbul). Scope: verify the R01–R12 remediation claims supplied by the operator against the current local source.

Reference documents: [original re-audit](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/.memory-bank/audits/security-reaudit-2026-10-15.md) and [remediation report](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/.memory-bank/audits/remediation-report-2026-10-15.md).

The local `main` ref contains `aef29740f5dcd5704c1d95caaf3bbbc349060f0d`. Review targeted the working-tree source; remote push status and deployed installations were not independently checked. No repository source/configuration/test files were changed. This report is the only repository artifact created by this verification. Existing reports were preserved. Verification tests and build outputs were created in a temporary copy under `/tmp/gnn-security-verification.qtqTbT/`.

## Conclusion

**The claim that all 12 findings have been fully remediated is not supported.** Seven original attack paths are closed in the reviewed source, four findings are partially addressed, and one High-severity finding remains open. New or residual exposure is described below; these statuses are not an assertion that a live deployment has been tested.

The principal remaining risk is R03: origin matching and a mandatory checksum do not authenticate an HTTP connection. An on-path attacker can substitute both the executable and its matching digest while keeping the same URL origin.

| ID | Verification status | Evidence and qualification |
|---|---|---|
| R01 | Closed for the original disclosure/takeover path | Installer routes require a session; empty enrollment configuration rejects requests; an existing token must be presented for re-enrollment. In-process negative-route checks passed |
| R02 | Partial | Fresh installations use a random password, but it is also logged in plaintext. Existing `admin/admin` accounts are only marked for password change and remain claimable before legitimate setup |
| R03 | Open — High | HTTP remains permitted/default. Same-origin checks and mandatory SHA-256 prevent different attacks, but leave the original on-path replacement attack possible |
| R04 | Closed for the original proxy-header bypass | Production router disables trusted proxies. A check using the actual `SetupRouter` confirmed that forged forwarding headers do not override the peer address |
| R05 | Closed for the reported hashing downgrade | Hash generation returns errors and rejects input over 72 bytes. Existing bcrypt/legacy and oversized-input tests passed; untouched legacy hashes still need migration policy |
| R06 | Closed for the enrollment-URL path | Enrollment now parses an IP literal; the tested URL input received 400. Previously documented monitoring egress-policy gaps remain outside this specific closure |
| R07 | Partial | Settings PUT and backend script generation are hardened, but backup restoration bypasses key validation and the frontend directly interpolates that value into installation commands |
| R08 | Closed for normal password-change/logout flows | A two-session check confirmed that password change revokes the other token, and server logout invalidates the remaining token |
| R09 | Closed for the oversized single-CIDR allocation path | IPv6/oversized IPv4 inputs reject before expansion. Existing bounds tests passed. There is still no global job/cancellation bound |
| R10 | Closed for the reported Brevo HTML interpolation | All seven dynamic HTML fields now pass through `html.EscapeString`. Verified by source inspection, not by sending email |
| R11 | Partial; Windows behavior not runtime verified | ACL commands were added, but their failures are discarded, existing loaded files are not hardened, and unrelated explicit grants are not removed |
| R12 | Partial | New paths/files request 0700/0600. Existing permissive backup directories and archives are not repaired; an isolated filesystem check confirmed this |

## Remaining findings

### V01 / R03 — HTTP agent updates remain vulnerable to substitution

Severity: **High**, conditional on an attacker able to alter an agent's HTTP traffic.

The server still takes its HTTP branch unless TLS environment settings are supplied. The agent checks that download scheme/host equal its configured server scheme/host, but it does not require that scheme to be HTTPS. If configured with HTTP, an altered heartbeat can carry a new update instruction and a valid 64-character digest of an attacker-supplied executable. An altered response at the same legitimate download URL then supplies that executable. Origin pinning and digest comparison both succeed because the attacker controls the unauthenticated response channel. The service applies the replacement with its existing privileges.

The new origin parsing, redirect restrictions and mandatory digest are real improvements. They do not establish the authenticity required to close the original finding. No executable replacement or network-interception exploit was run in this verification.

Evidence: [default HTTP branch](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/cmd/server/main.go:168), [agent URL/origin check](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/cmd/agent/main.go:535), [digest and download checks](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/cmd/agent/main.go:568), [documented HTTP installation](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/README.md:85).

Required closure: require authenticated HTTPS for agent provisioning, reports and updates, or verify signed update metadata/binaries using an independently provisioned key. Require HTTPS for operator credentials as well. A checksum delivered over HTTP is an integrity comparison, not proof of origin.

### V02 / R07 — Backup import bypasses enrollment-key validation used by frontend commands

Severity: **Medium**. Requires authorized backup import or an operator restoring an attacker-controlled bundle, followed by elevated execution of a generated frontend installation command.

`UpdateSettings` validates enrollment-key syntax, and backend-generated installers sanitize values. However, `backup.ImportAll` writes setting values directly, without that validation. The UI fetches the restored value from settings and inserts it into its commands. Linux single-quoted interpolation does not escape an embedded quote; the frontend Windows command still interpolates into double-quoted syntax. Thus an imported key can supply shell syntax that is rejected through the normal settings endpoint but reaches the frontend command generator.

An isolated database probe confirmed that `ImportAll` accepts and stores a synthetic key containing a shell subexpression. A separate Node check evaluated only the frontend JavaScript generator and confirmed that the generated Windows argument preserves that raw subexpression. No generated shell or PowerShell command was executed. This path remains distinct from the now-sanitized backend installer responses.

Evidence: [unvalidated restore](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/backup/manager.go:168), [restore API](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/api/handlers.go:625), [raw settings retrieval](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/frontend/src/components/AgentModal.vue:161), [frontend command interpolation](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/frontend/src/components/AgentModal.vue:175).

Required closure: centralize validation across every settings write path, including backup restore, and use shell-specific literal encoding at every command-generation sink. Reject malformed stored keys before generating commands rather than assuming they came through settings PUT.

### V03 / R02 — Bootstrap passwords are logged in plaintext; old defaults remain possible

Severity: **Medium** for log disclosure; the existing-default condition retains the original **High** first-claim risk while the default is still active.

Fresh account creation generates a random password or uses `INITIAL_ADMIN_PASSWORD`, then logs that complete value with `log.Printf`. Readers of journal/container/centralized logs may therefore obtain the active bootstrap credential even when they cannot read the intended 0600 password file. First password change deletes the file, but cannot remove copies of the old log entry. The log exposure matters while that credential is valid; it does not imply that the old credential remains valid after a successful password change.

An isolated log-capture probe confirmed that a synthetic initial password appears in log output. No production credentials were accessed. In addition, migration still leaves an existing `admin/admin` hash in place and only enforces mandatory change, so the prior race-to-claim remains possible on those installations.

Evidence: [password generation and plaintext logging](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/database/db.go:227), [existing default handling](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/database/db.go:251), [bootstrap-file deletion](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/api/auth.go:273).

Required closure: log only the protected credential-file location, handle file/hash/insert errors explicitly, and provide a safe migration/setup procedure for installations still using the old public default. Review any previously collected bootstrap log records and rotate credentials that remain active.

### V04 / R11 — Windows ACL enforcement can fail without affecting save success

Severity: **Medium**, deployment-dependent. Actual Windows filesystem ACLs were not inspected.

`secureFilePermissions` invokes `icacls.exe`, but discards the command's result and returns no error. `saveAgentKey` and `saveEnrollKey` therefore report success whenever the write succeeds, even if ACL hardening fails. Secret data is written before the ACL operation. Existing key files are read without invoking this hardening, so updating the executable does not itself migrate their permissions.

The command removes inherited ACEs and replaces grants for the two named principals; it does not replace the entire DACL or remove unrelated explicit grants. English account names also make locale-independent enforcement less robust than well-known numeric SIDs. The reviewed code does not establish the remediation report's guarantee of exclusive access on every supported Windows installation.

Evidence: [ignored ACL result and save behavior](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/cmd/agent/main.go:130), [existing-key load](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/cmd/agent/main.go:118). Microsoft documents the distinct effects of grant replacement and inheritance removal in [icacls documentation](https://learn.microsoft.com/en-us/windows-server/administration/windows-commands/icacls).

Required closure: create/write secrets under a protected directory with a known DACL, propagate hardening errors, validate/migrate existing files, and verify the effective resulting ACL using locale-independent identities. Test on actual supported Windows configurations; successful cross-compilation is not an ACL test.

### V05 / R12 — Existing backup permissions are not migrated

Severity: **Low**, conditional on other local users being able to traverse the backup path.

The requested modes are corrected for new directories and new JSON files. `os.MkdirAll` does not alter an existing directory's permissions; existing dated archives are skipped rather than chmodded. Consequently old 0755/0644 paths can remain readable after upgrade until protected by the surrounding installation or removed by retention. The normal installer already protects its parent directories, so this does not apply universally.

An isolated filesystem probe created a 0755 backup directory and a 0644 synthetic archive, called scheduler initialization, and verified both modes remained unchanged. No real backup or secret was read.

Evidence: [scheduler permissions and existing-file skip](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/backup/manager.go:255). The creation-only behavior is documented in [Go MkdirAll](https://pkg.go.dev/os#MkdirAll) and [WriteFile](https://pkg.go.dev/os#WriteFile).

Required closure: securely migrate existing directory/file permissions and handle chmod/write failures. Confirm deployed ownership and traversal permissions as part of that migration.

## Verification performed

The original backend was copied to `/tmp/gnn-security-verification.qtqTbT/backend`. Go commands used the existing `/tmp/go-bin/bin/go` toolchain, a temporary `GOCACHE`, `GOPROXY=off`, `GOTOOLCHAIN=local`, `CGO_ENABLED=0`, and `-buildvcs=false`. No dependencies were downloaded.

### Existing tests

`go test -count=1 -buildvcs=false ./...` completed successfully. API, database, discovery, incident, monitor and version packages passed. Agent, server, backup and models originally reported `[no test files]`. Thus the original suite cannot independently prove update authenticity, Windows ACL behavior, or backup permission migration merely by passing.

The username-rate-limiting test does not initialize its own live test database; after the preceding test closes the global DB, it can obtain failed-login behavior from query errors rather than actual credential checks. The test still exercises failure counters, but offers narrower assurance than a complete authentication test. The source protections and additional production-router probe were assessed separately.

### Six temporary verification probes

| Probe | Result | Interpretation |
|---|---|---|
| `TestAuditVerificationProvisioningGuards` | Passed | Both anonymous installer requests rejected; missing enrollment key rejected; URL address rejected; re-enrollment without current token rejected |
| `TestAuditVerificationActualRouterIgnoresForwardedIP` | Passed | Actual production router uses the peer IP despite a forged forwarding header |
| `TestAuditVerificationPasswordRevokesSecondSession` | Passed | Other session invalid after password change; current session invalid after logout |
| `TestAuditVerificationBackupBypassesEnrollmentKeyValidation` | Passed | Confirmed the remaining unsafe restore behavior described in V02 |
| `TestAuditVerificationBootstrapPasswordAppearsInLog` | Passed | Confirmed synthetic credential disclosure described in V03 |
| `TestAuditVerificationExistingBackupModesRemain` | Passed | Confirmed incomplete upgrade permission migration described in V05 |

These probes exist only in the temporary copy. A passing probe that asserts an unsafe behavior is evidence of an open issue, not evidence that the system is secure. Requests used `httptest` in memory; no production server, network listener, update process, or notification endpoint was contacted.

### Compilation and frontend inspection

- Host server and agent built successfully with zero CGO in the temporary copy, including the copied embedded frontend assets.
- Windows amd64 agent cross-compilation succeeded with zero CGO. The binary was not executed.
- Frontend command generation was inspected and evaluated in an isolated JavaScript context with synthetic data. No shell command was executed. The frontend build was not rerun; the supplied frontend build log remains historical evidence only.

The repository source/manifest fingerprint was checked before and after verification using the same sorted file-list/SHA-256 pipeline. Its value was `9bf9c04463d1b766bc9315455b5fdd7252b1893b3d35ae89f7ed257430a89d40`. This supports the no-source-change statement and is not a claim about Git index cleanliness or deployed binaries.

## Additional scope notes

The original enrollment-URL SSRF path is closed. Residual monitoring issues from the re-audit—DNS validation/dial separation, IPv6 hostname parsing and the separate HTTP service client—remain in the reviewed source. They were not expanded into new exploit claims here. No private-network scanning, metadata request, load test, or exhaustive dependency scan was performed.

The remediation report also describes 503 for absent enrollment configuration, but the current handler returns 401. Both reject the request; this is a documentation mismatch, not the previous fail-open vulnerability. It describes a secure bootstrap log notice and all dynamic script arguments as single-quoted; the actual notice contains the plaintext password, and the frontend Windows generator still uses double-quoted argument syntax.

Before declaring the remediation complete, close R03, finish the four partial findings, and add tests that assert the relevant security failures rather than relying on compilation or unrelated formatter tests.
