# Boundary Conditions & Security Contract (GNNwatchtower)

## 1. Public Repository Zero-Leak Invariant (CRITICAL)
- **Status**: Verified
- **Boundary**: Under NO circumstances may client infrastructure details, discovered hostnames, client IP spaces, MAC address maps, agent authentication tokens, or third-party notification credentials be committed to the public Git repository.
- **Enforcement**:
  - All sensitive files (`.env`, `data/`, `backups/`, `*.db`, `*.key`) are barred via `.gitignore`.
  - All markdown documentation and error logs must replace actual secrets with `<REDACTED>`.

## 2. Input Validation and Network Security
- **Subnet & IP Bounds**: CIDR inputs for network scanning must be validated using `net.ParseCIDR` and restricted to RFC 1918 private IP ranges (`10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`) unless explicitly overridden with administrative confirmation.
- **Port Ranges**: Monitored service ports must be strictly validated between integers `1` and `65535`.
- **MAC Address Format**: MAC addresses must be validated against standard IEEE 802 formats (`^([0-9A-Fa-f]{2}[:-]){5}([0-9A-Fa-f]{2})$`).
- **Ping & Raw Socket Boundaries**: Raw socket creation (`cap_net_raw`) or ICMP echo requests must be bounded by concurrency limits (max 64 simultaneous scan goroutines) to avoid host interface buffer saturation.

## 3. Database Concurrency & SQLite WAL Safety
- **Single-Writer Constraint**: Pure Go SQLite (`modernc.org/sqlite`) must maintain `db.SetMaxOpenConns(1)` for write operations to prevent file-lock contention and database corruption.
- **WAL Pragma**: Always initialize SQLite with `_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)`.
- **Cascading Deletion**: Deletion of a device must cascade to `services_monitored`, `downtime_incidents`, and `system_metrics` to guarantee zero-orphan database integrity.

## 4. Authentication, Session & Access Boundaries
- **Operator Passwords**: Passwords must never be logged or echoed in plaintext.
- **Session Tokens**: Cryptographically secure 24-byte hex strings generated with `crypto/rand`.
- **Machine Agent Tokens**: Unique 48-byte tokens generated during device provisioning for authenticated heartbeat and metric ingestion (`X-Agent-Token` header).
- **IDOR / BOLA Prevention**: All device and incident manipulation endpoints must verify valid authenticated session context.

## 5. Alert Egress Throttling & Outbound Cooldown Guard
- **Notification Debouncing**: Flapping hosts (toggling online/offline rapidly) must be debounced with a minimum cooldown of 3 minutes before dispatching repeated alerts via Brevo, Discord, or Telegram.
- **Power Outage Detection**: If more than the configured threshold of devices (default 3) go offline within a 60-second window, individual alerts must be suppressed and consolidated into a single "Probable Power/Core Switch Outage" alert.
- **Rate Limit Handling**: Third-party API calls must gracefully handle HTTP 429 and exponential backoff.

## 6. Frontend UI Declarative DOM Construction
- Dynamic UI elements in the Vue 3 console must be rendered declaratively via Vue template bindings, never through raw HTML sinks (`v-html` or `innerHTML`) to prevent Cross-Site Scripting (XSS).

## 7. Data Veracity and Anti-Mocking Contract (GOLDEN RULE)
- **Status**: Verified
- **Boundary**: No mock data, no synthetic statistics, and no visual placeholders may be presented in any UI view or API endpoint.
- **Enforcement**:
  - Live metric widgets (e.g. Host CPU/RAM, aggregate traffic, temperature, cluster node counts, and storage IOPS) must query real backend system telemetry (`/api/stats`, `/api/system/*`, `/api/metrics`).
  - If a metric cannot be measured or has not yet been collected (e.g. clean appliance start, offline agent), the UI must display a precise non-interpolated empty state (e.g. `—`, `Veri Yok`, or `Cihaz Bulunamadı`) rather than fabricated numbers.
  - Simulated placeholder arrays in frontend stores or components are strictly forbidden in production code.

