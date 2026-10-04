# RPC Read API v0.1

## Purpose

Define the first transport adapter over the node's read-only ChainReader boundary.

The implementation uses JSON-RPC 2.0 over HTTP. It is a read-only adapter:
RPC requests do not execute transactions, mutate canonical state, or write storage.

## Methods

### chain_head

Parameters: none.

Returns the current canonical head block and its stored hash.

### block_by_height

Parameters:

- one unsigned integer height.

Returns the canonical block at that height and its stored hash.

### transaction_by_hash

Parameters:

- one 32-byte transaction hash encoded as hexadecimal.

Returns the transaction plus its canonical block height, block hash, and transaction index.

## Error mapping

- -32600 invalid JSON-RPC request.
- -32601 unknown method.
- -32602 invalid method parameters.
- -32004 transaction not found.
- -32000 node/storage/internal read failure.

## Boundary

The RPC adapter depends only on node.ChainReader. It must not import storage
implementations or mutate node state directly.

This is a development API boundary. Authentication, rate limiting, pagination,
WebSocket subscriptions, batching, transaction submission, and final public
wire schemas remain outside this v0.1 read-only scope.
