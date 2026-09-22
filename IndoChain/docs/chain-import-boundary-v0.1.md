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

A production backend must provide durable atomic commit semantics across block metadata and state.

## Consensus Boundary

Consensus should call Node.ImportBlock rather than mutate canonical state directly. Consensus determines proposal/finality; chain execution determines whether block contents are valid and what state transition they produce.
