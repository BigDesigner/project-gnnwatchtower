# Migration Map

| Original Path | New Path | Archive Path | Action | Notes |
|---|---|---|---|---|
| `IMPLEMENTATION_PLAN.md` | `IMPLEMENTATION_PLAN.md` | N/A | Left in place | Primary Turkish architecture specification; referenced across `.specs/` |
| `LICENSE` | `LICENSE` | N/A | Left in place | GNU Affero General Public License v3 (AGPL-3.0) confirmed |
| `.env.example` | `.env.example` | N/A | Left in place | Sanitized with generic placeholder tokens and warning headers |
| `.gitignore` | `.gitignore` | N/A | Left in place | Hardened to block `.env`, SQLite DBs, backups, and secret keys |
| `docker-compose.yml` | `docker-compose.yml` | N/A | Left in place | Prototype/development container compose; superseded by single-binary target |
