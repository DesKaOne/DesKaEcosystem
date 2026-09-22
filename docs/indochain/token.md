# IndoChain Tokens

> Status: Draft / Token Design Baseline

## 1. Purpose

Token standards provide reusable asset primitives on top of the IndoChain execution layer.

## 2. Planned Standards

The roadmap proposes:

- IND-20 for fungible tokens;
- IND-721 for NFTs;
- IND-1155 for multi-token assets.

These names are design identifiers until formal specifications are frozen.

## 3. Fungible Token

IND-20 candidate capabilities:

- name;
- symbol;
- decimals;
- total supply;
- balance;
- transfer;
- allowance;
- approve;
- transferFrom;
- mint/burn where permitted.

## 4. NFT

IND-721 candidate capabilities:

- ownership;
- token ID;
- transfer;
- approval;
- metadata reference.

## 5. Multi-Token

IND-1155 candidate capabilities:

- multiple token IDs;
- balances per ID;
- batch transfer;
- metadata;
- mint/burn where permitted.

## 6. Authorization

Minting, burning, pausing, and administrative operations must have explicit authorization rules.

## 7. Metadata

Metadata references must not imply that canonical consensus depends on mutable external HTTP content.

## 8. Events

Token operations should emit deterministic events/receipts suitable for indexers and wallets.

## 9. Security

Standards should address:

- authorization;
- replay;
- integer overflow;
- approval race conditions where applicable;
- reentrancy where applicable;
- malformed metadata references;
- event consistency.

## 10. Open Decisions

- exact ABI/interface;
- native VM implementation;
- metadata conventions;
- permission model;
- upgradeability model;
- bridging representation.
