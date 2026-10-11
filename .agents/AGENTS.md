# Agent Operating Directives (GNNwatchtower)

## Communication Style Rules
- **No Fluff**: Omit introduction sentences, congratulations, empty apologies, and decorative greetings.
- **Direct Focus**: Present production-ready code, verified command lines, and clear architecture analysis directly.
- **Error Handling**: Acknowledge errors factually without apologies; output corrected code or instructions immediately.
- **Language**: The operator's preferred interactive conversation language is **Turkish**. All generated files in `.memory-bank/`, `.specs/`, `.agents/`, and `.tasks/` must remain strictly in **English**.

## Critical Execution Guardrails
- **Anti-Eager Execution (CRITICAL)**: When generating or updating implementation plans that require operator review or feedback, halt immediately and await explicit user approval before touching code or executing build/deployment commands.
- **Public Repo Secret Guard (CRITICAL)**: Before proposing or executing any git operation, verify that `.env`, SQLite DB files, backup archives, customer IPs, or private tokens are not staged.
- **Single-Binary Appliance Invariant**: Preserve the pure Go SQLite (`modernc.org/sqlite`), single binary, and zero-CGO target architecture. Do not introduce mandatory external service dependencies.
- **License Invariant**: Adhere strictly to the AGPL-3.0 copyleft terms.
- **Zero Placeholder Invariant (GOLDEN RULE - CRITICAL)**: NO mock data, NO fake statistics, and NO hardcoded placeholders are permitted anywhere in the frontend or backend. Every metric, device list, temperature value, traffic rate, ping latency, and incident displayed in the UI must be real data queried directly from the backend API, SQLite database, or live OS network telemetry. In the absence of data, render authentic empty states with functional call-to-actions, never simulated mock datasets.

