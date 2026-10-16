# Project Pipeline & Task Backlog (GNNwatchtower)

## Current Status
- **Active Sprint**: `v0.1.0-alpha` (Appliance Foundation & Security Hardening)
- **Status**: Memory Bank bootstrapped; AGPLv3 license confirmed; Secret protection enforced.

## Active & Immediate Next Tasks
- [x] Link repository with GitHub (`https://github.com/BigDesigner/project-gnnwatchtower.git`).
- [x] Verify and sync AGPL-3.0 license.
- [x] Configure zero-leak `.gitignore` and sanitized `.env.example`.
- [x] Initialize Sentinel Agent Memory Bank (`.memory-bank/`, `.specs/`, `.agents/`, `.tasks/`).
- [x] Implement embedded Vue 3 SPA frontend (Cisco Meraki Dark minimalist style).
- [x] Connect Go backend routes to serve embedded frontend (`//go:embed dist/*`).
- [x] Historical implementation: discovery engine; complete removal is now required by [TASK-001](TASK-001-deprecate-network-discovery.md).
- [x] Implement power-outage correlation classifier and multi-channel notification dispatchers (Telegram, Discord, Brevo).
- [x] Nightly 03:00 automated backup scheduler and rotation manager.
- [x] Standalone cross-platform Agent binary (`backend/cmd/agent/main.go`) with pre-shutdown signal emission.
- [x] Multi-stage production `Dockerfile` and single-binary `docker-compose.yml`.
- [x] Automated Linux installer script `install.sh` for Ubuntu 22.04 / 24.04 LTS.

## Backlog
- [x] Implemented the approved [navigation redesign](monitoring-navigation-redesign-plan-2026-10-16.md) and complete [network discovery removal — TASK-001](TASK-001-deprecate-network-discovery.md), including mode isolation, ICMP-only appliances, Turkish views, migration, backup compatibility and embedded assets. See [validation report](../.memory-bank/audits/monitoring-redesign-validation-2026-10-16.md). Local validation only; no production rollout.
- **Binding security gate (2026-10-16):** All feature work is subject to [ADR 0006](../.memory-bank/adr/0006-no-additional-attack-surface.md). Unknown or increased attack surface blocks a collection path. These backlog entries do not authorize enabling services, opening ports, widening access, or adding privileged collectors.
- [ ] Over-the-air appliance update system: CLI updater script (`update.sh`) and Web UI One-Click Update in Settings modal with pre-update DB snapshot.
- [ ] Deferred pending no-additional-attack-surface assessment: VMware ESXi/vCenter telemetry using an already available, assessed data path. No blanket API enablement or port-opening recommendation.
- [ ] Deferred pending no-additional-attack-surface assessment: Synology telemetry using an already available, assessed data path. No enabling SNMP/DSM services solely for monitoring.

## Validation Plan
1. Static analysis: `go vet ./...`
2. Test suite: `go test -v ./...`
3. Appliance build: `CGO_ENABLED=0 go build -o bin/gnnwatchtower ./cmd/server`
