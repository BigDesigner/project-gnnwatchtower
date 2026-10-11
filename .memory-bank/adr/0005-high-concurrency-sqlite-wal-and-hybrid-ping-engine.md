# ADR 0005: High-Concurrency SQLite WAL & Hybrid Ping Telemetry Engine

- **Status**: Accepted
- **Confidence**: Verified
- **Date**: 2026-10-11
- **Category**: Persistence
- **Supersedes**: [ADR 0001: Initial Architecture Stack & Appliance Topology](file://.memory-bank/adr/0001-initial-stack.md)
- **Superseded By**: None

## Context
In production deployments of GNNwatchtower on Ubuntu 22.04/24.04 LTS appliances, two critical architectural bottlenecks emerged:
1. **SQLite Deadlocks and Axios 15,000ms Timeouts**: ADR-0001 previously specified a single connection (`db.SetMaxOpenConns(1)`) with DSN pragma query parameters (`_pragma=journal_mode(WAL)`). Because the pure Go SQLite driver (`modernc.org/sqlite`) ignores query string pragmas, SQLite remained in default rollback journal mode with a 0ms busy timeout. When background polling workers (evaluating 20+ devices concurrently) and incoming HTTP requests (`/api/settings`, `/api/auth/login`, `/api/agent/report`) competed for the database, the single connection was completely exhausted, causing requests to stall in the connection queue until client timeouts fired.
2. **Gateway "Unreachable" False Positives under ICMP Filtering**: Probing network reachability exclusively via TCP ports (53, 80) or unprivileged raw ICMP sockets (`net.Dial("ip4:icmp")`) produced false "offline" classifications for routers, modems (e.g., `192.168.1.1`), and firewalled clients that explicitly reject TCP management connections but respond to standard ICMP echo requests in <1ms.

## Decision
1. **Explicit SQLite Runtime PRAGMAs & Connection Pool Expansion**:
   - Explicitly execute runtime PRAGMAs on database initialization:
     - `PRAGMA journal_mode = WAL;` (enables non-blocking concurrent readers while a writer commits)
     - `PRAGMA busy_timeout = 10000;` (writers wait up to 10 seconds for locks instead of failing immediately)
     - `PRAGMA synchronous = NORMAL;` (dramatically reduces disk sync latency in WAL mode without risking corruption)
     - `PRAGMA cache_size = -20000;` (allocates 20 MB memory cache)
     - `PRAGMA foreign_keys = ON;`
     - `PRAGMA temp_store = MEMORY;`
   - Expand database connection pool to `db.SetMaxOpenConns(25)` and `db.SetMaxIdleConns(10)` with `SetConnMaxLifetime(1h)`, allowing concurrent API reads during background polling sweeps.
   - Enforce atomic transaction batching (`tx.Begin()`, `tx.Prepare()`, `tx.Commit()`) for multi-record operations (such as `UpdateSettings`), reducing execution time from hundreds of milliseconds to <2ms.

2. **Hybrid OS ICMP Reachability Engine (`monitor.PingHost`)**:
   - Implement a two-tiered reachability engine:
     1. **Tier 1 (Fast TCP Probe)**: Probes common ports (53, 80, 443, 22, 3389, 445, 8080) with a 200ms timeout for ultra-fast response detection.
     2. **Tier 2 (OS ICMP Ping Fallback)**: Executes the native operating system ping binary (`ping -c 1 -W 1 <ip>` on Linux, `ping -n 1 -w 800 <ip>` on Windows).
   - Extracts real round-trip time (RTT) via regular expression (`(?:time|zaman)[=<]\s*([0-9.]+)\s*ms?`), guaranteeing authentic latency values without requiring root raw socket capabilities (`CAP_NET_RAW`).

3. **Daemonized Mini-Agent Architecture**:
   - Mini-Agents operate in an exclusively **outbound** push model (`/api/agent/report`), requiring zero open inbound listening ports on monitored Windows and Linux endpoints.
   - Automatically determine the precise local network interface by route-probing the central appliance host via UDP socket.
   - Native OS daemon lifecycle management:
     - Linux: Native `systemd` service (`multi-user.target`, `Restart=always`).
     - Windows: Task Scheduler SYSTEM task (`-AtStartup`, `ExecutionTimeLimit 0`).

## Consequences
- Completely eliminates the 15,000ms database lock timeout during settings updates and agent check-ins.
- Resolves gateway status false negatives, rendering true ~0.8ms modem latency on dashboard overview cards.
- Guarantees seamless agent startup across system reboots on both Linux and Windows platforms.
- Preserves the single-binary, pure Go, zero-CGO architectural invariant.

## Lineage & Migration
1. **Superseded Architectural Decision**: Supersedes the `db.SetMaxOpenConns(1)` and DSN pragma constraint in [ADR 0001](file://.memory-bank/adr/0001-initial-stack.md).
2. **Migration Path**: Applied in-place via `database.InitDB()`. Existing SQLite databases automatically transition to WAL mode upon executing `PRAGMA journal_mode = WAL;` without schema alterations or data migration.
3. **Backward Compatibility**: Fully backward compatible with existing SQLite data files (`gnnwatchtower.db`).

## Evidence
- `backend/internal/database/db.go` lines 25-58
- `backend/internal/monitor/ping.go` lines 1-84
- `backend/internal/monitor/poller.go` lines 124-235
- `backend/internal/api/handlers.go` lines 544-612, 930-960
- `backend/cmd/agent/main.go` lines 30-65
- Commit `be203ef`: `fix(core): implement real ICMP gateway ping, high-concurrency SQLite WAL, and harden agent deployment`
- Commit `1aaf035`: `fix(build): remove unused net import in handlers.go`
