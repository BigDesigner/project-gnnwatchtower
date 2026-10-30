# Security remediation 0.4.12

## Exposure delta (before implementation)
Baseline: optional TLS, all-interface HTTP listener, root collector and NET_ADMIN container, arbitrary web DNS resolved twice, unbounded authentication and notification state. Changes restrict transport, bind default to loopback, remove broad firewall modifications and privileges, pin web connections to validated DNS answers, and bound queues/state. No target service, listener port, capability, credential for monitoring, protocol fallback or cross-customer path is added. Existing installed agents still need an authenticated TLS migration; no certificate-verification bypass is permitted.

Application changes are authorized by the operator. This is an implementation record, not a request for plan approval.

## Implemented controls

- S01: cryptographically random 24-byte bootstrap password, owner-only local file, no environment/default password reset; unfinished legacy default accounts rotate once. Completed accounts remain intact.
- S02: server TLS is mandatory; every agent HTTP client rejects plaintext, cross-origin credential redirects and certificate failures. Re-enrollment rejects unexpected signing key replacement. Installer generation rejects non-TLS requests. No TLS bypass was added.
- S03: dedicated no-proxy transport resolves and validates once, then dials the validated IP while preserving TLS hostname verification. Loopback, link-local and non-unicast destinations are denied. Private unicast targets remain allowed for intentional customer monitoring.
- S04: collector account, restricted write paths, no new privileges/capabilities, loopback default publication and no automatic firewall allow rule. Update migration installs the same collector restrictions. Existing deployment rules require operator review; local code cannot revoke remote infrastructure policies.
- S05: bounded 4096-entry IP/user/session state, expiry of both failure maps, input limits and eight concurrent login handlers. Random generation errors fail closed.
- S06: SQLite driver DSN applies required PRAGMAs to every connection, including recycled connections, without globally reducing the pool and risking nested-query deadlocks.
- S07: bounded four-worker/128-job dispatch queue, bounded cooldown map, three-minute repeated event suppression, HTTPS-only notifications and at most two 429 retries respecting Retry-After within a bounded request deadline. Queue saturation logs an explicit drop; incident persistence is independent.

## Compatibility and validation scope

Release metadata is 0.4.12; historical audit references and signature/backup fixtures retain their historical versions. Unknown installed agent versions are not replaced with the collector version. TLS migration instructions are in `docs/security-upgrade-0.4.12.md`. No tests or customer network probes were run. Compilation results are recorded after builds complete. These changes do not certify absence of other vulnerabilities; external firewall policy and trust configuration remain deployment responsibilities.

## Build evidence

Vite production build completed. Final Linux amd64 server and agent builds completed with CGO disabled and version 0.4.12 injected; Windows amd64 agent compilation completed with CGO disabled. Shell syntax checks passed for install/update/hardening scripts. Release metadata checks cover VERSION, frontend manifest/lock/fallbacks, Go fallback and Windows manifest/resource source. No functional tests, container execution, systemd deployment or live TLS migration were performed. Deployment migration is intentionally gated before stopping an unprepared HTTP installation.
