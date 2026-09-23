# Consensus ↔ Block Candidate Construction Boundary v0.1

## Purpose

The block-production boundary now has a deterministic candidate-construction primitive in addition to proposal validation.

`BuildBlockCandidate` takes an explicit transaction sequence, executes it against a snapshot of canonical state, calculates the development transaction root and resulting state root, and constructs the next block candidate.

## Flow

```text
Canonical State
     │
     ├── snapshot
     │
     ↓
Explicit Transaction Sequence
     │
     ├── validate + execute
     │
     ↓
Working State
     │
     ├── State Root
     │
     └── Transactions Root
              │
              ↓
        Block Candidate
              │
              ↓
      ValidateProducedBlock
```

The caller remains responsible for transaction selection. This keeps mempool ordering/selection separate from block construction.

## Implemented invariants

The builder requires:

- a non-nil canonical state;
- a valid consensus round state;
- a non-empty proposer;
- a non-negative timestamp;
- execution rules matching protocol version and chain ID;
- every supplied block transaction to be a supported development transaction;
- every transaction to pass the existing state execution boundary.

The resulting candidate uses:

- next height = current height + 1;
- current chain ID and protocol version;
- supplied timestamp;
- supplied previous block hash;
- deterministic transactions root;
- resulting deterministic state root;
- supplied proposer;
- supplied consensus evidence.

The caller's canonical state is not mutated.

## Current limitation

The existing v0.1 transaction execution rule carries one explicit public key. Therefore this builder intentionally does not invent a sender-to-public-key registry for multi-sender blocks.

A production block builder needs a canonical authority/key-resolution design before arbitrary multi-sender transaction execution can be treated as finalized protocol behavior.

## Not frozen

This milestone does not freeze:

- mempool transaction-selection economics;
- fee/gas accounting;
- block gas/byte limits;
- canonical block serialization;
- public-key registry;
- proposer selection policy;
- production BFT semantics;
- persistence;
- P2P block proposal transport.

## Tests

Tests cover deterministic empty-block construction, next-height calculation, state-root preservation, canonical-state non-mutation, execution-context mismatch, and nil-state rejection.