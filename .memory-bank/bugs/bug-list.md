# Bug List & Technical Debt Registry

| ID | Issue Description | Confidence | Status | Severity | Suggested Next Action |
|---|---|---|---|---|---|
| BUG-001 | `docker-compose.yml` specifies PostgreSQL (`netmon-db`) whereas production architecture uses pure Go SQLite (`modernc.org/sqlite`) | Verified | Open | Low / Tech Debt | Deprecate or refactor `docker-compose.yml` to reflect single-binary SQLite or remove Postgres service to avoid architectural confusion. |
| BUG-002 | `backend/internal/api/auth.go` session storage uses in-memory map without persistent token eviction across process restarts | Verified | Open | Medium | Implement session expiry cleanup worker and consider optional token persistence in SQLite. |
