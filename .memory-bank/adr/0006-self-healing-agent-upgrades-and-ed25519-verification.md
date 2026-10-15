# ADR 0006: Self-Healing Cryptographic Agent Upgrades & Autonomous Zero-Touch Provisioning

- **Status**: Accepted
- **Confidence**: Verified
- **Date**: 2026-10-15
- **Category**: Auth
- **Supersedes**: None
- **Superseded By**: None

## Context
In production deployments of GNNwatchtower across multi-node enterprise environments (e.g. 100+ endpoints spanning Windows Server and Ubuntu Linux), managing remote agent lifecycles presented two critical operational and architectural challenges:

1. **Manual Intervention Bottleneck**: Inbound SSH, WinRM, or RDP access is often prohibited by enterprise security policies, firewall rules, and NAT/VLAN segmentation. Requiring manual terminal or PowerShell intervention on each client endpoint to upgrade agents or install cryptographic trust anchors makes fleet maintenance unsustainable at scale.
2. **Missing Trust Anchor Deadlocks**: Early agent installations (v0.4.4 through v0.4.7) were deployed without pre-provisioning the appliance's public signing key (`signer.pub`) in `C:\Program Files\GNNwatchtower-Agent\`. Consequently, when remote updates were dispatched, agents blocked the binary execution with a security violation (`Dogrulama anahtari yuklenemedi`), creating an unrecoverable upgrade impasse.

## Decision
We established a zero-touch, cryptographically signed, pull-based autonomous agent upgrade architecture consisting of four pillars:

1. **Self-Healing Cryptographic Key Handshake**:
   - The appliance server generates an Ed25519 key pair on initial boot (`data/signer.key` and `data/signer.pub`).
   - The server delivers the hex-encoded Ed25519 public verification key (`UpdatePublicKey`) within the routine `/api/agent/heartbeat` and `/api/agent/auto-enroll` responses.
   - The agent inspects its local filesystem upon each handshake. If `signer.pub` is missing or unreadable, the agent dynamically creates and persists the key to its secure root configuration directory (`C:\Program Files\GNNwatchtower-Agent\signer.pub` on Windows, `/etc/gnnwatchtower-agent/signer.pub` on Linux) with zero administrator intervention.

2. **Pull-Based Signed Binary Delivery**:
   - Monitored nodes poll the appliance via outbound HTTPS/HTTP heartbeats every 15 seconds.
   - When the appliance server version bumps (e.g. upon `update.sh`), the database setting `agent_latest_version` is updated.
   - The server signs the compiled binary's SHA-256 checksum with its Ed25519 private key.
   - The heartbeat response instructs outdated agents to upgrade with:
     ```json
     {
       "action": "upgrade",
       "upgrade_version": "0.4.10",
       "upgrade_url": "/api/agent/download?os=windows&arch=amd64",
       "upgrade_checksum": "<sha256>",
       "upgrade_signature": "<ed25519_signature>"
     }
     ```

3. **Atomic Binary Swap & OS Service Recovery**:
   - The agent downloads the new binary, computes its SHA-256 checksum, and cryptographically verifies the Ed25519 signature against `signer.pub`.
   - On Windows, where executing binaries cannot be overwritten directly due to OS file locking, the agent writes to a staging `.tmp` file and performs an atomic rename/replace.
   - The agent initiates an asynchronous service self-restart (`GNNwatchtowerAgent` service on Windows, `systemd` unit on Linux) and exits cleanly.
   - Upon service restart, the new agent binary immediately reports `agent_upgrade_status: "success"` and the active version string during its initial heartbeat.

4. **Dual-Control Upgrade Policy**:
   - **Autonomous Fleet Rollout**: Triggered automatically when the appliance is updated and devices are queued.
   - **Manual Operator Override**: Preserved in `frontend/src/components/DeviceTable.vue` with individual retry buttons and real-time status badges (`success`, `pending`, `failed`, `upgrading`) to handle transient failures or canary testing.

## Consequences
- **Zero-Touch Maintenance**: Upgrading the appliance server automatically cascades verified agent updates across the entire client fleet without touching monitored endpoints.
- **Strict Cryptographic Integrity**: Prevents man-in-the-middle binary tampering; unsigned or improperly signed binaries are rejected before disk execution.
- **Firewall & NAT Agnostic**: Zero inbound ports required on client endpoints; all operations flow through the outbound 15-second heartbeat loop.
- **Architectural Invariants Preserved**: Implemented entirely with pure Go zero-CGO dependencies and backward-compatible SQLite persistence.

## Evidence
- `backend/internal/api/handlers.go` (`AgentHeartbeat`, `AgentAutoEnroll`, `RequestAgentUpgrade`)
- `backend/internal/app/server.go` (`InitUpdateSigner`, `GetUpdatePublicKeyHex`, `SignUpdatePayload`)
- `backend/cmd/agent/main.go` (`saveSignerKey`, `verifyAndApplyUpgrade`, `autoEnroll`)
- `frontend/src/components/DeviceTable.vue` (Manual upgrade trigger, status badges, explicit `#ID` columns)
- Commit `4a1c871`: `fix(agent): deliver signer.pub in auto-enroll, save signer key locally, fix settings modal reactivity`
- Commit `6fc4520`: `feat(agent): self-healing signer key, explicit #ID columns in device tables, incident links, task deprecation`
- Commit `af94e89`: `fix(agent): suppress routine auto-enroll alerts, fix upgrade status overwrite and dispatch upgrade action`
