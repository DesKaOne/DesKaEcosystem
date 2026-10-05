# P2P Handshake Boundary v0.1

## Purpose

Define the minimum identity and network-compatibility checks performed before a
peer is admitted to the local P2P layer.

## Handshake fields

A handshake currently carries:

- PeerID;
- ProtocolVersion;
- ChainID;
- NetworkProfile.

## Validation

The local node accepts a handshake only when:

1. PeerID is present;
2. NetworkProfile is present;
3. protocol version matches the local rule;
4. chain ID matches the local rule;
5. network profile matches the local rule.

A malformed identity returns an invalid-handshake error. A compatibility mismatch
returns a protocol-mismatch error.

## Boundary

Handshake validation does not:

- establish a transport;
- authenticate a cryptographic identity;
- exchange chain data;
- execute transactions;
- participate in consensus;
- mutate canonical state.

Cryptographic peer identity, capability negotiation, transport framing, and
wire-level encoding remain future work.
