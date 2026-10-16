# Monitoring Navigation, Device Entry and Dashboard Redesign Plan

Date: 2026-10-16. Status: **Implemented locally after explicit operator approval; validation recorded below.**

Scope authorization: the operator explicitly approved applying this plan. Local implementation, focused tests and build preparation are authorized. Deployment, new management protocols and device discovery are outside this authorization. Governing constraint: [ADR 0006](../.memory-bank/adr/0006-no-additional-attack-surface.md).

## 1. Intended outcome

Separate Windows/Linux servers, websites/URLs, and IP-only infrastructure into clear navigation destinations with their own device-entry flows, lists and appropriate metrics. NAS, ESXi and similar appliances receive a dedicated IP-only view comparable to the current websites view. They use only existing ICMP reachability checks, with downtime/recovery incidents and configured alerts. Do not add SNMP, vSphere, Redfish, IPMI, SSH, management credentials, listeners, privileged helpers, or protocol fallback.

The dashboard must distinguish reachability from hardware health and real telemetry availability. Adding a category must not create another device record, start additional probes, or inflate counts.

Scope revision: fully remove network discovery and subnet scanning as specified in [TASK-001](TASK-001-deprecate-network-discovery.md). This is a required part of the redesign, not merely hiding its menu. Preserve scheduled ping of individually registered IP devices. Device entry is manual per category or through the existing explicitly authorized agent enrollment flow in Ajan Özeti; no automatic subnet enumeration remains.

Revision 2026-10-16: the operator requires the complete application interface to be Turkish, including menus, page headings, tabs, buttons, tooltips, empty/loading states, validation, status descriptions and error messages. This supersedes the earlier English navigation proposal. The documentation prose remains English; Turkish literals below specify the requested product interface. Vendor names and protocol acronyms remain recognizable. Internal API fields, database modes and legacy view identifiers are not renamed merely to match visible wording.

## 2. Current implementation findings

- `App.vue` builds `nodeDevices` from every non-website record. NAS/ESXi/network devices therefore enter the general server view; NAS also enters a separate view.
- NAS reuses the general device table. Its server-oriented columns and telemetry inspector suggest capabilities that are not collected for these devices.
- The sidebar contains general devices, websites, NAS and power/temperature destinations, but no shared explicit IP-only category.
- `AddDeviceModal.vue` mixes websites, Windows, Linux, ESXi, NAS, switches and other hardware in a single form. Website/NAS add functions construct a draft through the edit-record prop, making entry context and editing ambiguous.
- The poller treats an HTTP-prefixed address as a website irrespective of declared device type. It also calls `pollDeviceServices` after ordinary device checking; merely changing the frontend cannot guarantee ping-only execution.
- Device creation generates an agent token for every device. An IP-only target should not receive provisioning/upgrade capabilities.
- The dashboard's `uptime_percentage` currently represents the fraction of devices online at the current instant, despite a comment describing 24-hour uptime. It must not be displayed as historical SLA.
- `last_seen_at` represents successful observation, not every attempted check. The configured interval field is read but active polling does not use it to schedule each device accurately.
- Power/temperature copy currently mentions IPMI, a fixed three-device threshold and an unconditional stable-power message. These claims need removal or replacement with measured, correctly scoped statements.

Primary touchpoints: [App.vue](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/frontend/src/App.vue), [Sidebar.vue](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/frontend/src/components/Sidebar.vue), [entry form](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/frontend/src/components/AddDeviceModal.vue), [device table](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/frontend/src/components/DeviceTable.vue), [API](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/api/handlers.go), [poller](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/monitor/poller.go).

## 3. Navigation and information architecture

| Destination | Records | Principal display and actions |
|---|---|---|
| Genel Bakış | Aggregate of three mutually exclusive monitoring categories | Category counts, current reachability, active incidents, correctly scoped telemetry |
| Ajan Özeti | Windows and Linux/Ubuntu hosts eligible for the existing agent workflow | `Tümü` / `Windows` / `Linux` tabs; agent availability, reachability, valid telemetry; the sole location for agent distribution |
| Web İzleme | Domain/URL records monitored with existing HTTP(S) logic | HTTP status, response time, last check and website incidents |
| IP Cihazları | NAS, ESXi, switches/routers and other explicitly IP-only devices | Simple ICMP table; optional inventory-type filter; dedicated explanatory notice |
| Olay Günlüğü | Existing persisted incidents across categories | Category/type/status filters, timestamps, duration and classification |
| Yedekleme Merkezi | Existing backup workflows | Preserve existing behavior and validate new metadata on restore |
| Sistem Ayarları | Existing authenticated settings | Existing alert-channel configuration; no new channel or protocol |

### Naming and action placement

- Use these exact visible navigation labels: `Genel Bakış`, `Ajan Özeti`, `Web İzleme`, `IP Cihazları`, `Olay Günlüğü`, `Yedekleme Merkezi`, `Sistem Ayarları`.
- `Ajan Özeti` replaces the proposed visible Servers label. It remains the Windows/Linux host inventory and agent lifecycle view, including hosts awaiting an agent; its name does not imply every listed host already has one installed.
- All visible text is Turkish. Use `Cihaz Ekle`, `IP Cihazı Ekle`, `Web Sitesi Ekle`, `Düzenle`, `Sil`, `Kaydet`, `İptal`, and `Ajan Dağıtımı` where applicable. Use `Erişilebilir`, `Ulaşılamıyor`, `Henüz Kontrol Edilmedi`, and `Bakımda` for the corresponding measured/monitoring states. Short Turkish supporting descriptions explain scope: Windows/Linux agent telemetry, HTTP(S) website availability, and NAS/ESXi ICMP reachability respectively. Do not rename HTTP checks as domain registration or expiry tracking.
- Menu wording is a presentation change. Keep compatible internal identifiers and map them centrally; do not rename API paths, database enums or stored preferences solely for stylistic consistency.
- Remove the standalone Agent Distribution action from the global sidebar. No distribution shortcut appears in Genel Bakış, the global toolbar, Web İzleme, IP Cihazları, generic add-device choices or settings.
- Show a contextual `Ajan Dağıtımı` action only while Ajan Özeti is active. Its contents use the existing authorized Windows/Linux distribution workflow; showing the button does not authorize automatic installation or new privileges.
- Per-host distribution/re-enrollment and eligible update actions also remain inside Ajan Özeti. Selecting another destination closes/reset the distribution dialog and clears any displayed provisioning material. Direct UI events or retained modal state must not reopen it outside that view.
- Dashboard agent cards navigate to Ajan Özeti; they do not open deployment directly. Global creation of a Windows/Linux host returns the operator to Ajan Özeti, where deployment is a separate explicit action.
- Keep agent installation state distinct from current host reachability and telemetry freshness. Count actual enrolled agents separately from all eligible hosts; do not count an awaiting-agent host as an installed agent.
- Preserve current backend authentication and device-mode enforcement. Restricting the distribution button to a screen is a UI scope rule, not a substitute for API authorization or the no-additional-attack-surface rule.

The old NAS destination is consolidated into IP Cihazları. Legacy frontend `nas` navigation is redirected internally to that view with its NAS filter selected. This is a frontend state mapping, not a new public HTTP endpoint. Preserve stored navigation preferences if any exist.

Power/temperature content moves into an optional server-telemetry section of Genel Bakış, showing only real available data. It is not presented as universal power or room-health monitoring. No dedicated hardcoded healthy-power screen remains.

Each destination has its own search/filter state and count. Category changes clear incompatible selections and filters. Dashboard cards and incident links route through one shared device-to-view mapping so NAS/ESXi alerts open the IP view rather than the server inspector.

## 4. Device identity and monitoring contract

Separate inventory identity from the permitted monitoring behavior:

- Keep `device_type` for Windows, Linux/Ubuntu, ESXi, NAS/vendor aliases, switch/router and other inventory labels.
- Introduce a closed, server-validated `monitoring_mode` enum: `server`, `website`, `icmp`. `server` preserves the existing Windows/Linux workflow and already-authorized agent telemetry; it does not automatically install an agent or add service checks. `website` uses existing HTTP(S) behavior. `icmp` permits only the established IP reachability path.
- Derive the three navigation categories consistently from monitoring mode. Never guess HTTP behavior from an IP field after normalization/validation. Do not classify the same record into multiple monitoring categories.
- NAS/ESXi records are constrained to `icmp`. A Windows/Linux machine may be explicitly registered as an IP-only target without agent capabilities; such a record appears in IP Cihazları and is counted there once.
- Mode is a validation/policy field, never an arbitrary executable handler name, script, plugin URL, or extensible remote configuration object.
- Determine effective mode for legacy records on the server and return it consistently to clients. Share frontend presentation mappings and backend enforcement semantics; avoid unrelated string heuristics in each component.

Proposed additive fields: `monitoring_mode`, a nullable `last_checked_at`, and a telemetry-sample timestamp where currently missing. Retain existing IDs, IP uniqueness, incident relationships and response-time compatibility. New timestamps must originate from actual completed observations, not page refresh time. Do not introduce per-poll raw log tables unless independently justified by retention/capacity review.

### Complete network-discovery removal — TASK-001

- Remove the discovery sidebar item and every scan action from dashboard headers, server/IP views, host cards, device tables and empty states. Empty states offer manual entry appropriate to their category.
- Remove `DiscoveryModal.vue`, its import/state/events/mounting, and the frontend `scanSubnet` client method. No global, hidden, direct-link or retained-state path may invoke scanning.
- Delete the registered `POST /api/discovery/scan` route, `TriggerDiscovery` handler/imports, the dedicated `internal/discovery` scanner implementation and obsolete scanner tests. Remove `DiscoveryHost` if no callers remain. Do not retain the scanner behind a flag or ship archived executable scanner code in the active application.
- An authenticated or anonymous request to the retired API path must return the existing API not-found response and perform no network work. No replacement scanning endpoint or privileged helper is introduced.
- Retire `DEFAULT_SCAN_SUBNET` from installer/Compose/example configuration and active documentation, and stop seeding/reading the discovery-only `scan_subnet` setting. Handle old stored/restored values as retired configuration, without reactivating scanning. Never edit a customer's live environment file during implementation without the applicable deployment authorization.
- Distinguish discovery settings from shared polling settings: retain `scan_interval_seconds` or other fields still used by ordinary device checks until their actual consumers have been reviewed. Do not remove settings by a broad name/prefix match.
- Keep `monitor.PingHost`, individual-device scheduling, gateway reachability, existing device records and incident history. Previously discovered devices remain registered inventory; removal of their creation mechanism must not delete them.
- Regenerate the embedded frontend during the approved delivery phase so obsolete scan UI/client code is not left in the shipped bundle. Historical audit/changelog records remain historical; update active guidance to reflect removal.
- TASK-001's earlier agent-required rationale is superseded: IP Cihazları intentionally supports agentless manual registration and ICMP-only monitoring. Removing discovery must not force agent installation on NAS/ESXi or change their security boundary.

## 5. Category-specific device entry

### Ajan Özeti

- Entry actions are Windows server and Linux/Ubuntu server, within one servers destination.
- Fields: name, IP, OS family, optional descriptive location/notes, and supported monitoring profile.
- Display existing agent availability separately from ICMP reachability. No agent means no invented CPU/RAM/disk values.
- Existing agent enrollment remains a separate authenticated, explicitly selected workflow accessible only from Ajan Özeti. Remove the global sidebar entry; do not place deployment in other categories or global menus. This redesign does not authorize automatic installation, increased privileges, or an expanded update channel.

### Web İzleme

- Dedicated name and HTTP(S) URL fields. Explain that this checks website reachability/HTTP response, not registrar expiry, DNS security, or certificate inventory unless those features already exist and are accurately represented.
- Remove hardware type, NAS/ESXi, agent and hardware-telemetry controls from this flow.
- Preserve existing URL behavior and security restrictions; do not expand URL schemes, redirects, egress destinations, or credentials to support the redesign.

### IP Cihazları

- Fields: device name, IP literal, inventory type (NAS, ESXi, network device, other), optional location/notes, monitoring profile and supported interval.
- IP only: reject hostnames, URLs, ports, paths, embedded credentials and malformed/multicast/unspecified destinations. Define acceptable unicast address scope consistently with current customer-network policy; do not invent routes to unreachable segments. Reuse the existing validated ping path and supported address families.
- Never ask for NAS/ESXi/iLO/iDRAC credentials. Do not expose hardware metrics, agent installation/upgrades, service addition, port scan or protocol-discovery controls.
- Use a new explicit creation context (`category`/`initialMode`) rather than a fabricated edit record. Editing requires an existing ID. Reset drafts correctly on close, reopen and category switching.
- If IP editing is supported in the new UI, update and validate it atomically in the existing API; otherwise render it explicitly read-only. Do not offer an input the backend ignores.

Global Add Device opens a short category chooser; category-specific Add buttons open the appropriate form directly. Server validation must independently enforce the category contract regardless of the UI.

## 6. IP Cihazları page, table and explanatory copy

Page contents:

1. Title and Add IP Device action.
2. Persistent concise security/scope notice, with an optional expanded explanation.
3. Small counters: total, reachable, unreachable, and not yet checked/monitoring suspended. Counters use actual category data.
4. Search and inventory-type/status filters.
5. Dedicated simple table and a genuine empty state.

Table columns: device name, type, IP address, reachability status, ping RTT, last check, and actions. Location may appear as secondary name text. Actions are edit, delete, maintenance/resume and view incidents as supported by existing authorization. A compact detail area may show last successful response and related incidents, but no hardware-telemetry inspector.

Notice intent, to be localized accurately:

> These devices are monitored only for network reachability through ICMP ping. Watchtower does not enable additional management services or use SNMP, device APIs, SSH or agents for this category. This preserves the project's attack-surface constraint. Disk, RAID, CPU and memory health are not collected. A missing ping response may indicate network loss or ICMP filtering and does not prove that the device is powered off.

Do not claim that ping makes the whole application risk-free, that every device accepts ICMP, or that an online device has healthy storage. Do not instruct users to enable a management protocol or open broad firewall access to resolve unavailable data. If existing network policy blocks ICMP, show that the device cannot be assessed through the permitted method.

Ping latency is shown only for a successful, fresh observation with a valid measured RTT. Offline/stale/unknown samples show a dash. A true zero/missing/parse-failed value must not be presented as a measured zero or automatically as sub-millisecond latency.

## 7. Server and website tables

- Replace the oversized all-purpose table branches with explicit `ServerTable`, `WebsiteTable` and `IpDeviceTable` views/components, sharing small safe presentation primitives where useful. Avoid nesting more mode booleans into `DeviceTable.vue`.
- Server columns: name, OS/IP, reachability, agent telemetry availability, CPU/RAM/disk when current real samples exist, last observation and authorized actions. No hardware bars for appliances.
- Website columns: name, URL, HTTP result, response time, current availability and last check; no agent/hardware controls.
- Keep existing table search, selection, pagination/filter behavior and edit/delete authorization consistent. Disabled/nonfunctional row-selection controls should be removed unless backed by a real scoped action.
- Keep the existing visual language, responsive layout and accessible labels; support empty, loading, failed-fetch, stale and filtered-empty states. Fetch failures must not masquerade as empty inventory or all-online status.

## 8. Dashboard redesign

- Top-level summary shows inventory total and current status distribution, with unknown and suspended monitoring explicitly accounted for.
- Three clickable category cards: Ajan Özeti, Web İzleme and IP Cihazları, each with its own total/reachable/unreachable counts and the appropriate destination.
- Inventory breakdown within IP Cihazları may show NAS/ESXi/network counts without duplicating them in the top-level total. Count devices/records, not monitoring sources.
- Active incidents show device name, category, target, start time and elapsed duration, and open the correct filtered list/history.
- Existing hardware metric summaries use only fresh agent-backed server samples. Show their source and sample population/time; do not label host CPU temperature as measured room temperature.
- Rename the instantaneous online ratio to a current reachability ratio. Do not advertise 24-hour uptime/SLA until historical coverage, exclusion rules and calculation are actually implemented and tested; no new historical SLA feature is included in this scope.
- Maintenance/paused/dormant profiles and stale observations must not be silently classified as currently healthy or as confirmed outages. Document numerator/denominator definitions for each summary.
- Remove unconditional power-good assertions, unsupported IPMI collection claims and hardcoded outage-correlation thresholds. If an existing correlation result is displayed, label it a hypothesis supported by recorded incidents, not measured power state.

## 9. Backend enforcement and incident behavior

### ICMP execution boundary

- Choose the probe by validated effective mode before any network action.
- The `icmp` path cannot enter HTTP/TCP service polling, protocol discovery, hardware collection, agent provisioning/report ingestion, or upgrade orchestration. Reject attempts through existing protected service/upgrade APIs as well as through forms.
- Device creation in `icmp`/`website` mode does not issue agent credentials. Enrollment and heartbeat cannot silently convert an IP-only record into a server or overwrite its mode/status through agent reports.
- Preserve legacy service configuration for audit/history if present, but make it ineligible for execution on an ICMP-only device. Do not silently delete records. Backend imports must enforce the same rule.
- No new network listeners, target-side changes, dependencies, credentials, firewall changes or additional probes are introduced. Use existing per-device scheduled checks, bounded timeouts/concurrency and validated destinations.

### Status, cadence and alarms

- User-facing status means reachable/unreachable by the selected method, not proven power on/off. Unknown means no trustworthy observation yet; stale means an observation is too old to describe current reachability. Maintenance/paused are monitoring states rather than hardware-health results.
- Reuse persisted downtime/recovery incidents and existing configured notification channels. First actual failed observation may open an incident according to the existing alert policy; record creation, page navigation, category migration or a stale value alone must not fabricate an outage.
- Open one incident on an online-to-unreachable transition; repeated failed checks must not create one incident or alert per poll. Recovery closes the relevant active reachability incident, persists duration and emits the existing recovery notification once.
- Preserve any existing configured cooldown/correlation behavior and verify its interaction with IP devices. A correlation hypothesis must never replace the underlying reachability evidence with a claimed physical power failure.
- Persist/return actual last-check time separately from last successful response. Mark timeout/local probe error distinctly in diagnostic evidence where needed; do not expose credentials or command output containing sensitive data.
- Respect supported configured cadence using last-check time, not last-success time. Default retains the existing 15-second baseline; do not expose a faster interval than the scheduler supports or add retry traffic as a side effect. Validate per-device intervals consistently in UI/API.
- Maintenance and suspended profiles do not emit new reachability-loss alerts while intentionally unchecked. Resume performs a fresh scheduled check; migration does not close/open incidents merely because a label changed.

## 10. Data migration and backward compatibility

1. Perform a read-only inventory preflight: count legacy aliases, agent associations, attached services, open incidents, malformed addresses and unknown types. Redact actual customer identifiers in any committed notes.
2. Backfill effective categories deterministically: website/web/HTTP URL records to website; Windows/Linux/Ubuntu/Debian server records to server; NAS/vendor aliases, ESXi, network and non-agent appliances to ICMP. Preserve the inventory label where meaningful.
3. Conflicts such as a NAS-labeled active agent, URL stored as an ESXi address, unknown agent-backed type, or incompatible service config require explicit migration classification. Produce a migration summary; do not silently widen privileges, issue credentials, change probe type or discard telemetry/history. Resolve ambiguous entries before applying their conversion.
4. Apply additive idempotent schema migration and policy enforcement without changing device IDs or incident foreign keys. Migration is local and does not probe devices, install agents, enable protocols or send notifications.
5. Preserve historical telemetry but exclude appliance history from current server-health summaries after correct classification. Retain existing incident timelines.
6. Include new fields in backup export/import. Infer modes safely for older bundles; validate every restored field and attached service. Restore cannot bypass ICMP-only restrictions or cause agent token issuance. Define bundle-version compatibility and test older exports.
7. Document rollout/rollback compatibility. A mode column alone is insufficient rollback protection if an older executable ignores it and resumes old service checks. Provide an explicit downgrade procedure or prohibit incompatible downgrade after conversion; do not assume schema addition makes rollback safe.

## 11. Implementation phases and proposed file boundaries

| Phase | Work | Expected touchpoints | Completion evidence |
|---|---|---|---|
| 0 | Review/approve category design, copy, policy and migration rules | This document; ADR 0006 | Explicit operator approval before code changes |
| 1a | Fully remove network discovery/subnet scanning under TASK-001, preserving per-device ping | `DiscoveryModal.vue`, `App.vue`, `Sidebar.vue`, `DeviceTable.vue`, `HostCards.vue`, `services/api.js`, discovery route/handler/package/model, discovery-only settings and active deployment documentation | Retired API returns not-found; no scan actions/client calls/compiled scanner remain; registered-device ping and history are preserved |
| 1 | Central mode classification, schema/preflight and constrained request validation | `models/models.go`, `database/db.go`, `api/handlers.go`, `backup/manager.go`; small shared classification helper | Legacy mapping and invalid-input tests; existing endpoint/auth compatibility |
| 2 | Enforce ICMP-only execution and agent/service/upgrade exclusions; accurate check cadence/timestamps | `monitor/poller.go`, relevant agent handlers and API validation | Negative assertions that ICMP records cannot reach other collectors or side effects |
| 3 | Navigation and category-specific entry; separate list views | `Sidebar.vue`, `App.vue`, `AddDeviceModal.vue`, dedicated table/view components, shared frontend device mapping | Each record appears in exactly one category; add/edit context and direct navigation work |
| 4 | Dashboard, row details and incident navigation | `App.vue`, `HostCards.vue`, `TelemetryInspector.vue`, stats API, incident presentation | Counts reconcile; stale/missing telemetry and scope labels are truthful |
| 5 | Security/regression checks, migration/backup round trip and visual review | Focused tests and build artifacts as appropriate | Acceptance matrix below passes; no live-customer probing |

Do not start a broad framework rewrite, introduce another frontend dependency, change provisioning/update cryptography, or implement the deferred SNMP/API roadmap as part of this work. Limit new helper code to validated classification, rendering and enforcement of the existing collection boundaries.

## 12. Acceptance and validation matrix

| Area | Required acceptance evidence |
|---|---|
| Discovery removal | No discovery modal/buttons/events/client methods or runtime scanner/route remain; retired API returns not-found even with a valid session and triggers zero probes; delivered embedded assets contain no callable discovery UI |
| Discovery compatibility | Previously discovered records and their incidents remain; manual IP registration and scheduled ping still work; older backups cannot restore scanning; shared polling settings remain functional |
| Category isolation | Mixed Windows/Linux/web/NAS/ESXi/network inventory yields three disjoint lists whose totals equal overall inventory |
| IP-only network boundary | A controlled test double records only existing ICMP calls; no HTTP/TCP service requests, scans, credentials or agent/upgrade actions occur for an ICMP record |
| API bypass resistance | URL/hostname/port/mode-tampering inputs, service-add/upgrade calls and agent reports targeting ICMP records are rejected before side effects |
| Legacy and restore paths | Old aliases/bundles retain IDs/history, gain safe modes, and cannot re-enable excluded services; ambiguous data is reported rather than guessed |
| Incidents | A sequence of successful/failed/failed/recovered scheduled checks produces one open incident, no repeated alert per failure, one recovery, and recorded duration |
| Monitoring state | New/unknown, stale, maintenance, paused and resumed devices display accurately and do not fabricate downtime or healthy metrics |
| Dashboard math | Unknown/stale/suspended populations are explicit; no double counting; current ratio is not labeled historical uptime |
| Telemetry truthfulness | No appliance hardware bars, fabricated zero/sub-ms RTTs, stale server metrics rendered current, fake gateway IP or unsupported power/IPMI claims in revised screens |
| UI workflows | Category-specific add/edit/delete, validation failures, contextual buttons, filters, empty/loading/error states, incident links and old NAS navigation behave correctly |
| Agent distribution placement | Ajan Dağıtımı and per-host provisioning actions are visible/openable only in Ajan Özeti; no global/sidebar/other-view shortcuts; navigation closes the modal and clears provisioning material; existing API authorization remains enforced |
| Naming and counts | Exact Turkish navigation labels are consistent across sidebar/page headings/dashboard links; all visible interface copy is Turkish; eligible hosts, installed agents and fresh reporting agents are not conflated |
| Delivery | Focused backend tests, relevant existing regression tests, frontend production build and zero-CGO server/agent build as required by touched code; Windows build if agent code changes |
| Exposure | Evidence shows unchanged listeners/protocols/credentials/privileges, reduced dispatch ambiguity and no new target-side changes; unresolved expansion blocks release |

Use synthetic records, in-process handlers and controlled probe substitutes for tests. No customer scans, real alert messages or production service changes are needed for validation. Existing pending security findings are separate remediation debt and must not be described as closed merely because these interface tests pass.

## 13. Default decisions submitted for review

- One new IP Cihazları menu with NAS/ESXi/type filters, rather than a separate menu for every vendor or product version.
- One Ajan Özeti menu with Windows/Linux tabs and the only agent-distribution entry point; Web İzleme retains the website list. No visible Servers menu label.
- Reachable/unreachable terminology with a clear ICMP explanation rather than a claim of confirmed power state.
- A simple IP table and existing incident history/alerts; no hardware health collection or new historical SLA subsystem.
- Backend-enforced monitoring mode, safe migration and truthful dashboard totals, not merely frontend filtering.
- Full network-discovery removal as TASK-001, including UI, API, implementation and retired settings; preserve only individual registered-device reachability checks and existing authorized agent enrollment.

Implementation and local validation completed. See [validation and exposure report](../.memory-bank/audits/monitoring-redesign-validation-2026-10-16.md) for evidence, compatibility limits and downgrade requirements. No production rollout was performed.
