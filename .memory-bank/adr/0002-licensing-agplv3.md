# ADR 0002: Selection of GNU Affero General Public License v3 (AGPL-3.0)

- **Status**: Accepted
- **Confidence**: Verified
- **Date**: 2026-10-11
- **Supersedes**: None
- **Superseded By**: None

## Context
GNNwatchtower is an open-source network and server infrastructure monitoring appliance intended for deployment across client networks and managed service providers. Because the software interacts with users and administrators over local computer networks (web dashboard on port 3800), standard copyleft licenses (GPLv3) suffer from the Application Service Provider (ASP) / SaaS loophole: third parties or competing MSPs could modify the platform, host it as a closed service, and refuse to contribute modifications back to the community.

## Decision
Adopt the **GNU Affero General Public License, Version 3 (AGPL-3.0)** as the official repository license.

1. **Copyleft Protection (Section 13)**: Any party modifying GNNwatchtower and executing it as a network service must provide the corresponding source code of their modified version to network users free of charge.
2. **Protection Against Proprietary Expropriation**: Prevents third-party SaaS providers or competing managed service providers from stripping branding and redistributing GNNwatchtower as a proprietary closed-source product without opening modifications.
3. **Dual-Licensing Foundation**: Leaves open the standard open-core/dual-licensing path where the community edition is AGPL-3.0, and closed-source enterprise/commercial licensing exceptions may be granted by the author.

## Consequences
- Protects the project's intellectual integrity and community contributions.
- Clear legal obligation for network deployments.
- Ensures all forks and extensions remain open source.

## Evidence
- `LICENSE` file in repository root
- Remote GitHub repository: `https://github.com/BigDesigner/project-gnnwatchtower.git` commit `c5fc144dfe7a233491d20f968130d8d0a948eb19`
