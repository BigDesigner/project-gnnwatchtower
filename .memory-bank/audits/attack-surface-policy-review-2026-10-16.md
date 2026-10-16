# Watchtower Attack-Surface Policy Review

Date: 2026-10-16. Governing decision: [ADR 0006](../adr/0006-no-additional-attack-surface.md).

Scope: documentation changes requested by the operator, followed by a source-based architectural assessment of current monitoring/deployment and previously suggested NAS/ESXi integrations. No device was contacted, protocol enabled, application source changed, test/build run, dependency installed, or deployment modified. This is not a replacement for the historical security audits or a complete revalidation of their findings.

## Assessment

The earlier proposal to add SNMP/vSphere/Redfish connectivity for broad device compatibility is **not accepted as an implementation plan under the new governing rule**. Compatibility and protocol encryption do not establish absence of added attack surface. Enabling disabled management services, adding new reachability, storing new privileged credentials, or introducing unassessed protocol code would conflict with the operator's priority.

Watchtower's current architecture already has security-relevant exposure. Updating its Markdown rules does not remove that exposure. No statement that the existing system meets a zero-risk or zero-attack-surface standard is supported by this review.

## Reassessment of NAS and server health collection

| Situation | Decision under the governing rule |
|---|---|
| NAS has SNMP disabled | Do not enable it for Watchtower. Keep reachability-only monitoring unless another compliant existing data path is demonstrated |
| NAS already exposes SNMPv3 on a restricted management segment | Candidate for assessment only. Existing enablement does not authorize new credentials, broader allowed sources, parser/dependency risk, or network reachability |
| ESXi/vCenter API is available only to an existing management system | Do not widen access or copy management credentials into Watchtower. Consider existing collected data only if reuse passes the same trust/input assessment |
| iLO/iDRAC/IMM is isolated or its API is disabled | Preserve isolation and disabled services. Do not expose it or enable a protocol to obtain disk metrics |
| Older hardware needs SSH, a privileged helper, weaker authentication, or certificate bypass | Reject that collection path; accept missing hardware-health telemetry |
| Existing agent already submits a supported metric over an assessed channel | Prefer reuse where the code/data/credential delta can be shown not to expand exposure; no automatic exemption for extending the report schema |
| Someone proposes a passive trap/syslog receiver or exporter | Treat it as a new service/input surface and reject/defer unless absence of added exposure is established |

No detailed model list is required to enforce these rules. A specific deployment baseline is required before declaring an integration compliant. Global support for ESXi 6/7/8 or a NAS family is not global authorization to activate its management interfaces.

## Current implementation observations

### 1. Watchtower API and dashboard are a listening surface

The server configures an address using only a port, binding to available interfaces, and retains an HTTP path unless TLS is configured. Compose publishes port 3800 without a host-interface restriction, and the installer adds an inbound UFW rule. Application routes include login, enrollment, report ingestion, downloads, settings, backup import and remote upgrade orchestration. These are existing exposure points requiring an explicit permitted audience; an HTTP service is not harmless merely because it is local to the customer network.

Evidence: [server startup](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/cmd/server/main.go:214), [API routes](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/api/router.go:39), [Compose networking](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/docker-compose.yml:28), [installer firewall rule](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/install.sh:163).

Policy implication: feature additions must not extend these routes, privileges or reachability without satisfying the governing acceptance rule. Existing deployment exposure needs remediation assessment before expanding coverage. The actual customer firewall/proxy configuration was not inspected.

### 2. Execution privileges amplify the impact of collector compromise

The installer runs the appliance as root. Compose grants NET_RAW and NET_ADMIN; the current source does not demonstrate that all those permissions are necessary. Monitoring should not receive extra privileges simply to support more devices. No privilege was removed during this documentation change.

Evidence: [systemd service identity](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/install.sh:140), [container capabilities](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/docker-compose.yml:16).

### 3. Discovery is active probing and requires scope/resource bounds

Discovery currently probes several TCP ports and uses ping/reverse DNS. Its IPv4 size limit constrains one scan, but a device inventory feature must not be treated as authorization for arbitrary new protocol probing, service activation, or repeated unbounded jobs. Ping supplies reachability, not disk health.

Evidence: [discovery implementation](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/discovery/scanner.go:15), [device polling](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/monitor/poller.go:128).

### 4. Outbound collection introduces inputs and internal-access risk

Website monitoring and HTTP service checks already issue outbound requests. Website checks use a host filter; the service-check path has a separate HTTP client. New NAS/ESXi request paths would add response parsers, credentials and potential access to sensitive management networks. Existing outbound code cannot serve as proof that another integration adds no exposure.

Evidence: [website HTTP client](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/monitor/poller.go:231), [service HTTP client](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/internal/monitor/poller.go:385).

### 5. Agents and self-update remain trust and execution boundaries

The current source now includes Ed25519 signature verification, so it would be inaccurate to describe the current agent as relying only on a checksum. However, HTTPS enforcement is opt-in. When a signer key is absent, code can obtain one from enrollment, an update instruction or the server's public-key endpoint. If that initial exchange occurs over unauthenticated HTTP, the authenticity of the initial trust anchor requires separate assessment. A key independently provisioned through a trusted path is a materially different case. Signing support alone does not certify secure provisioning.

Existing agents therefore cannot be described as having no attack surface merely because their normal communication is outbound. Installing another privileged agent to gain hardware visibility would also require the same no-additional-exposure assessment.

Evidence: [optional HTTPS requirement](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/cmd/agent/main.go:319), [enrollment signer storage](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/cmd/agent/main.go:288), [signer acquisition and verification](/Users/muratgonen/Documents/Antigravity/project-gnnwatchtower/backend/cmd/agent/main.go:656).

## Consequences for development

1. Broad NAS/ESXi/Redfish support is deferred as an implementation commitment; there is no authorization to enable protocols, add credentials, install agents or widen networks.
2. Every feature proposal must record before/after exposure, including the collector's own code and trust relationships. Unknown exposure blocks that path.
3. Prefer hardening existing entry points, privilege scopes, initial trust and outbound policies over increasing telemetry coverage. This is an assessment direction, not an executed remediation plan.
4. Model capability and telemetry freshness separately from health. Missing disk data must remain unavailable; a reachable machine is not thereby storage-healthy.
5. Preserve customer boundaries and accept less detail. No shared fleet administrator credential, management-network bridge or certificate bypass is justified by compatibility.

## Documentation recorded

- `.agents/AGENTS.md`: persistent developer/agent instruction and precedence.
- `.specs/constitution.md`: single governing golden rule and subordinate data veracity requirement.
- `.specs/boundary-conditions.md`: enforceable security boundary and rejection behavior.
- `.memory-bank/system-coherence.md`: aligned core invariants.
- `.memory-bank/adr/0006-no-additional-attack-surface.md`: decision, scope and feature acceptance criteria.
- `.tasks/pipeline.md`: existing NAS/ESXi backlog entries explicitly gated/deferred; old port-based suggestions removed.

Only Markdown documentation was changed. Link/content checks do not establish runtime compliance; no production safety guarantee is made.
