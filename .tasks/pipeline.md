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
- [x] Implement discovery engine with local CIDR range scanner and RFC 1918 security guards.
- [x] Implement power-outage correlation classifier and multi-channel notification dispatchers (Telegram, Discord, Brevo).
- [x] Nightly 03:00 automated backup scheduler and rotation manager.
- [x] Standalone cross-platform Agent binary (`backend/cmd/agent/main.go`) with pre-shutdown signal emission.
- [x] Multi-stage production `Dockerfile` and single-binary `docker-compose.yml`.
- [x] Automated Linux installer script `install.sh` for Ubuntu 22.04 / 24.04 LTS.

## Backlog
- [ ] VMware ESXi vSphere API status polling module (Port 443/902).
- [ ] Synology DSM SNMP polling module (Port 5000/5001).

## Validation Plan
1. Static analysis: `go vet ./...`
2. Test suite: `go test -v ./...`
3. Appliance build: `CGO_ENABLED=0 go build -o bin/gnnwatchtower ./cmd/server`
