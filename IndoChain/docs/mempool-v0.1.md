# IndoChain Mempool v0.1

## Status

Development implementation and design baseline. Not protocol-frozen.

## Responsibilities

The mempool holds transactions that:

1. Have passed node-level admission checks.
2. Have not yet been included in a canonical block.
3. May be selected by a block builder/proposer.

The mempool is not canonical state.

## Current implementation

The development pool:

- indexes transactions by development transaction hash;
- rejects duplicate hashes;
- enforces a configurable capacity;
- supports lookup, removal, length, and snapshots;
- uses a mutex for concurrent access.

## Future admission pipeline

~~~text
Incoming Transaction
        ↓
Decode
        ↓
Structural Validation
        ↓
Signature Validation
        ↓
Chain / Replay Checks
        ↓
Nonce Checks
        ↓
Fee / Balance Checks
        ↓
Mempool
~~~

The exact nonce policy, replacement-by-fee policy, fee rules, per-sender limits, prioritization, eviction, persistence, and DoS controls remain TBD.

## Consensus boundary

Consensus must never treat arbitrary mempool contents as canonical state.

Only transactions included in an accepted block participate in state transition.
