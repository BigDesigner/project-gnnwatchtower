# Task: Deprecate Network Discovery & Subnet Scanning Module

**Status**: PENDING (Operator Deferred)  
**Priority**: Medium  
**Created At**: 2026-10-15  
**Origin**: Operator Architectural Decision  

## Description
With the GNNwatchtower platform adopting a strict Zero-Touch Auto-Enrolled Telemetry Agent architecture, passive/active ICMP/ARP subnet discovery (`internal/discovery/scanner.go`, `DiscoveryModal.vue`, and `/api/discovery/scan`) is considered redundant. Monitored assets must have the telemetry agent installed to report authentic system health, process list, temperatures, and hardware metrics. Unmanaged or agentless devices found via network scanning cannot report full telemetry without agent deployment.

## Planned Scope of Work
1. **Frontend**:
   - Remove "Ağ Taraması Başlat" (Trigger Scan) button from navbar and action bar.
   - Remove `DiscoveryModal.vue` component and all references in `App.vue`.
   - Remove "Ağ Keşfi (CIDR)" from sidebar navigation.
2. **Backend**:
   - Remove or deprecate `POST /api/discovery/scan` in `internal/api/router.go` and `handlers.go`.
   - Archive or remove `backend/internal/discovery/scanner.go`.
   - Clean up `DEFAULT_SCAN_SUBNET` setting if no longer needed.
3. **Database**:
   - Clean up any discovery-specific settings or columns if applicable.
