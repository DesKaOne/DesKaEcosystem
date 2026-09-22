# P2P Message Envelope v0.1

## Purpose

Define the first transport-independent message boundary for IndoChain P2P traffic.

## Envelope

Each message contains:

- Type — a uint16 message discriminator;
- PayloadLength — a uint32 payload length;
- Payload — opaque message bytes.

The development wire encoding is: uint16 type, uint32 payload_length, followed by payload bytes.

All integer fields use big-endian encoding.

## Message types

The initial development message types are:

- Handshake
- Transaction
- Block
- BlockRequest
- BlockResponse

These identifiers are implementation-level placeholders until the canonical P2P wire protocol is frozen.

## Validation

The envelope layer rejects unknown message types, empty payloads, payloads above the configured maximum, truncated frames, and frames with trailing bytes.

The maximum payload is supplied by the caller so transport and deployment profiles can impose different limits.

## Boundary

The envelope does not interpret transaction, block, or synchronization payloads. It also does not provide authentication, encryption, transport reliability, peer scoring, or consensus semantics.

Those responsibilities remain in their respective layers.
