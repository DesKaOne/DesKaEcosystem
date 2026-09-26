# P2P Implementation Boundary v0.1

## Purpose

Define the first implementation boundary for peer management without coupling peer
transport to consensus or canonical state.

## Peer model

A peer currently has:

- stable local PeerID;
- network Address;
- negotiated ProtocolVersion.

The PeerSet provides concurrent-safe add, lookup, removal, snapshot, and count
operations.

## Boundary

Peer management is infrastructure. It does not:

- execute transactions;
- mutate canonical state;
- decide block validity;
- finalize consensus;
- persist chain state.

Future P2P layers can add transport, handshake, capability negotiation, discovery,
gossip, peer scoring, rate limits, and block/state synchronization.

The exact transport and wire protocol remain outside this v0.1 implementation
boundary.
