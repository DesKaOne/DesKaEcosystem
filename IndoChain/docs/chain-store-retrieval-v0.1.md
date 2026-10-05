# Chain Store Retrieval Semantics v0.1

> Status: Development implementation
> Scope: Devnet chain storage
> Persistence: development-only

## Purpose

This document defines the current retrieval contract between the node and a ChainStore.

## Block Lookup

ChainStore.GetBlock retrieves a stored block and its associated block hash by height.

- Existing height: return the stored block and hash.
- Missing height: return ErrBlockNotFound.
- Empty store: return ErrBlockNotFound for a normal height lookup because no block exists at that height.
- Nil store receivers return ErrEmptyStore in the current implementations.

The returned block/hash pair represents the storage record for that height. The node remains responsible for validating whether the record is canonical and internally consistent during recovery.

## Head Semantics

ChainStore.Head returns the block/hash pair at the highest saved height in the development stores.

MemoryStore and FileStore advance their stored head when a block with a height greater than or equal to the current head is successfully persisted.

Saving an older block does not move the head backward.

This is a storage retrieval rule, not a consensus finality rule. A production chain may require stronger canonical/finality metadata than the current development store exposes.

## State Retrieval

LoadState returns an isolated state snapshot.

Mutating the returned state must not mutate the store's canonical stored state.

## Commit Boundary

CommitBlockState remains the node import commit boundary. It persists the block/hash and resulting state as one storage operation from the node's perspective.

GetBlock and Head are retrieval operations only; they do not perform consensus validation or state execution.

## Node Recovery

OpenDevnet uses Head and LoadState, then recomputes the head block hash and verifies the stored state root before constructing the recovered Node.

A production backend must preserve the same observable contract while adding durable atomicity and any stronger canonical-chain metadata required by the final protocol.

## Protocol Boundary

These semantics are implementation-level behavior for v0.1. They do not freeze the final persistent database format, canonical block indexing scheme, state database, or consensus/finality metadata.

## Status

Implemented and covered by development storage tests.
