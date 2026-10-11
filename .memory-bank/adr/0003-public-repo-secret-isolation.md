# ADR 0003: Public Repository Secret & Client Data Isolation Architecture

- **Status**: Accepted
- **Confidence**: Verified
- **Date**: 2026-10-11
- **Supersedes**: None
- **Superseded By**: None

## Context
GNNwatchtower is hosted on a public GitHub repository (`https://github.com/BigDesigner/project-gnnwatchtower.git`). An appliance for network monitoring inevitably handles sensitive client data: discovered internal subnet IPs, MAC addresses, machine hostnames, operator password hashes, agent communication tokens, and third-party alert credentials (Telegram Bot Tokens, Discord Webhooks, Brevo API Keys). Under no circumstances may client data or production credentials be committed to the public Git repository.

## Decision
Enforce a multi-layered secret isolation architecture:
1. **Repository-Level Exclusion (.gitignore)**:
   - Exclude `.env` and all `.env.*` variants; permit only `.env.example`.
   - Exclude all SQLite database files (`data/`, `*.db`, `*.db-wal`, `*.db-shm`, `*.sqlite*`).
   - Exclude all backup tarballs and database dumps (`backups/`, `*.tar.gz`, `*.sql`, `*.bak`).
   - Exclude private keys and certificates (`*.pem`, `*.key`, `*.crt`).
2. **Template Sanitization (.env.example)**:
   - All environment templates must strictly use dummy placeholder tokens (`replace_with_a_secure_random_32_char_secret`) and explicitly warn operators against committing real secrets.
3. **Database Configuration Storage**:
   - Alert integration secrets (Telegram, Discord, Brevo) and client corporate naming are stored exclusively in the local runtime SQLite `system_settings` table on the appliance, entered through the dashboard or local CLI.
4. **Agent Token Security**:
   - Machine agents communicate using dynamically generated 48-character hex cryptographic tokens stored only on the local machine and local appliance database.

## Consequences
- Zero risk of client network topology or API token leakage to public GitHub.
- Safe automated builds and open-source contributions.
- Clear separation between application source code and deployment runtime state.

## Evidence
- `.gitignore`
- `.env.example`
- `backend/internal/database/db.go` (table definitions for `system_settings` and `devices`)
