# Consensus ↔ Mempool Deterministic Ordering Boundary v0.1

## Purpose

This document defines the development boundary between the transaction mempool and future block-production logic.

The current mempool stores transactions in a map keyed by transaction hash. Go map iteration order is intentionally not deterministic, so a raw mempool snapshot must not be used directly as a canonical block transaction sequence.

## Implemented boundary

`Pool.SnapshotSorted()` now returns a transaction snapshot ordered by the lexicographic transaction-hash string.

The method:

1. takes a read lock;
2. copies each transaction together with its already-known hash key;
3. sorts entries by the transaction hash;
4. returns only the transaction values;
5. leaves the mempool unchanged.

Because duplicate transaction hashes are already rejected by `Pool.Add`, the ordering is strict for all admitted transactions.

## Why this boundary exists

Block transaction order is consensus-sensitive because the transaction sequence contributes to the transactions root and state transition order.

Development architecture:

```text
Mempool
  │
  ├── Snapshot()       → unordered inspection only
  │
  └── SnapshotSorted() → deterministic development candidate order
                              │
                              ↓
                       Block Production
```

This prevents a future block producer from accidentally depending on Go map iteration order.

## What remains intentionally open

This does **not** freeze the production transaction-selection policy.

The final protocol may need to define:

- fee/gas priority;
- sender nonce sequencing;
- replacement rules;
- per-sender limits;
- transaction expiry;
- maximum block gas;
- maximum block bytes;
- invalid/stale transaction eviction;
- dependency-aware ordering;
- fee sponsorship ordering;
- canonical tie-breaking beyond the development hash order.

`SnapshotSorted()` is therefore a deterministic development primitive, not the final economic transaction-selection algorithm.

## Testing

Tests verify:

- repeated sorted snapshots have identical transaction order;
- transaction hashes are strictly ordered;
- taking a sorted snapshot does not mutate pool membership.

## Scope

This boundary does not implement:

- block production;
- state execution;
- gas/fee accounting;
- canonical transaction ordering policy;
- persistence;
- P2P mempool propagation;
- consensus finality.