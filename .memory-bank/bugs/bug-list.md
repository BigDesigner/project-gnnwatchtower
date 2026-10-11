# Bug List & Technical Debt Registry

| ID | Issue Description | Confidence | Status | Severity | Suggested Next Action |
|---|---|---|---|---|---|
| BUG-001 | `docker-compose.yml` specifies PostgreSQL (`netmon-db`) whereas production architecture uses pure Go SQLite (`modernc.org/sqlite`) | Verified | Closed | Low / Tech Debt | Resolved. Refactored `docker-compose.yml` to single-binary `gnnwatchtower` appliance service on port 3800 with SQLite volume persistence. |
| BUG-002 | `backend/internal/api/auth.go` session storage uses in-memory map without persistent token eviction across process restarts | Verified | Closed | Medium | Resolved. Added background hourly ticker to evict expired tokens from in-memory session registry. |

