# Consensus Finality ↔ Block Execution/Commit Boundary v0.1

## Purpose

The consensus layer now has an explicit authority boundary for binding a finalized block candidate to its finality certificate.

`ValidateFinalizedBlock` validates the block-production context, validates the finality certificate, and requires the certificate payload to equal the deterministic development block hash.

## Flow

```text
Block Candidate
      ↓
ValidateProducedBlock
      ↓
Development Block Hash
      │
      ├──────────────┐
      ↓              ↓
FinalityCertificate  Candidate
      │              │
      └──────┬───────┘
             ↓
  ValidateFinalizedBlock
             ↓
  Execution / Commit Boundary
```

## Separation of responsibility

Consensus is responsible for proving that validator authority finalized the exact candidate payload.

The execution/commit owner remains responsible for executing the candidate against canonical state and committing the resulting state/block atomically.

`FinalizedBlockCommitter` is the narrow interface between those responsibilities. It does not prescribe the concrete storage implementation.

## Important v0.1 limitation

The current transaction execution model still requires an explicit public key for signature verification. Therefore this boundary does not invent multi-sender key resolution. A concrete node integration must retain the existing execution authority context until a canonical public-key/validator authority registry is specified.

## Not frozen

This milestone does not define canonical block serialization, certificate encoding, P2P finality transport, production BFT semantics, timeout/round changes, validator-set transitions, fee/gas accounting, or a new public-key registry.

## Tests

Tests cover successful certificate-to-candidate binding and rejection when the candidate hash differs from the certificate payload.
