# Engineering Constitution (GNNwatchtower)

## 1. Core Engineering Principles
1. **The Governing Golden Rule - No Additional Attack Surface (CRITICAL)**: Watchtower must not introduce or expand attack surface on customer infrastructure or its monitoring environment. Reject features requiring new exposed services, broader network access, increased privileges, weakened protection, or unverified additional trust/credential/code exposure. Monitoring coverage and device compatibility never override this constraint. Existing-service reuse requires the same assessment. If a collection path cannot satisfy the rule, omit it and report unavailable data. See [ADR 0006](../.memory-bank/adr/0006-no-additional-attack-surface.md).
   **Supporting Data Veracity Requirement:** Every displayed metric and health state must be backed by real collected data. Fabricated statistics are prohibited. Missing, stale, unsupported, or deliberately uncollected data must be explicit; security takes precedence over filling a dashboard.
2. **Single Binary Portability**: The appliance backend and embedded frontend must build into a single static binary requiring zero external system runtime packages.
3. **Pure Go / Zero-CGO**: All database operations and network utilities must run with `CGO_ENABLED=0` to support cross-compilation and easy updates on target appliances.
4. **No Decorative Emojis**: System logs, terminal outputs, commits, and UI elements must never use decorative emojis. All interface status indicators must use structured CSS LED dots and Google Material Symbols (Outlined).

## 2. Go Coding Standards
- **Error Handling**: Functions must return explicit `error` values. Never swallow errors silently or rely on `panic` in HTTP handlers or poller routines.
- **Context Awareness**: All background goroutines, registered-target checks, and HTTP requests must respect `context.Context` cancellation to prevent orphaned threads.
- **Concurrency Safety**: Mutex locks must protect in-memory state (`sessionsMu`, poller tables) with strict `defer unlock` scoping.

## 3. UI/UX Design System (Cisco Meraki Dark Minimalist)
- **Palette**:
  - Base Background: Slate/Dark Navy (`#0b132b`, `#1c2541`, `#0f172a`)
  - Accent Color: Cisco Cyan/Sky (`#00bceb`, `#0284c7`)
  - Card & Grid Borders: Slate-800 (`border-slate-800`)
- **Status LED Elements**:
  - Online: Emerald (`#10b981`) with soft glow
  - Offline: Rose (`#f43f5e`)
  - Warning/Maintenance: Amber (`#f59e0b`)
  - Dormant/Sleep: Slate (`#64748b`)
- **Typography**: Google Inter font family. All IP addresses, MAC addresses, ports, and numeric metric columns must enforce `font-mono tabular-nums`.

## 4. Git & Commit Hygiene
- **No Automatic Commits**: All changes must be explicitly approved by the operator prior to `git commit` or `git push`.
- **Commit Format**: Conventional commits (`feat:`, `fix:`, `docs:`, `refactor:`, `chore:`).
- **Public Repository Cleanliness**: Verify `git status` before every stage to ensure untracked runtime files are never introduced.

## 5. Licensing Governance
- GNNwatchtower is governed by the **GNU Affero General Public License v3 (AGPL-3.0)**.
- Any imported third-party library must have an AGPLv3-compatible license (MIT, BSD, Apache 2.0, MPL 2.0, or AGPLv3).
