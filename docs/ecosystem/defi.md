# IndoChain DeFi

> Status: Draft / Ecosystem Design Baseline

## Purpose

DeFi applications may be built on top of the IndoChain VM and token standards after the core protocol is stable.

## Candidate Applications

- lending;
- borrowing;
- staking interfaces;
- liquidity management;
- collateralized assets;
- payment applications;
- yield strategies.

## Design Principle

DeFi applications must not be treated as part of the consensus-critical core unless a future protocol decision explicitly requires it.

## Risk Controls

Applications should define:

- collateral rules;
- liquidation behavior;
- oracle dependencies;
- interest calculations;
- emergency controls;
- upgrade authority;
- governance authority.

## Security

Independent contract testing and review are required before production deployment. Application-level risks must remain separate from node-level consensus security.

## Open Decisions

Application architecture, risk parameters, governance, incentives, and deployment order remain open.
