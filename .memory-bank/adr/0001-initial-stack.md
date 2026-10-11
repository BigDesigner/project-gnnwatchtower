# ADR 0001: Initial Architecture Stack & Appliance Topology

> [!WARNING]
> **SUPERSEDED ARCHITECTURAL DECISION**
> The single-connection database constraint and reachability mechanism of this decision were superseded on 2026-10-11 by [ADR 0005: High-Concurrency SQLite WAL & Hybrid Ping Telemetry Engine](file://.memory-bank/adr/0005-high-concurrency-sqlite-wal-and-hybrid-ping-engine.md).
> Refer to the succeeding record for active architecture guidelines and concurrency rationale.

- **Status**: Superseded in part by [ADR 0005: High-Concurrency SQLite WAL & Hybrid Ping Telemetry Engine](file://.memory-bank/adr/0005-high-concurrency-sqlite-wal-and-hybrid-ping-engine.md)
- **Confidence**: Verified
- **Date**: 2026-10-11
- **Supersedes**: None
- **Superseded By**: [ADR 0005: High-Concurrency SQLite WAL & Hybrid Ping Telemetry Engine](file://.memory-bank/adr/0005-high-concurrency-sqlite-wal-and-hybrid-ping-engine.md)

## Context
GNNwatchtower is designed as an on-premise infrastructure, network, and server monitoring appliance deployed on standalone Ubuntu Virtual Machines (20 GB Disk, 4 GB RAM) for Managed Service Providers (MSP). The system must run reliably without external container engine (Docker) or reverse proxy (Nginx) dependencies.

## Decision
1. **Core Language & Runtime**: Go 1.22.
2. **Binary Packaging**: Single self-contained binary (`/opt/gnnwatchtower/gnnwatchtower`) hosting both API/monitoring workers and embedded Vue 3 SPA frontend assets (`//go:embed`).
3. **Database**: Pure Go SQLite (`modernc.org/sqlite`) configured in Write-Ahead Logging (WAL) mode (`_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)`), single writer connection for absolute concurrency safety.
4. **Appliance Port**: Dedicated port `3800` to prevent collisions with common web, hypervisor, or NAS services on local subnets.
5. **Operating Target**: Native Linux Systemd service with automated scheduled backup rotations at 03:00.

## Consequences
- Ultra-lightweight footprint (~25-35 MB RAM idle).
- Zero external runtime dependencies (no Node.js, Python, Docker, or CGO required on the appliance).
- Backup and restore is as simple as copying the single SQLite DB file.

## Evidence
- `IMPLEMENTATION_PLAN.md` lines 7-68
- `backend/internal/database/db.go` lines 12-49
- `backend/go.mod`
