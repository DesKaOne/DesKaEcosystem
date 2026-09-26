# P2P Gossip Boundary v0.1

## Purpose

Define the first boundary for propagating transactions and blocks between peers.

## Supported gossip

The initial gossip layer recognizes two payload classes:

- signed transactions;
- blocks.

Transaction gossip uses the transaction message type. Block gossip uses the block message type.

## Validation

A gossip message must match its declared message type and concrete payload type.

The gossip layer rejects unsupported message types and mismatched payloads.

## Boundary

Gossip validation does not:

- execute a transaction;
- mutate canonical state;
- commit a block;
- determine consensus or finality;
- choose peers;
- define the canonical wire encoding.

A future transaction gossip adapter can validate protocol rules before admitting a transaction to the local mempool. A future block gossip adapter can validate and execute a received block through the node's canonical import boundary.

Peer selection, duplicate suppression, propagation policy, rate limiting, and transport remain separate concerns.
