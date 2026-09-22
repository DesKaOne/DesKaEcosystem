# IndoChain Explorer

> Status: Draft / Ecosystem Design Baseline

## Purpose

The explorer provides public read-only visibility into blocks, transactions, accounts, validators, assets, and network activity.

## Data Flow

~~~text
IndoChain Nodes
      ↓
Indexer
      ↓
Query / API Layer
      ↓
Explorer Web UI
~~~

## Core Views

- latest blocks;
- block details;
- transaction details;
- account balances and activity;
- validator status;
- token and NFT metadata where supported;
- network statistics.

## Network Separation

Devnet, Testnet, and Mainnet must be clearly separated. Explorer pages must expose chain identity and network context.

## API

The explorer should consume indexed data through a dedicated query/API layer rather than coupling the UI directly to node storage.

## Security

Explorer data is informational. The UI must not become an authorization or signing authority.

## Open Decisions

UI framework, indexing backend, caching strategy, analytics schema, and public API limits remain open.
