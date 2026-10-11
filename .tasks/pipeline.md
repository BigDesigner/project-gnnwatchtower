# Project Pipeline & Task Backlog (GNNwatchtower)

## Current Status
- **Active Sprint**: `v0.1.0-alpha` (Appliance Foundation & Security Hardening)
- **Status**: Memory Bank bootstrapped; AGPLv3 license confirmed; Secret protection enforced.

## Active & Immediate Next Tasks
- [x] Link repository with GitHub (`https://github.com/BigDesigner/project-gnnwatchtower.git`).
- [x] Verify and sync AGPL-3.0 license.
- [x] Configure zero-leak `.gitignore` and sanitized `.env.example`.
- [x] Initialize Sentinel Agent Memory Bank (`.memory-bank/`, `.specs/`, `.agents/`, `.tasks/`).
- [ ] Implement embedded Vue 3 SPA frontend skeleton (Cisco Meraki Dark minimalist style).
- [ ] Connect Go backend routes to serve embedded frontend (`//go:embed dist/*`).
- [ ] Verify discovery engine with synthetic/local CIDR range.
- [ ] Validate power-outage correlation classifier and test notification dispatchers.

## Backlog
- [ ] Standalone Windows Agent binary (`gnnwatchtower-agent-win.exe`) with pre-shutdown signal emission.
- [ ] Standalone Ubuntu/Debian Agent binary (`gnnwatchtower-agent-linux`) with systemd service unit.
- [ ] VMware ESXi vSphere API status polling module (Port 443/902).
- [ ] Synology DSM SNMP polling module (Port 5000/5001).
- [ ] Nightly 03:00 automated backup scheduler and rotation manager.

## Validation Plan
1. Static analysis: `go vet ./...`
2. Test suite: `go test -v ./...`
3. Appliance build: `CGO_ENABLED=0 go build -o bin/gnnwatchtower ./cmd/server`
