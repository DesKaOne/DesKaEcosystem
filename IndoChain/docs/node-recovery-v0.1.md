# Node Recovery Boundary v0.1

## Purpose

Define how an IndoChain node opens an existing chain store without silently resetting the chain to genesis.

This is a development boundary for indochain-v0.1. It prepares the node lifecycle for persistent storage while keeping storage durability and canonical serialization outside the current implementation scope.

## Lifecycle

The node has two initialization paths:

1. NewDevnet(store) initializes a fresh Devnet store from the canonical Devnet genesis.
2. OpenDevnet(store) opens an existing store, or initializes it from genesis when the store is empty.

An existing non-empty store MUST NOT be overwritten during recovery.

## Recovery Validation

OpenDevnet validates:

- chain configuration against the Devnet profile;
- stored head availability;
- stored state availability;
- head chain ID;
- head protocol version;
- every stored block from genesis through the head height;
- each stored block height matches its storage key;
- each stored block chain ID and protocol version;
- each stored block hash against a recomputed block hash;
- non-zero stored block hashes;
- genesis block identity;
- each non-genesis PreviousHash against the preceding stored block hash;
- stored head hash against the final historical block hash;
- stored state root against the head state root when the header contains one.

Historical validation prevents a store from appearing healthy merely because its latest block and state are internally consistent while an earlier link in the chain is missing or corrupted.

If any consistency check fails, recovery returns an error instead of creating a fresh chain.

## Persistent Recovery Integration Test

The node test suite exercises the recovery boundary against the development FileStore implementation.

The integration path is:

~~~text
FileStore(path)
    │
    ▼
NewDevnet()
    │
    ▼
persist genesis block + state
    │
    ▼
new FileStore(path)
    │
    ▼
OpenDevnet()
    │
    ├── recover head
    ├── recover state
    └── validate stored history
~~~

The test verifies that the reopened node preserves:

- head height;
- head hash;
- state root;
- the ability to import another block after recovery.

Additional recovery tests cover:

- missing historical blocks;
- broken historical parent hashes.

These tests are intentionally focused on the storage-to-node lifecycle. They do not freeze the FileStore format as a protocol format.

## State Isolation

The recovered state is returned as a snapshot from storage.

The node does not retain a mutable reference to the storage-owned state object.

This preserves the existing storage isolation rule:

~~~text
Storage State
     │
     │ LoadState()
     ▼
Node State Snapshot
~~~

## Empty Store

An empty store is identified through the storage boundary's ErrEmptyStore.

For an empty store:

~~~text
OpenDevnet()
    │
    ├── store empty
    │
    ▼
NewDevnet()
    │
    ▼
Devnet Genesis
~~~

This behavior is intentionally explicit. A store that is non-empty but inconsistent is not treated as empty.

## Corruption Handling

Recovery is fail-closed.

Examples:

~~~text
stored head hash != recomputed block hash
        │
        ▼
ErrHistoryMismatch / ErrBlockHashMismatch
~~~

or:

~~~text
block[N].PreviousHash != hash(block[N-1])
        │
        ▼
ErrHistoryMismatch
~~~

or:

~~~text
head.StateRoot != loadedState.Root()
        │
        ▼
ErrStateRootMismatch
~~~

The node must not continue from a state that fails these checks.

## Persistent Storage Boundary

MemoryStore remains a development implementation.

A future persistent implementation MUST provide:

- durable block storage;
- durable state storage;
- durable head metadata;
- atomic block/state commit;
- crash-safe recovery;
- corruption detection;
- appropriate locking/concurrency behavior.

OpenDevnet is designed to consume those guarantees through ChainStore; it does not implement persistence itself.

## What This Does Not Freeze

This document does not freeze:

- database engine;
- block serialization;
- state serialization;
- storage schema;
- database indexes;
- snapshot format;
- WAL/journaling strategy;
- canonical block hash algorithm;
- canonical state commitment;
- pruning;
- archival storage;
- snapshot sync.

Those remain separate protocol or implementation decisions.

## Relationship to Financial Integration

Financial services such as DesKaCash remain outside the node storage boundary.

Their application databases must not be treated as IndoChain node state.

Conceptually:

~~~text
DesKaCash Application Database
          │
          │ application data
          ▼
     DesKaCash Service

IndoChain Node Storage
          │
          │ blockchain state
          ▼
       IndoChain
~~~

Settlement between the two layers happens through documented public integration interfaces rather than by sharing internal storage.

## Status

This document records the intended node recovery boundary for indochain-v0.1.

It is an implementation guideline, not a final persistence specification.
