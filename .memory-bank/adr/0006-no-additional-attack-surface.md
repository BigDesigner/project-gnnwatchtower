# ADR 0006: No Additional Attack Surface

Date: 2026-10-16. Status: Accepted by explicit operator instruction.

## Decision

**Watchtower's single governing golden rule is that monitoring must not introduce or expand attack surface.** Observability is subordinate to protecting the systems being observed. Reduced telemetry, unsupported devices, and unavailable metrics are acceptable outcomes; increasing exposure to obtain a more complete dashboard is not.

This supersedes earlier documents calling data veracity the golden rule. Truthful data and authentic empty states remain mandatory supporting requirements. The operator explicitly requested updates to the relevant governance Markdown files, including documents otherwise designated as restricted for routine autonomous edits.

This is a binding acceptance constraint, not a claim that the current code has no attack surface or that future code can be guaranteed free of vulnerabilities. Unknown exposure is a reason to reject/defer a collection path, not to label it safe.

## What counts as attack surface

Assess the monitored device, Watchtower, its agents, deployment tooling, storage, and any central management environment. Count network services and reachability, authentication/authorization, credential custody, execution privileges, remote actions and update channels, parsers and dependencies, egress/SSRF, resource exhaustion, and paths across customer boundaries. Reusing a port or applying a read-only role does not eliminate these considerations.

An existing customer service is not blanket authorization to connect. Creating a new monitoring identity or collecting its credential can expand trust even if no port changes. An outbound-only agent can still introduce privileged code, secrets, an ingestion path and an update mechanism. A protocol's encryption does not establish end-to-end safety.

## Prohibited tradeoffs

- Enabling SNMP, vSphere APIs, Redfish, IPMI, SSH, vendor management services, event listeners, or a new agent solely to obtain telemetry when doing so adds exposure.
- Opening firewall/NAT rules, exposing management interfaces externally, widening source access, or creating routes/VPN trust between otherwise isolated customers for monitoring convenience.
- Using administrator/root/domain-wide credentials or adding privileges to avoid missing metrics; reusing customer secrets across installations.
- Disabling certificate verification, accepting untrusted update keys from an unauthenticated channel, downgrading protocols, or enabling default/shared credentials to support older devices.
- Adding remote shell execution, repair/reboot/storage actions, package execution, or unrestricted device-management proxies under the label of health monitoring.
- Introducing a new syslog/SNMP-trap receiver or exporter and calling it passive or harmless without assessing its listener, inputs and privileges.
- Adding parsers, external services, libraries or install-time downloads without assessing their security effect. Optional features and disabled flags do not excuse vulnerable code that is still reachable.

## Permissible direction, subject to evidence

Prefer data already available within an existing authenticated, isolated and assessed workflow. Reuse still requires proof that the proposed change adds no attack surface or privileges. Use the narrowest existing data scope, preserve customer isolation, validate inputs and destinations, bound processing, and retain independent trust roots. Absence of a compliant path means no collection.

Do not automatically enable device capabilities or try alternative protocols/credentials. Capability discovery itself must remain within an assessed scope; blanket port probing or protocol fallback is not authorized by adding a device.

For NAS/ESXi hardware metrics, device family/version support is a compatibility goal only. It never authorizes activation of a management service. An appliance whose disk health cannot be read through a compliant existing path remains at reachability-only monitoring. The UI must show that disk health is unavailable, not infer healthy storage from ping.

## Required feature acceptance record

Before implementation/activation, record:

1. The evidenced deployment baseline: currently active services, listeners, routes, allowed sources, identities/permissions, existing telemetry and trust roots. Do not promote existing defects to an approved baseline.
2. The proposed delta: every added handler, input/parser, dependency, credential, privilege, outbound destination, listener, update/provisioning operation and tenant relationship, including indirect/install-time effects.
3. Threat paths and controls: compromised source/device/operator, credential disclosure, SSRF, malicious responses, resource exhaustion, update substitution, lateral movement and cross-customer access as applicable.
4. Concrete evidence that the change does not expand exposure, including security-specific negative checks and environment constraints. Label unverified assumptions explicitly. A happy-path test or a successful build is insufficient.
5. Rejection and failure behavior: unsupported, unavailable, stale and authorization-limited states; no security downgrade or alternate-service enablement.

If the delta is increased or unresolved, do not implement/activate that path. Ordinary feature approval or operator opt-in does not waive this requirement. A different tradeoff would require the operator explicitly revising this policy.

## Existing implementation and maintenance

Existing API listeners, agent execution/update paths, broad deployment permissions, and polling/probing remain subject to review and exposure reduction. This decision does not certify them. Hardening existing features takes priority over extending device coverage. Do not silently disable working services or change production deployments as part of a documentation task; record debt and handle remediation through authorized implementation work.

Current assessment: [attack-surface review, 2026-10-16](../audits/attack-surface-policy-review-2026-10-16.md).
