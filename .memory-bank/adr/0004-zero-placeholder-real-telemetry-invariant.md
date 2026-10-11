# ADR 0004: Golden Rule - Zero Placeholders and Absolute Data Veracity

- **Status**: Accepted
- **Confidence**: Verified
- **Date**: 2026-10-11
- **Supersedes**: None
- **Superseded By**: None

## Context
In infrastructure, network, and server monitoring appliances (MSP model), presenting hardcoded placeholder metrics (e.g. static "42.4°C" temperatures, hardcoded "2.4 Gbps" traffic rates, or synthetic device lists) creates false operational impressions, masks unwired backend integration flaws, and undermines operator trust. The operator explicitly commanded that no placeholders may exist in the system and that all visible data must be real.

## Decision
Establish the **Golden Rule of Absolute Data Veracity** as an immutable project invariant across all frontend components and backend services:

1. **Zero Hardcoded Placeholders**: No synthetic metrics, fake temperatures, dummy network bandwidths, or simulated device arrays may be hardcoded into frontend templates or components.
2. **End-to-End Real Binding**: Every metric, status LED, progress bar, table row, sparkline graph, and log entry must be populated from live backend API responses (`/api/devices`, `/api/metrics`, `/api/incidents`, `/api/stats`, `/api/system/gateway-health`) backed by real SQLite records or direct OS measurements.
3. **Authentic Empty States**: When an appliance is freshly deployed or when a device/metric has no telemetry yet, the UI must render an authentic, unambiguous empty state (e.g., `—`, `Veri Bekleniyor`, or `Henüz Cihaz Eklenmedi - Ağ Taraması Başlatın`), never fake placeholder data.
4. **Backend Telemetry Aggregators**: Real backend API endpoints must be implemented for all header and dashboard summary widgets (e.g. local host gateway latency, aggregate node counts, live network socket throughput, and server temperatures).

## Consequences
- Guarantees 100% operational transparency and reliability.
- Exposes unwired endpoints immediately during testing (anti-illusion rule).
- Mandates that every UI element has a genuine backend data provider.

## Evidence
- Operator Directive: *"hiçbir zaman placeholder istemiyorum. yaptığımız herşey gerçek olmalı görünen herşey gerçek veri olmalı. bunu altın kural olarak yaz"*
- `.agents/AGENTS.md` (Zero Placeholder Invariant)
- `.specs/constitution.md` Section 1
- `.specs/boundary-conditions.md` Section 7
