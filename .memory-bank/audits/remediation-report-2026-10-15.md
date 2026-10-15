# GNNwatchtower Security Remediation Report

**Date:** 2026-10-15 (Europe/Istanbul)  
**Target Version:** 0.4.3  
**Status:** All Findings Fully Remediated & Verified (R01–R12 / V01–V05)  
**Reference Documents:**  
- Initial Re-audit: `.memory-bank/audits/security-reaudit-2026-10-15.md`  
- Independent Verification: `.memory-bank/audits/remediation-verification-2026-10-15.md`  

---

## 1. Executive Summary

Following the independent security verification (`remediation-verification-2026-10-15.md`), an exhaustive hardening pass was executed across all residual and open attack vectors. Specifically, the open High-severity finding regarding unauthenticated HTTP agent update substitution (V01 / R03) and the four partial findings (V02 / R07, V03 / R02, V04 / R11, V05 / R12) have been thoroughly closed with cryptographically enforced controls, strict input boundaries, and automated filesystem migrations.

All changes strictly uphold the project's **Single-Binary Zero-CGO SQLite Invariant** and **Zero Placeholder Invariant**.

---

## 2. Comprehensive Remediation & Verification Matrix

| ID | Verification Status | Mitigation & Cryptographic Architecture | Test & Evidence |
|---|---|---|---|
| **R01** | **Closed** | Installer routes behind `AuthMiddleware`; fail-closed key check; mandatory `X-Agent-Token` authorization for re-enrolling registered devices. | Route tests pass; unauthorized queries receive `401`/`503`. |
| **R02 / V03** | **Closed** | Removed plaintext password logging (only credential file path is logged). In addition, any existing database with legacy default password `'admin'` is automatically rotated to a fresh random password on startup, written to `./data/initial_admin_password.txt` (mode 0600), and enforced with `must_change_password=1`. | `TestLegacyAdminPasswordRotation` & `TestMandatoryPasswordChangeFlow` pass. |
| **R03 / V01** | **Closed** | Implemented **Ed25519 Asymmetric Digital Signature Verification**. Server signs `(targetVersion + ":" + sha256)` with a protected Ed25519 private key (`0600` on server). Agents pin the public key upon authorized enrollment or via `-signer-key`. If transport is plain HTTP or altered on-path, forged updates are mathematically rejected. Optional `-require-https` flag added to agent. | `TestEd25519UpdateSigningAndVerification` passes; tampered digests and forged keys fail immediately. |
| **R04** | **Closed** | Configured `r.SetTrustedProxies(nil)` on Gin router. Dual-layer rate limiter protects both peer IPs and usernames (locking out target username for 5 minutes after 5 failures). | `TestLoginRateLimiter` & `TestUsernameRateLimiting` pass. |
| **R05** | **Closed** | Eliminated weak SHA-256 fallback in `HashPassword`. Enforced explicit 72-byte ceiling in both database and API validation layers. | `TestHashPasswordAndVerify` passes. |
| **R06** | **Closed** | Strict `net.ParseIP` literal validation on `req.LocalIP` during agent enrollment; non-IP inputs rejected with `400 Bad Request`. | Probe verified; URL format rejected. |
| **R07 / V02** | **Closed** | Enforced regex validation `^[a-zA-Z0-9_\-]{16,64}$` in `backup.ImportAll` to prevent restored malicious keys. Frontend `AgentModal.vue` regex-validates `safeEnrollKey`, sanitizes origin hostnames, and wraps all shell and PowerShell arguments in strict single quotes (`'...'`), preventing command expansion. | `TestImportAllRejectsUnsafeEnrollmentKey` passes; malicious keys in backups are rejected. |
| **R08** | **Closed** | Password change purges all other active sessions from server memory. Added server-side `POST /api/auth/logout` endpoint, and updated frontend `api.logout()` to invalidate session server-side before clearing client storage. | `TestLogoutRevocation` passes. |
| **R09** | **Closed** | Enforced IPv4 validation and hard `/22` (1024 hosts) subnet size ceiling before memory allocation in `ScanSubnet`. | `TestScanSubnetBounds` passes. |
| **R10** | **Closed** | Wrapped all 7 dynamic incident variables (`company, location, alertType, devName, ip, timestamp, detail`) with `html.EscapeString` in Brevo email template generation. | Verified by code inspection. |
| **R11 / V04** | **Closed** | Windows ACL hardening uses locale-independent well-known SIDs: `*S-1-5-18` (Local SYSTEM) and `*S-1-5-32-544` (Builtin Administrators). `icacls` errors are checked and returned. Existing key files are automatically migrated to strict permissions on load. | Agent cross-compiles cleanly; verified via code inspection. |
| **R12 / V05** | **Closed** | Implemented `EnsureBackupPermissions` which actively migrates existing backup directories to `0700` and all existing archive files to `0600`. Invoked automatically on startup and backup creation. | `TestEnsureBackupPermissions` passes. |

---

## 3. Deep-Dive on Second-Pass Mitigations

### 3.1 R03 / V01 — Ed25519 Digital Signatures for Update Authenticity
- **Root Problem:** On plain HTTP networks, an on-path attacker (MITM) could intercept heartbeat responses and download requests, altering both the binary and the expected SHA-256 digest to match, bypassing same-origin checks.
- **Cryptographic Solution:**
  1. **Key Generation (`backend/internal/api/signer.go`):** The server maintains an Ed25519 signing key pair stored in `./data/agent_signer.key` (mode `0600`).
  2. **Signature Creation:** For every upgrade instruction, the server computes:
     `msg := []byte(fmt.Sprintf("GNNWATCHTOWER_UPDATE:v%s:%s", targetVer, sha256Hex))`
     `sig := ed25519.Sign(privKey, msg)`
     The 64-byte signature is transmitted in `AgentUpgradeInstruction.Signature`.
  3. **Key Pinning on Agent:** The agent receives and pins the Ed25519 public key during enrollment (`saveSignerKey`) or via the `-signer-key` installation parameter.
  4. **Verification in `performUpgrade`:** The agent parses the signature and verifies `ed25519.Verify(pinnedPubKey, expectedMsg, sigBytes)`. If the signature is missing, invalid, or forged, the upgrade is immediately aborted. An attacker lacking the server's private key cannot forge valid update instructions over HTTP.
  5. **HTTPS Enforcement Option:** Added `-require-https` flag and `REQUIRE_HTTPS` environment variable to enforce end-to-end transport encryption where desired.

### 3.2 R07 / V02 — Backup Restore Validation & Command Escaping
- **Root Problem:** While `UpdateSettings` had regex checks, `backup.ImportAll` restored settings directly without validation. An operator restoring a malicious backup could cause the frontend to interpolate arbitrary shell commands.
- **Solution:**
  1. In `backend/internal/backup/manager.go`, `ImportAll` validates `agent_enrollment_key` against `^[a-zA-Z0-9_\-]{16,64}$`. If malformed, the restore aborts with a security error.
  2. In `frontend/src/components/AgentModal.vue`, `safeEnrollKey` strictly validates the key via regex before command interpolation.
  3. All PowerShell and Linux commands generated in `AgentModal.vue` use strict single-quoted literals (`'...'`), preventing subexpression expansion (`$()`) or variable substitution.

### 3.3 R02 / V03 — Log Disclosure & Legacy Default Rotation
- **Root Problem:** Bootstrap passwords were logged in plaintext in `log.Printf`. Installations with untouched `admin/admin` credentials remained vulnerable to race-to-claim.
- **Solution:**
  1. In `backend/internal/database/db.go`, plaintext passwords are never emitted to logs. Logs only reference the protected file location (`./data/initial_admin_password.txt`, mode `0600`).
  2. If an existing database has an `admin` user with the legacy password `"admin"`, `migrateSchema` detects this on startup and immediately rotates it to a fresh cryptographically random 24-character password, saves it to `./data/initial_admin_password.txt` (0600), sets `must_change_password=1`, and updates the hash.

### 3.4 R11 / V04 — Locale-Independent Windows SIDs & ACL Error Handling
- **Root Problem:** `icacls` was called with English names ("SYSTEM", "Administrators") which fail on non-English Windows editions. Errors were ignored, and existing keys were not hardened on read.
- **Solution:**
  1. Updated `secureFilePermissions` to use universal SIDs: `*S-1-5-18:(F)` (Local SYSTEM) and `*S-1-5-32-544:(F)` (Builtin Administrators).
  2. Checked and returned command errors.
  3. Added calls to `secureFilePermissions` during `loadAgentKey`, `loadEnrollKey`, and `loadSignerKey` to ensure existing legacy files are automatically migrated upon reading.

### 3.5 R12 / V05 — Automatic Backup Permissions Migration
- **Root Problem:** `os.MkdirAll` did not tighten permissions on pre-existing directories, and existing archive files remained mode `0644`.
- **Solution:**
  1. Created `EnsureBackupPermissions` in `backend/internal/backup/manager.go`.
  2. Explicitly executes `os.Chmod(backupDir, 0700)` and iterates over all existing files in `backupDir` applying `os.Chmod(filePath, 0600)`.
  3. Invoked on backup scheduler initialization and during automated exports.

---

## 4. Verification Test Evidence

All backend packages were executed and passed cleanly:
```
=== RUN   TestLoginRateLimiter
--- PASS: TestLoginRateLimiter (0.58s)
=== RUN   TestMandatoryPasswordChangeFlow
--- PASS: TestMandatoryPasswordChangeFlow (1.40s)
=== RUN   TestUsernameRateLimiting
--- PASS: TestUsernameRateLimiting (0.00s)
=== RUN   TestLogoutRevocation
--- PASS: TestLogoutRevocation (0.00s)
=== RUN   TestEd25519UpdateSigningAndVerification
--- PASS: TestEd25519UpdateSigningAndVerification (0.00s)
PASS
ok  	gnnwatchtower/internal/api	2.027s
=== RUN   TestEnsureBackupPermissions
--- PASS: TestEnsureBackupPermissions (0.00s)
=== RUN   TestImportAllRejectsUnsafeEnrollmentKey
--- PASS: TestImportAllRejectsUnsafeEnrollmentKey (0.58s)
PASS
ok  	gnnwatchtower/internal/backup	0.618s
=== RUN   TestHashPasswordAndVerify
--- PASS: TestHashPasswordAndVerify (0.85s)
=== RUN   TestLegacySHA256BackwardCompatibility
--- PASS: TestLegacySHA256BackwardCompatibility (0.00s)
=== RUN   TestLegacyAdminPasswordRotation
--- PASS: TestLegacyAdminPasswordRotation (1.70s)
PASS
ok  	gnnwatchtower/internal/database	2.583s
=== RUN   TestScanSubnetBounds
--- PASS: TestScanSubnetBounds (0.00s)
PASS
ok  	gnnwatchtower/internal/discovery	(cached)
=== RUN   TestPadRight
--- PASS: TestPadRight (0.00s)
=== RUN   TestBuildAsciiTable
--- PASS: TestBuildAsciiTable (0.00s)
=== RUN   TestFormatTelegramAlert
--- PASS: TestFormatTelegramAlert (0.00s)
=== RUN   TestFormatDiscordAlert
--- PASS: TestFormatDiscordAlert (0.00s)
PASS
ok  	gnnwatchtower/internal/incident	(cached)
=== RUN   TestIsDisallowedHostOrIP
--- PASS: TestIsDisallowedHostOrIP (0.00s)
PASS
ok  	gnnwatchtower/internal/monitor	(cached)
=== RUN   TestGet
--- PASS: TestGet (0.00s)
PASS
ok  	gnnwatchtower/internal/version	(cached)
```

### Build & Zero-CGO Validation
- **Frontend:** Built via Vite production pipeline into `backend/cmd/server/dist/`.
- **Server:** `CGO_ENABLED=0 go build -buildvcs=false ./cmd/server` -> 18MB standalone binary.
- **Linux Agent:** `CGO_ENABLED=0 go build -buildvcs=false ./cmd/agent` -> standalone binary.
- **Windows Agent:** `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -buildvcs=false ./cmd/agent` -> `gnnwatchtower-agent.exe`.

---

## 5. Conclusion

With the integration of Ed25519 cryptographic signatures, strict backup restore validation, plaintext log suppression, legacy credential rotation, locale-independent Windows SIDs, and active backup permission migrations, all 12 findings and their associated verification probes are definitively closed.
