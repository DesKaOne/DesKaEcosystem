# Consensus ↔ Execution Authority Handoff v0.1

## Purpose

The finalized-block boundary now has an explicit handoff point for execution signature authority.

`ExecutionAuthorityResolver` is supplied by the execution/node layer. Consensus only carries the validator identity and finalized block hash; it does not invent or persist a validator-to-public-key registry.

## Boundary

```text
Finality Certificate
        +
Finalized Block Hash
        +
Proposer / Validator ID
        ↓
FinalizedBlockAuthorization
        ↓
ExecutionAuthorityResolver
        ↓
Public Key
        ↓
Execution / Commit Layer
```

`ResolveProposerAuthority` validates the authorization object, requires an external resolver, resolves the proposer public key, and returns a defensive copy.

## Scope

This milestone establishes an explicit dependency-injection boundary. It does not define:

- a canonical validator registry;
- validator registration or lifecycle;
- public-key serialization in transactions;
- multi-sender block execution;
- production consensus authority rules;
- fee/gas accounting;
- persistent authority storage.

The existing v0.1 transaction model still does not contain a canonical sender public-key field, and the block execution API still accepts one explicit public key. Therefore this milestone intentionally stops at authority resolution rather than fabricating a multi-sender execution model.
