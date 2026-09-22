# IndoChain DEX

> Status: Draft / Ecosystem Design Baseline

## Purpose

A decentralized exchange may provide permissionless on-chain asset exchange after the core protocol and smart-contract environment are stable.

## Candidate Components

- liquidity pools;
- swap contracts;
- liquidity-provider positions;
- fee accounting;
- routing;
- price discovery;
- optional limit-order or order-book components.

## Architecture

~~~text
Wallet
  ↓
DEX Interface
  ↓
DEX Contracts
  ├── Pools
  ├── Swaps
  └── Fee Accounting
  ↓
IndoChain VM / State
~~~

## Security

The design must address reentrancy, arithmetic safety, oracle manipulation, sandwich/MEV risks, liquidity attacks, upgrade authority, and emergency controls.

## Dependency

DEX deployment should occur only after the transaction, gas, VM, token, and security baselines are stable.

## Open Decisions

AMM model, fee model, routing, governance, upgradeability, and MEV mitigation remain open.
