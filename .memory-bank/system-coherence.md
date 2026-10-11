# System Coherence Protocol (GNNwatchtower)

## 1. Overview and Core Invariants
This document defines the governance rules, operational procedures, and runtime safety boundaries for the **GNNwatchtower** repository. All autonomous agents, CI processes, and human contributors are bound by these invariants.

### Core Principles
1. **The Golden Rule - Absolute Real Data & Zero Placeholders (CRITICAL)**: NO mock data, synthetic statistics, or static placeholders are permitted. Every number, chart, status LED, and row in the UI must be backed by real live backend telemetry, SQLite storage, or real OS socket checks. Missing or uncollected telemetry must render authentic empty states (`—` or `Veri Bekleniyor`), never simulated mock values.
2. **Never Leak Secrets or Client PII**: This is a public repository (`https://github.com/BigDesigner/project-gnnwatchtower.git`). All customer IP spaces, MAC addresses, operator tokens, credentials, and notification secrets (Telegram, Discord, Brevo) MUST reside strictly in local non-committed SQLite databases or git-ignored `.env` files.
3. **Single-Binary & Pure Go Architecture**: The production runtime must produce a self-contained binary on port `3800` containing embedded UI assets, pure Go SQLite (WAL mode), zero external C dependencies, and no mandatory Docker daemon on customer appliances.
4. **Traceable Migrations & Preservation of Documentation**: No legacy documentation is destroyed. All historical specs are archived or maintained with explicit migration mapping.
5. **No Unapproved Commits**: Automated agents may prepare commit messages and `git add` file lists, but must never execute `git commit` or `git push` without explicit operator consent.

---

## 2. Session Protocol and State Lifecycle

### 2.1 Session Initialization
1. Check for `.memory-bank/.session.lock`. If modified in the last 10 minutes, halt and notify. If stale (>10 min), log a warning in `bugs/bug-list.md`, remove it, and continue.
2. Read `.memory-bank/active-session.json` to restore context, branch status, and active tasks.
3. Read `.tasks/pipeline.md` and `.tasks/handoff.md` to identify the current sprint and immediate priorities.
4. Set `status` in `active-session.json` to `ACTIVE` using atomic write (`.tmp.json` -> `active-session.json`).

### 2.2 Pre-Change Checklist
- [ ] Verify git worktree status (`git status`). Ensure no uncommitted collision.
- [ ] Cross-check proposed changes against `.specs/boundary-conditions.md`.
- [ ] Confirm no hardcoded secrets, test API keys, or customer identifiers are introduced.
- [ ] Verify changes conform to AGPL-3.0 licensing requirements.

### 2.3 Post-Change Checklist
- [ ] Verify syntax and compilation (`go build ./...` or `go vet ./...`).
- [ ] Ensure `.gitignore` continues to mask all `.env*`, `data/*.db`, and backup files.
- [ ] Update `.memory-bank/changelog/verified-worklog.md`.
- [ ] Update `.tasks/pipeline.md` with task progress.
- [ ] Release lock by deleting `.memory-bank/.session.lock`.

---

## 3. Locking and Concurrency
- Primary lock: `.memory-bank/.session.lock`.
- Stale lock timeout: 10 minutes.
- Updates to `active-session.json` MUST be atomic (write to `.tmp.json`, then rename).

---

## 4. Unconfirmed Decision Protocol
- Critical architecture, licensing, security, or deployment items that lack verifiable repository evidence must be labeled `Unconfirmed`.
- In **Interactive Mode**, the agent halts and requests operator clarification before making architectural commitments.
- In **CI Mode**, decisions are recorded as `Status: Proposed` and `Confidence: Unconfirmed` in ADRs and logged to `changelog/ci-run-summary.md`.

---

## 5. Public Repository Security and Redaction Contract
- Staged files must never contain unredacted secrets or real network customer information.
- If an agent detects potential secrets, tokens, or PII in code or markdown documentation, it must redact them to `<REDACTED>` immediately and alert the operator.
