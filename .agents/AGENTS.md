# Agent Operating Directives (GNNwatchtower)

## Governing Golden Rule: No Additional Attack Surface
- **Binding operator decision, 2026-10-16:** Watchtower must not introduce or expand attack surface to monitor customer systems. Security takes precedence over telemetry coverage, device compatibility, convenience, automation, and feature completeness.
- Do not enable disabled APIs, SNMP, Redfish, IPMI, SSH, management services, listeners, firewall paths, remote access, or privileged agents solely to collect telemetry. Do not widen service reachability or weaken authentication, encryption, certificate verification, or segmentation.
- An existing service, read-only account, encrypted protocol, outbound-only connection, or reused port does not establish compliance by itself. Assess credential custody, trust relationships, handlers/parsers, dependencies, update mechanisms, resource use, and cross-customer reachability as well.
- Before implementing a feature, document its attack-surface delta against an evidenced deployment baseline. If absence of added exposure cannot be established, omit that collection path and show an authentic unavailable state. Discovery must not activate services or try alternate credentials/protocols automatically.
- Existing insecure behavior is remediation debt, not permission to extend it. Neither opt-in nor an ordinary feature request waives this constraint; an exception requires an explicit operator revision of the governing policy.
- See [ADR 0006](../.memory-bank/adr/0006-no-additional-attack-surface.md) and [security boundaries](../.specs/boundary-conditions.md). This is the single governing golden rule; data veracity remains a mandatory supporting requirement.

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
- **Supporting Data Veracity Invariant (CRITICAL)**: No fabricated metrics or health states. Every displayed metric must be grounded in real collected data. If collecting it would violate the governing golden rule, omit collection and display an authentic unavailable state; never weaken security to fill a dashboard.


## Git Commit Anonymity Rule
- **CRITICAL GIT PROHIBITION**: Never append, inject, or suggest any "Co-Authored-By" trailers, metadata, or attribution lines (e.g., "Co-Authored-By: Claude...") in git commit messages, code blocks, or automated git scripts. All git commit messages must remain completely anonymous or strictly limited to the user's explicit content.
