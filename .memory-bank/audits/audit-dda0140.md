# Security Audit Report: dda0140

- **Audited Commit:** `dda0140`
- **Audit Date:** 2026-10-15
- **Audit Scope:** Entire codebase (Go Backend, Windows/Linux Agent, Vue 3 SPA Dashboard, Build & Installation Scripts)
- **Status:** Complete (Report-Only - Zero Source Code Modified)

---

## Vulnerability Dashboard

| Severity | Count | Classes Detected |
|---|---|---|
| **Critical** | 0 | None detected |
| **High** | 0 | None detected |
| **Medium** | 2 | Cryptographic Storage (Fast Hash), SQLite Concurrency Lock Risk |
| **Low** | 2 | Command Argument Sanitization (Flag Injection), Cloud Metadata SSRF Hardening |

---

## Detailed Vulnerability Inventory

### 1. Cryptographic Storage: Fast Password Hashing with Static Salt
- **Severity:** Medium
- **Location:** [`backend/internal/database/db.go#L220-L224`](file:///Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/database/db.go#L220-L224)
- **Description:** The `HashPassword` function applies a single iteration of SHA-256 with a static hardcoded salt (`_gnnwatchtower_salt_2026`). SHA-256 is an algorithm designed for rapid cryptographic hashing, making it vulnerable to high-throughput GPU-accelerated dictionary attacks if the local SQLite database file is accessed or exported.
- **Remediation:** Upgrade password hashing to Argon2id (`golang.org/x/crypto/argon2`) or bcrypt (`golang.org/x/crypto/bcrypt`) with a per-user random cryptographic salt (cost factor >= 12).
- **Code Snippet:**
```go
func HashPassword(password string) string {
	h := sha256.New()
	h.Write([]byte(password + "_gnnwatchtower_salt_2026"))
	return hex.EncodeToString(h.Sum(nil))
}
```

---

### 2. Concurrency: SQLite Connection Pool Exceeds Single-Writer Constraint
- **Severity:** Medium
- **Location:** [`backend/internal/database/db.go#L56-L59`](file:///Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/database/db.go#L56-L59)
- **Description:** Boundary Condition 3 defines a Single-Writer Constraint (`db.SetMaxOpenConns(1)` for writes) for `modernc.org/sqlite`. The database initialization currently configures `db.SetMaxOpenConns(25)`. Although WAL mode (`PRAGMA journal_mode = WAL`) and `busy_timeout = 10000` handle concurrent reads and serialized writes, a large pool of concurrent writers during high-frequency agent reporting spikes could experience `SQLITE_BUSY` contention errors.
- **Remediation:** Implement a synchronized write mutex or maintain a dedicated single-connection writer pool alongside a read-only pool for queries.
- **Code Snippet:**
```go
	// WAL mode enables concurrent readers while writers wait with busy_timeout.
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
```

---

### 3. Input Validation: Unsanitized Host Parameter in Ping Command
- **Severity:** Low
- **Location:** [`backend/internal/monitor/ping.go#L42-L49`](file:///Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/monitor/ping.go#L42-L49)
- **Description:** The target `ip` string is passed directly into `exec.CommandContext(ctx, "ping", ... ip)`. Although `exec.Command` bypasses the shell (preventing traditional shell injection like `; rm -rf`), passing an unvalidated string that begins with a dash (`-`) could trigger command-line flag injection on Linux or Windows ping utilities.
- **Remediation:** Verify that `net.ParseIP(ip) != nil` or ensure the target string does not start with `-` before constructing the ping execution command.
- **Code Snippet:**
```go
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "ping", "-n", "1", "-w", strconv.Itoa(int(timeout.Milliseconds())), ip)
	} else {
		timeoutSec := int(timeout.Seconds())
		cmd = exec.CommandContext(ctx, "ping", "-c", "1", "-W", strconv.Itoa(timeoutSec), ip)
	}
```

---

### 4. Network Security: Unrestricted Cloud Metadata Range in Website Probing
- **Severity:** Low
- **Location:** [`backend/internal/monitor/poller.go#L190-L208`](file:///Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/monitor/poller.go#L190-L208)
- **Description:** The `checkWebsiteReachability` function performs outbound HTTP/HTTPS requests to monitor configured web endpoints. If deployed on cloud environments (AWS, GCP, Azure, DigitalOcean), an administrator could configure `169.254.169.254`, allowing the server to probe the cloud provider's internal metadata endpoint.
- **Remediation:** Add a link-local address filter (`169.254.0.0/16`) in `checkWebsiteReachability` to block outbound HTTP requests to cloud metadata services.
- **Code Snippet:**
```go
func checkWebsiteReachability(targetURL string, timeout time.Duration) (bool, float64, int, string) {
	urlStr := targetURL
	if !strings.HasPrefix(urlStr, "http://") && !strings.HasPrefix(urlStr, "https://") {
		urlStr = "https://" + urlStr
	}
	resp, err := client.Get(urlStr)
```

---

## Project Boundary Conditions Audit

| Boundary Condition | Status | Audit Findings & Verification |
|---|---|---|
| **1. Public Repository Zero-Leak Invariant** | **VERIFIED (PASS)** | All `.env`, `data/`, `*.db`, `*.key`, and `backups/` directories are strictly ignored via `.gitignore`. No credentials or customer IPs are staged or present in Git history. |
| **2. Input Validation and Network Security** | **VERIFIED (PASS)** | Port ranges are validated between 1 and 65535. Discovery IP bounds use standard CIDR parsing. Concurrency limits are in place. |
| **3. Database Concurrency & SQLite WAL Safety** | **PARTIAL (WARNING)** | WAL mode, foreign keys, and synchronous NORMAL are active. However, `db.SetMaxOpenConns` is set to 25 rather than enforcing a dedicated single-writer connection. |
| **4. Authentication, Session & Access Boundaries** | **VERIFIED (PASS)** | All sensitive endpoints are protected behind `AuthMiddleware`. Cryptographically secure 24-byte hex session tokens and 48-byte machine agent tokens are used. Passwords are never logged in plaintext. |
| **5. Alert Egress Throttling & Cooldown Guard** | **VERIFIED (PASS)** | Flapping hosts debounced with cooldown timer. Power outage detection consolidates alerts if >3 devices drop within 60 seconds. Rate limits are handled. |
| **6. Frontend Declarative DOM Construction** | **VERIFIED (PASS)** | Vue 3 templates strictly use declarative data binding. Zero occurrences of `v-html`, `innerHTML`, or raw DOM element injection sinks exist across `frontend/src/`. |
| **7. Data Veracity Contract (Zero Mock Data)** | **VERIFIED (PASS)** | Zero synthetic statistics, fake arrays, or placeholders exist. All metrics, device records, and latencies reflect authentic real-time telemetry from SQLite and OS probes. |

---

## Agent Security & Attack Surface Invariants

1. **Zero Listening Ports:** Confirmed. The agent binary (`backend/cmd/agent`) does not invoke `net.Listen` or open any inbound TCP/UDP ports. Communication is 100% egress-only over outbound HTTPS/HTTP.
2. **Zero Remote Command Execution (RCE):** Confirmed. The agent server instruction handler accepts only the `"upgrade"` action. There are no command-line evaluation, shell execution, or remote script running endpoints.
3. **Strict Appliance Pinning & Integrity Verification:** Confirmed. Upgrade downloads are strictly pinned to the verified appliance host, and SHA-256 checksums are enforced prior to executable replacement.
4. **Token Security:** Agent authentication tokens are stored locally with `0600` permissions and never logged in full plaintext.
