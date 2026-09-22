# IndoChain Storage

> Status: Draft / Storage Design Baseline

## 1. Purpose

Storage persists canonical blockchain data and supports deterministic state execution, recovery, synchronization, and node operation.

## 2. Logical Stores

The node should separate storage responsibilities conceptually:

~~~text
Block Store
├── Block headers
├── Block bodies
└── Commit/finality metadata

State Store
├── Account state
├── Contract state
└── State commitments

Index / Cache
├── Height lookup
├── Transaction lookup
├── Recent state/cache
└── Operational metadata

Snapshot Store
└── Authenticated state snapshots
~~~

The physical database engine remains an implementation decision.

## 3. Canonical Data

Canonical node data must be recoverable without depending on PostgreSQL, an explorer, or an external service.

PostgreSQL may serve as an indexer/service database but is not the canonical consensus store.

## 4. Atomicity

Block commit should be atomic from the perspective of node recovery:

1. validate block;
2. execute state transition;
3. persist block/state changes;
4. persist commit metadata;
5. expose finalized state.

Recovery must detect incomplete writes.

## 5. Database Abstraction

Storage should be accessed through interfaces so the implementation can evaluate engines such as Pebble, Badger, RocksDB-compatible solutions, or other suitable embedded stores without changing consensus semantics.

No database choice is frozen by this document.

## 6. Pruning

Future pruning modes may include:

- archive node;
- full node with bounded historical state;
- validator-oriented retention;
- snapshot-assisted pruning.

Pruning must never remove data required by the selected node mode without explicit configuration.

## 7. Snapshots

Snapshots should include:

- chain ID;
- protocol version;
- block height;
- block/state commitment;
- state data;
- metadata required for verification.

A snapshot must not be trusted solely because it was downloaded from a peer.

## 8. Backup and Recovery

Node operators should be able to:

- stop safely;
- back up canonical data;
- restore to a known height;
- verify integrity;
- resume synchronization.

## 9. Performance Goals

Storage design should optimize for:

- sequential block writes;
- predictable state reads/writes;
- bounded memory usage;
- efficient snapshot creation;
- fast startup;
- crash recovery.

Benchmark targets will be defined after the first Go prototype exists.
