# Persistent Storage Boundary v0.1

## Purpose

Define the first persistent implementation behind the existing ChainStore interface without making its file format part of the IndoChain protocol.

The implementation is intentionally development-oriented. It proves that node state can survive process restart while keeping canonical serialization and database selection unfrozen.

## FileStore

IndoChain v0.1 provides FileStore as a development persistent ChainStore.

Its storage unit is a single local file containing:

- stored blocks and their block hashes;
- the current head height;
- canonical account state required by the current node implementation.

The on-disk representation uses Go gob.

Gob is an implementation detail. It MUST NOT be treated as canonical block, transaction, or state serialization.

## Atomic Commit

CommitBlockState builds a complete candidate snapshot and writes it to a temporary file.

The persistence sequence is:

~~~text
validated block + executed state
          │
          ▼
   candidate snapshot
          │
          ▼
      temp file
          │
       fsync
          │
       close
          │
       rename
          ▼
     live store file
~~~

The previous file remains intact if encoding, sync, close, or rename fails before replacement.

This gives the development implementation a single-file atomic replacement boundary.

## Recovery

NewFileStore(path) opens the existing file when present.

An empty or nonexistent path starts with an empty store. The node's OpenDevnet lifecycle then decides whether to initialize Devnet genesis.

This preserves the distinction between:

- empty storage;
- existing storage;
- existing but inconsistent chain data.

## State Boundary

The storage adapter receives account state through explicit State snapshot methods.

Storage does not retain a mutable reference to the node's State object.

Loaded state is reconstructed into a new State instance before being returned to the node.

## Limitations

FileStore is not a production database implementation.

It does not yet provide:

- multi-process locking;
- incremental writes;
- WAL/recovery journal;
- corruption repair;
- partial block indexing;
- pruning;
- snapshots;
- archive storage;
- optimized random access;
- authenticated storage proofs.

These remain future storage work.

## Protocol Boundary

The FileStore format MUST NOT be used as:

- network serialization;
- transaction serialization;
- canonical block serialization;
- canonical state-root serialization;
- snapshot interchange format.

Changing the FileStore format must not require a protocol change.

## Financial Integration

DesKaCash does not read or write FileStore directly.

Financial application data remains in its application storage. IndoChain persistent storage is reserved for canonical chain data and state.

## Status

This document defines the first persistent storage implementation boundary for indochain-v0.1.
