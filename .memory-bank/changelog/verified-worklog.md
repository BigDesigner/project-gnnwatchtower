# Verified Worklog

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
- **Agent Download Verification**:
  - Updated `GenerateLinuxAgentScript` and `GenerateWindowsAgentScript` to fail fast with clear instructions if download fails or file size is invalid.
  - Hardened `DownloadAgentBinary` to enforce minimal executable file size verification (>10KB).
- **Frontend Production Build**:
  - Recompiled Vue 3 SPA via `npm run build` directly to `backend/cmd/server/dist/`.


