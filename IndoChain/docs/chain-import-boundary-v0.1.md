# Chain Import Boundary v0.1

> Status: Development implementation
> Scope: Devnet chain execution
> Persistence: development-only

## Purpose

The node import boundary connects block validation, deterministic execution, block hashing, and storage.

    Incoming Block
         |
         v
    Header + Parent Validation
         |
         v
    TransactionsRoot
         |
         v
    Execute Transactions
         |
         v
    StateRoot
         |
         v
    Block Hash
         |
         v
    Commit Block + State
         |
         v
    Update Node Head

## Rules

A block must have the next height, matching parent hash, chain ID, and protocol version. Execution happens on a state snapshot. Transaction failure or StateRoot mismatch leaves canonical node state unchanged.

## Commit Boundary

ChainStore.CommitBlockState is the development commit boundary. MemoryStore snapshots state and stores the block/hash before advancing the node head.

ImportBlock updates the in-memory node head and state only after CommitBlockState succeeds. If the storage commit returns an error, the node head, head hash, and canonical state remain unchanged. The failed block is therefore not treated as imported.

A production backend must provide durable atomic commit semantics across block metadata and state. The node relies on that backend contract before advancing its in-memory canonical view.

## Consensus Boundary

Consensus should call Node.ImportBlock rather than mutate canonical state directly. Consensus determines proposal/finality; chain execution determines whether block contents are valid and what state transition they produce.

## Configuration boundary

Node block import derives block execution rules from its ChainConfig rather than accepting a caller-supplied chain ID or protocol version. The caller supplies only the public key required by the current development transaction verification path. This reduces accidental chain-rule mismatches while the transaction public-key field remains unfrozen.
