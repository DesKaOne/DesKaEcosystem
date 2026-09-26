# Chain Query Boundary v0.1

## Purpose

Define the read-only chain access boundary between the IndoChain node and future
RPC, explorer, indexer, wallet, and financial-service adapters.

The boundary is intentionally small. It exposes canonical chain data without
allowing consumers to mutate node state or bypass block execution.

## Interface

The development node implements node.ChainReader:

- HeadBlock() returns the canonical head block and stored hash.
- BlockByHeight(height) returns a block and stored hash for a canonical height.
- TransactionByHash(hash) returns the canonical transaction plus containing block metadata.
- StateSnapshot() returns an isolated state snapshot.

## Read-only guarantees

Query methods must not:

- execute transactions;
- advance the canonical head;
- mutate canonical state;
- rewrite storage;
- bypass node recovery validation.

StateSnapshot() must return an isolated copy. A caller may inspect or modify
the returned snapshot without changing the node's canonical in-memory state.

## Missing data

A height greater than the current head is treated as not found and returns
storage.ErrBlockNotFound.

The underlying storage remains the source of truth for persisted block
retrieval. The query boundary does not invent historical blocks.

Transaction lookup returns ErrTransactionNotFound when no canonical transaction
matches the requested hash. The initial implementation scans canonical blocks
linearly; a future index may optimize lookup without changing the interface.

## Adapter boundary

Future JSON-RPC, WebSocket, explorer, indexer, and DesKaCash integration
adapters should depend on the read-only query boundary rather than reaching
into storage internals or importing implementation details from unrelated
services.

This keeps application/service concerns outside canonical node state.

## Scope

This is a v0.1 implementation boundary, not a final RPC specification.
Method names, response schemas, pagination, event lookup, and proof-oriented
queries remain subject to later protocol/API design.
