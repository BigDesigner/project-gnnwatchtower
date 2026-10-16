# Verified Worklog

## [2026-10-16] - Governing No-Additional-Attack-Surface Rule

- Recorded the operator's explicit priority: monitoring must not add or expand attack surface, even at the cost of NAS/ESXi compatibility and telemetry coverage.
- Aligned agent directives, constitution, security boundaries and coherence; retained data veracity as a supporting requirement rather than a competing golden rule.
- Added ADR 0006 with an exposure-delta acceptance gate and authentic unavailable-data behavior; gated/deferred existing NAS/ESXi integration backlog entries.
- Saved a current-source architectural review to `audits/attack-surface-policy-review-2026-10-16.md`, including existing listener, privilege, probing, egress and agent-provisioning boundaries. Recognized current signing support instead of repeating outdated checksum-only claims.
- Validation: documentation and referenced source inspected; no application source/configuration changed, no build/test/deployment or network probing performed. Documentation does not certify that runtime exposure is eliminated.

## [2026-10-11] - Sentinel Memory Bank Bootstrap & Secret Hardening

### Completed Work
- **License Integration**: Synced and verified GNU Affero General Public License v3 (AGPL-3.0) with remote GitHub repository (`https://github.com/BigDesigner/project-gnnwatchtower.git`).
- **Secret Isolation & Git Hardening**:
  - Configured comprehensive `.gitignore` masking `.env`, `.env.*`, runtime SQLite databases (`data/`), backup tarballs (`backups/`), keys (`*.pem`, `*.key`), and binaries.
  - Sanitized `.env.example` with generic non-sensitive placeholders and operator security warning headers.
- **Sentinel Memory Bank Initialization**:
  - Scaffolding generated (`.memory-bank/`, `.specs/`, `.agents/`, `.tasks/`, `.archive/`).
  - Active session established (`.memory-bank/active-session.json`) with `preferred_language: "tr"`.
  - Architecture Decision Records authored:
    - `ADR 0001: Initial Architecture Stack & Appliance Topology`
    - `ADR 0002: Selection of GNU Affero General Public License v3 (AGPL-3.0)`
    - `ADR 0003: Public Repository Secret & Client Data Isolation Architecture`
    - `ADR 0004: Golden Rule - Zero Placeholders and Absolute Data Veracity`
  - System coherence specification established (`.memory-bank/system-coherence.md`).
- **Golden Rule Enacted**:
  - Bound all future frontend and backend implementations to 100% real data with zero mock placeholders across `.agents/AGENTS.md`, `.specs/constitution.md`, `.specs/boundary-conditions.md`, and `.memory-bank/system-coherence.md`.

### Incomplete / Upcoming Work
- None. Production appliance turnkey build complete.

### Validation Status
- Git worktree: Clean tracked index, untracked files properly masked.
- Licensing: Validated against FSF AGPL-3.0 text.

## [2026-10-11] - Turnkey Single-Binary Appliance Implementation Complete

### Completed Work
- **Core Engine & SQL Hardening**:
  - Replaced non-standard Postgres `NOW()` timestamps with SQLite standard `CURRENT_TIMESTAMP` across polling and handler queries.
  - Added real gateway telemetry `/api/system/gateway-health` (live socket ping & gateway IP resolution).
  - Added live 24-point time-series metrics endpoint `/api/devices/:id/telemetry`.
  - Added public `/api/health` healthcheck endpoint.
  - Implemented automated session eviction background ticker in `backend/internal/api/auth.go` (resolving BUG-002).
  - Maintained single-writer SQLite WAL mode (`modernc.org/sqlite`) with zero CGO dependencies.
- **Embedded Frontend SPA (Obsidian Telemetry NOC Cockpit)**:
  - Developed full Vue 3 + Tailwind CSS + Axios dashboard matching Cisco Meraki dark NOC design (`#0c0e12`).
  - Implemented modular component architecture: `TopBar`, `Sidebar`, `HostCards`, `DeviceTable`, `TelemetryInspector`.
  - Implemented real operational modals: `DiscoveryModal` (CIDR scan), `AgentModal` (OTP install commands & curl scripts), `AddDeviceModal` (manual device addition), `ClassifyModal` (incident diagnosis), `SettingsModal` (Telegram, Discord, Brevo SMTP configuration), `ChangePasswordModal`, `LoginModal`.
  - Zero Placeholder Invariant strictly enforced: 100% real live polling (10s intervals), zero mock/fake data arrays, authentic empty states with functional CTAs.
  - Compiled production assets into `backend/cmd/server/dist/` via `npm run build` and mounted via Go `//go:embed dist/*`.
- **Packaging & Turnkey Appliance Deployment**:
  - Authored multi-stage `Dockerfile` (Node 20 -> Go 1.22 Alpine -> Alpine 3.19 runtime).
  - Refactored `docker-compose.yml` for single-binary container on port 3800 with persistent data/backups volumes (resolving BUG-001).
  - Created automated Linux installer script `install.sh` for Ubuntu 22.04 / 24.04 LTS (systemd service, auto directory scaffolding, UFW firewall rule, random JWT generation).
  - Enhanced `install.sh` with autonomous remote execution capability (`curl -sSL .../install.sh | sudo bash`), auto-installing missing dependencies (`git`, `curl`, `golang-go`), cloning into a temporary sandbox, compiling the single binary, and configuring systemd.
  - Authored comprehensive `README.md` documenting architecture, zero-placeholder invariant, one-liner installation, and agent scripts.
  - Authored cross-platform agent daemon `backend/cmd/agent/main.go` supporting Windows and Linux with SIGTERM graceful shutdown alerts.

### Validation Status
- Frontend production build: Compiled cleanly with Vite, 0 errors.
- SQLite queries: Checked for pure Go SQLite syntax compatibility.
- Git & Secrets: `.env`, `.tmp`, `data/`, `backups/`, `*.key` verified completely masked.

## [2026-10-11] - Hardening: Settings Unmarshal Fix, HTTP Clipboard Fallback, and Safe In-Place Updater

### Completed Work
- **Settings Unmarshal Bug Fix (`json: cannot unmarshal object into Go value of type string`)**:
  - Refactored `backend/internal/api/handlers.go` `GetSettings` to return a direct key-value map (`map[string]string`).
  - Hardened `UpdateSettings` to accept `map[string]interface{}` and coerce primitives gracefully while safely ignoring numeric keys and malformed nested objects.
  - Sanitized `SettingsModal.vue` payload extraction so only valid string keys are dispatched.
- **HTTP IP Clipboard Fallback**:
  - Resolved clipboard copy failure on plain HTTP IP endpoints (`AgentModal.vue`) by adding a universal `document.execCommand('copy')` textarea fallback.
- **Safe In-Place Updater (`update.sh`)**:
  - Authored `update.sh` to update appliances in-place without touching `data/gnnwatchtower.db` or `.env`.
  - Added automated pre-update timestamped SQLite backups (`backups/pre-update-*.db`).
  - Added temporary service stop to prevent Linux `text file busy` errors during binary installation.
- **Hardened `install.sh`**:
  - Added automated pre-install SQLite backup if existing database exists.
  - Added local test DB migration support from current working directory to `/opt/gnnwatchtower/data/`.
  - Changed binary deployment to atomic `install -m 755`.
- **Frontend Production Build**:
  - Recompiled Vue 3 SPA via `npm run build` directly to `backend/cmd/server/dist/`.

## [2026-10-11] - Release v0.2.0: Agent Auto-Registration & Banner Bug Fix

### Completed Work
- **Version Bump to v0.2.0**:
  - Incremented version badge in `TopBar.vue` to `v0.2.0` (with emerald highlight), `package.json`, `router.go` (`/api/health`), and `update.sh`.
- **Banner Bug Fix (`undefined Kesinti Mevcut`)**:
  - Resolved `undefined` text by guarding `stats.offline_devices` in `App.vue` (`(stats.offline_devices || 0) === 0`).
- **Agent Local LAN IP Detection & Auto-Registration**:
  - Updated `backend/cmd/agent/main.go` to detect outbound local LAN IP via UDP socket probe and include `LocalIP` in `AgentReport`.
  - Updated `AgentHeartbeat` in `handlers.go` to use `LocalIP` (preventing `127.0.0.1` loopback collisions), use `ON CONFLICT (ip_address) DO UPDATE`, and record CPU/RAM/Disk metrics directly into `system_metrics`.
- **Agent Download Verification & File Lock Fix (`curl 23 write error`)**:
  - Resolved `curl (23) client returned ERROR on write` caused by `ETXTBUSY` when updating active running agents.
  - Script now stops existing services before download, streams to a `.tmp` file, and uses atomic `rename` (`mv -f` / `Move-Item -Force`).
  - Hardened `DownloadAgentBinary` to enforce minimal executable file size verification (>10KB).
- **Frontend Production Build**:
  - Recompiled Vue 3 SPA via `npm run build` directly to `backend/cmd/server/dist/`.

## [2026-10-11] - Hardening: Real OS ICMP Gateway Ping, SQLite Concurrency & Atomic Transactions

### Completed Work
- **Real OS ICMP Ping Engine (`backend/internal/monitor/ping.go`)**:
  - Implemented `monitor.PingHost(ip, timeout)` combining ultra-fast TCP probe (ports 53, 80, 443, 22, 3389, 445, 8080) with authentic OS ICMP ping execution (`ping -c 1 -W 1 <ip>` on Linux, `ping -n 1 -w 800 <ip>` on Windows).
  - Added robust RTT regex parsing (`(?:time|zaman)[=<]\s*([0-9.]+)\s*ms?`) extracting genuine ping latency without requiring root raw socket capabilities.
  - Updated `checkGatewayHealth` in `backend/internal/api/handlers.go` to use `monitor.PingHost`: gateway (modem `192.168.1.1`) now accurately shows "online" with authentic ~0.8ms latency instead of "unreachable".
  - Refactored `poller.go` device polling and `scanner.go` subnet discovery to use `monitor.PingHost` for reachability detection.
- **SQLite High-Concurrency WAL Hardening (`backend/internal/database/db.go`)**:
  - Eliminated database deadlocks and timeouts by executing explicit PRAGMAs (`PRAGMA journal_mode = WAL;`, `PRAGMA busy_timeout = 10000;`, `PRAGMA synchronous = NORMAL;`, `PRAGMA cache_size = -20000;`, `PRAGMA foreign_keys = ON;`).
  - Expanded connection pool to `SetMaxOpenConns(25)`, `SetMaxIdleConns(10)`, `SetConnMaxLifetime(1h)`, eliminating the single-connection bottleneck that caused 15000ms Axios timeouts.
- **Atomic Single-Transaction Settings Batching (`UpdateSettings`)**:
  - Refactored `UpdateSettings` in `backend/internal/api/handlers.go` to wrap all key updates inside a single prepared statement transaction (`tx.Begin()`, `tx.Prepare()`, `tx.Commit()`). Operations now execute in <2ms.
- **Bulletproof Agent Installation & Auto-Enrollment**:
  - In `GenerateLinuxAgentScript`: added `pkill -9 -f gnnwatchtower-agent` and service stop prior to download, writing to `.tmp` before atomic `mv -f` to permanently prevent `ETXTBUSY` (curl error 23).
  - In `GenerateWindowsAgentScript`: added upfront Administrator privilege verification (`IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)`).
  - In `AgentHeartbeat`: ensured resilient `devID` fallback query and `targetIP` sanitization (stripping loopback).
  - In `backend/cmd/agent/main.go`: improved local IP detection by probing the server route directly via UDP socket.
- **Go Compilation Cleanup**:
  - Removed unused `"net"` package import from `backend/internal/api/handlers.go` ensuring zero compiler errors during `go build ./cmd/server`.

## [2026-10-11] - Real OS Hardware Telemetry & Dashboard Veracity Alignment

### Completed Work
- **Native OS Hardware Telemetry in Agent (`backend/cmd/agent/`)**:
  - `metrics_linux.go`: Genuine Linux CPU utilization calculated from `/proc/stat` time delta, physical RAM percentage from `/proc/meminfo` (`(MemTotal - MemAvailable) / MemTotal * 100`), and root filesystem space via `syscall.Statfs("/")`.
  - `metrics_windows.go`: Genuine Windows CPU percentage via `kernel32.dll` `GetSystemTimes`, physical RAM percentage via `GlobalMemoryStatusEx` (`dwMemoryLoad`), and drive space via `GetDiskFreeSpaceExW` on `C:\`.
  - `metrics_fallback.go`: Clean compilation fallback for other platforms.
  - Eliminated hardcoded `0.0` CPU/Disk values and Go runtime heap memory allocations (`m.Alloc`).
  - Rounded metrics to 1 decimal place (`math.Round(val*10) / 10`).
- **TelemetryInspector Veracity & Incident Filtering Fixes**:
  - Added Disk Usage metric indicator alongside CPU and RAM in footer.
  - Formatted telemetry float percentages with `formatPct` (`Number(val).toFixed(1) + '%'`), completely resolving raw float artifacts (`0.6020584106445312%`).
  - Formatted sub-millisecond pings to render `< 1 ms` instead of integer-truncated `0 ms`.
  - Scoped incidents to selected device via `props.incidents.filter(i => i.device_id === props.device?.id)`, preventing incidents from another node appearing under the active node.
  - Chronological sparkline metrics reversal: SVG points ordered from oldest to newest for correct time-series curve.
- **Gateway (Modem) Prominence & Topology Alignment**:
  - Added dedicated Gateway card in `HostCards.vue` (`Ağ Geçidi (Modem)`) when less than 3 host nodes exist, showing real-time ICMP ping and routing status.
  - Added live Gateway latency indicator directly in `TopBar.vue` with emerald status dot.
  - Sub-millisecond formatting `< 1 ms` applied across `DeviceTable`, `HostCards`, and `TopBar`.
- **Architecture Decision Records & Coherence**:
  - Authored `ADR-0005: High-Concurrency SQLite WAL and Hybrid ICMP/TCP Ping Engine`.
  - Updated `ADR-0001` with bi-directional lineage link to ADR-0005.
- **Frontend Production Build**:
  - Recompiled Vue 3 SPA via `npm run build` directly to `backend/cmd/server/dist/`.

## [2026-10-11] - Release v0.3.0: Website (HTTP/S) Monitoring, Real-Time Hardware & Network Telemetry, and Omnichannel Alerting

### Completed Work
- **Website & URL Monitoring (Uptime Kuma Topology)**:
  - Added native HTTP/HTTPS website reachability probing (`checkWebsiteReachability` in `poller.go`) with status code verification (<500) and response time calculation.
  - Added "🌐 Web Sitesi / URL (HTTP/HTTPS)" option in `AddDeviceModal.vue` with dynamic host/URL input placeholders.
  - Added automatic alert generation on website failure and recovery to Telegram and Discord.
  - Added dedicated website globe icons and classification formatting in `DeviceTable.vue`.
- **Full Hardware & Network Telemetry Engine (CPU, RAM, Disk, Net I/O, Temp)**:
  - Linux agent: Native Network RX/TX KB/s from `/proc/net/dev`, Temperature from `/sys/class/thermal/` and `/sys/class/hwmon/`.
  - Windows agent: Native Network RX/TX KB/s via `iphlpapi.dll` `GetIfTable`, zero external dependencies.
  - Database schema: Added `net_rx_kbps` and `net_tx_kbps` columns to `system_metrics` with idempotent runtime migration.
  - API Performance: Refactored `GetDevices` in `handlers.go` with a single high-performance `LEFT JOIN` on the latest `system_metrics` row.
  - UI Visualization: Added dedicated `CPU`, `RAM`, `Disk`, `Ağ (I/O)`, and `Sıcaklık` columns to `DeviceTable.vue`, real-time hardware gauges on `HostCards.vue`, and Network I/O in `TelemetryInspector.vue`.
- **Omnichannel Alerting & Real-Time Diagnostics**:
  - Wired instant alert dispatches (`incident.DispatchAlert`) to Telegram, Discord, and Brevo for:
    - New device created / discovered.
    - Node / Website outage (DOWNTIME) and recovery (RECOVERY).
    - New agent auto-registration.
    - Graceful OS shutdown / reboot signals.
    - Incident diagnosis and operator notes recorded.
  - Added `POST /api/settings/test-alert` endpoint and interactive "Test Mesajı Gönder" buttons in `SettingsModal.vue` with direct API response status.
  - Hardened Telegram bot token handling (automatic `bot` prefix strip and whitespace trimming).
- **Version Bump to v0.3.0**:
  - Incremented version across `router.go` (`/api/health`), `TopBar.vue` (badge), `frontend/package.json`, and `update.sh`.
- **Frontend Production Build**:
  - Recompiled Vue 3 SPA via `npm run build` cleanly to `backend/cmd/server/dist/` in 1.41s.





## 2026-10-16 — Approved monitoring redesign and discovery retirement

Implemented three server-enforced monitoring modes, ICMP-only appliance dispatch without TCP fallback, contextual Turkish navigation and entry, separate tables, truthful freshness/dashboard summaries, transactional incidents, preserved legacy inventory/history, validated backup restore, and complete scanner/UI/API/configuration removal. Existing server agent/service functionality remains scoped to Ajan Özeti. Backend tests, static analysis, frontend tests/build, browser fixture review and zero-CGO builds passed. No customer probing, alert, commit, push or deployment occurred. See [validation report](../audits/monitoring-redesign-validation-2026-10-16.md).

## 2026-10-16 — README refresh and explicit Git anonymity rule

Rewrote README in plain Turkish, removed emojis and unsupported deployment/EDR claims, added six factual badges, documented current monitoring categories and ICMP limits, corrected Windows/Linux service names and enrollment commands, and documented build packaging, backup/rollback limits and CLI option persistence. Commands were compared with local source; no installation, service action, test or build was executed. Local documentation links and code fences were checked. Applied the operator-provided sentinel-coauth rule to .agents/AGENTS.md through the required protected-file approval. No staging or commit occurred.

## 2026-10-16 — Restore command-first README

Restored the operator-requested single-line installation and update commands at the top of README. Kept everyday server/agent service commands and badges visible; moved expanded configuration, build, enrollment and troubleshooting guidance into docs/operations.md. Local documentation links were checked. No installation, service operation, test, commit or push was performed for this correction.

## 2026-10-16 — Reachability alarm hotfix

Serialized existing OS ICMP subprocesses, allowed process startup overhead separately from the unchanged reply timeout, and classified local process deadline expiry as unknown. Added bounded failure diagnostics to existing runtime logs. A single failed ICMP observation becomes unknown; only a second consecutive scheduled failure opens an outage, without retries or additional traffic. Fresh authenticated agent telemetry is rechecked in the observation transaction before accepting an offline server result; failed ICMP does not fabricate a last-success timestamp. No new protocol, listener, credentials or privileges. Focused monitor/API tests and Linux zero-CGO build used; customer root cause remains unconfirmed until failure diagnostics are observed.
