# Task: Deprecate Network Discovery & Subnet Scanning Module

**Status**: Implemented and locally validated; not deployed
**Priority**: Medium  
**Created At**: 2026-10-15  
**Origin**: Operator Architectural Decision  

## Description
Operator revision, 2026-10-16: fully remove network discovery/subnet scanning to reduce unnecessary probing and application attack surface. This task is incorporated into the [monitoring navigation redesign plan](monitoring-navigation-redesign-plan-2026-10-16.md), including its security acceptance criteria and phase 1a.

The earlier requirement that every monitored asset must have an agent is superseded. Windows/Linux agent workflows remain in Ajan Özeti; manually registered NAS/ESXi and other IP devices remain agentless and use existing per-device ICMP checks. Removing discovery does not remove scheduled ping or registered inventory/history.

## Planned Scope of Work
1. **Frontend**:
   - Remove "Ağ Taraması Başlat" (Trigger Scan) button from navbar and action bar.
   - Remove `DiscoveryModal.vue` component and all references in `App.vue`.
   - Remove "Ağ Keşfi (CIDR)" from sidebar navigation.
2. **Backend**:
   - Remove `POST /api/discovery/scan` and its handler/imports entirely; the retired path must return the existing API not-found response without network work.
   - Remove the dedicated discovery scanner package, obsolete tests and unused discovery model from the active build; no flag-controlled replacement.
   - Remove the frontend discovery API method and all hidden/global scan triggers, including card/table/empty-state events. Regenerate embedded assets in the approved delivery phase.
   - Retire `DEFAULT_SCAN_SUBNET` from installer, Compose and example configuration; update active documentation without rewriting historical audits.
3. **Database**:
   - Stop seeding/using discovery-only `scan_subnet`; handle legacy/restored discovery settings without reactivation.
   - Preserve shared polling configuration such as `scan_interval_seconds` where still consumed; do not delete by naming pattern.
   - Preserve previously discovered device records, IDs, telemetry and incident history.

## Acceptance and authorization

- No discovery UI, callable API, scanner implementation or automatic subnet/port enumeration remains in the delivered application.
- Manual category-specific entry and scheduled ping of registered IP devices remain functional. Existing enrollment is available only through its authorized workflow.
- Negative checks cover the retired endpoint with and without authentication; existing records and old backup imports cannot restart scanning.
- Implementation and local tests/builds were explicitly authorized by the operator. Evidence: [validation report](../.memory-bank/audits/monitoring-redesign-validation-2026-10-16.md). Production deployment was not performed.
