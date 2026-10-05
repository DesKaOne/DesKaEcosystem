# IndoChain State Implementation v0.1

> Status: Development implementation baseline
> Scope: Account-state mechanics only

## Purpose

Provide the first executable state boundary for deterministic account-state experiments without freezing the final state tree, persistence format, or StateRoot algorithm.

## Account

The development account contains:

- native balance (uint64);
- nonce (types.Nonce).

Contract code, storage commitments, metadata, and other VM state are intentionally deferred.

## State Operations

The initial in-memory state supports:

- Get;
- Set;
- Delete;
- deterministic single-account transfer semantics;
- snapshot;
- replacement from a snapshot.

The implementation is concurrency-safe for node-local development use.

## Transfer Semantics

A transfer requires:

1. a sender account;
2. an expected sender nonce matching the stored nonce;
3. a non-zero amount;
4. sufficient sender balance.

On success:

- sender balance decreases by the amount;
- sender nonce increments by one;
- recipient balance increases by the amount;
- a missing recipient is created implicitly with zero nonce.

No fee charging is implemented yet.

## Intentionally Not Frozen

This implementation does not define:

- canonical state serialization;
- persistent storage engine;
- state trie/tree structure;
- StateRoot algorithm;
- proof format;
- fee ordering;
- failed-execution semantics;
- contract storage model.

Those remain governed by the state-transition and serialization specifications.
