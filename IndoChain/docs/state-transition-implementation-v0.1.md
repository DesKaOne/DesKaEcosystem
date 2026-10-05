# IndoChain State Transition Implementation v0.1

> Status: Development implementation baseline

## Purpose

Define the first executable boundary between a validated transaction and node-local account state.

The implementation is intentionally narrower than the final protocol state-transition specification. It currently executes native-value transfers only.

## Execution Flow

~~~text
Transaction
    |
    v
Structural + signature validation
    |
    v
State snapshot
    |
    v
Native transfer execution
    |
    +---- failure ---> discard working state
    |
    v
Replace canonical in-memory state
~~~

A failed transaction must not partially mutate the supplied state.

## ExecutionRules

The development execution boundary accepts:

- transaction validation rules;
- public key material for signature verification.

Fee charging, VM execution, contract storage, gas accounting, and state commitment are deferred.

## Current Semantics

`ApplyTransaction`:

1. rejects a nil state;
2. validates transaction structure and signature;
3. requires sender and recipient;
4. executes against a snapshot;
5. commits the snapshot only after successful transfer.

The current transfer updates:

- sender balance;
- sender nonce;
- recipient balance.

A missing recipient account is created with nonce zero.

A self-transfer leaves the balance unchanged and advances the sender nonce.

Balance overflow is rejected.

## Atomicity

Snapshot-and-replace is a development mechanism for commit-on-success semantics.

It is not yet the final persistence or transaction-journal mechanism.

## Intentionally Not Frozen

This implementation does not freeze:

- fee ordering;
- gas charging;
- fee market;
- failed-transaction fee semantics;
- VM execution;
- contract storage;
- state tree/trie structure;
- StateRoot algorithm;
- state proofs;
- persistent database format;
- block-level execution ordering;
- consensus finality interaction.

Those remain governed by the protocol specifications under `docs/indochain/`.