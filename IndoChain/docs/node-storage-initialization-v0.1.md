# Node Storage and Initialization v0.1

> Status: Development implementation
> Scope: Devnet bootstrap
> Persistence backend: not frozen

## 1. Boundary

The node owns the composition of genesis, canonical state, chain head, and storage.

The storage package exposes a small `ChainStore` interface so the node does not depend on a particular database implementation.

## 2. Development Store

`storage.MemoryStore` is the current development implementation.

It stores:

- blocks by height;
- block hashes;
- a snapshot of canonical state;
- the current highest stored block as the head.

It is intentionally non-persistent and is not suitable as a production database.

## 3. Devnet Startup

`node.NewDevnet(store)` performs:

1. load deterministic Devnet genesis configuration;
2. build genesis block;
3. calculate development block hash;
4. build initial state;
5. verify the genesis StateRoot;
6. persist genesis block/hash into the store;
7. persist a state snapshot;
8. return the initialized node with head and state.

Conceptually:

~~~text
Devnet Genesis
      |
      +----> Genesis Block ----> Block Hash ----> ChainStore
      |
      +----> Initial State ----> StateRoot ----> ChainStore
      |
      v
     Node Head
~~~

## 4. Atomicity Boundary

The current bootstrap sequence is intended for development only. A production store must provide durable commit semantics so block and state metadata cannot become inconsistent across crashes.

## 5. Persistence Roadmap

Future storage work should define:

- durable block storage;
- canonical head metadata;
- state snapshots;
- state database/trie;
- crash recovery;
- atomic block/state commit;
- pruning and archival policy;
- snapshot export/import;
- corruption detection;
- migration/versioning.

Those are intentionally outside this development implementation.
