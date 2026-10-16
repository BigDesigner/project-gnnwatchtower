# Conservative appliance reachability monitoring

## Exposure assessment before implementation
Baseline: registered unicast IP targets, one operating-system ICMP echo per scheduled check, no management authentication or protocol fallback. Existing device workers are bounded at 20.
Change: enforce a 60-second minimum interval for ICMP inventory, allow three seconds for a reply, require three consecutive completed negative observations before opening an incident. Positive observations clear confirmation immediately. Local execution failures remain unknown and interrupt confirmation.
No new endpoint, listener, target port, credential, dependency, privilege, firewall exception, ARP scan or alternate protocol is introduced. Packet frequency decreases for existing 15-second IP inventory. OS neighbor resolution remains ordinary network-stack behavior.

## Semantics and limitations
An incident means ICMP reachability loss from this collector, never proven power failure or disk failure. Filtering by firewall/EDR and path outages cannot be distinguished from target failure using ICMP alone. No security-control bypass is attempted. Three failures at the minimum cadence require approximately two minutes after the first failed observation; scheduler load may delay detection further. Recovery uses one successful reply. Existing slower and dormant schedules remain respected. Confirmation is stored in the existing probe note, so no schema migration is needed. No new outbound notification mechanism is added.

## Delivery
Source changes only; production deployment requires the normal update workflow after publication. No live customer checks or tests were run.
