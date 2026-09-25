# IndoChain P2P Specification v0.1

> Status: Draft Specification
> Scope: Devnet protocol baseline

## 1. Purpose

The P2P layer transports blocks, transactions, consensus messages, and synchronization requests between nodes.

## 2. Layering

~~~text
Application / Consensus
        ↓
P2P Protocol
        ↓
Transport
        ↓
Network
~~~

Consensus logic must not depend on a specific transport implementation.

## 3. Transport

TCP and QUIC remain candidates. The selected transport must provide reliable framing and connection management appropriate to the node protocol.

## 4. Node Identity

Nodes require a protocol-defined identity for peer sessions. The final identity/key model is TBD.

## 5. Handshake

A peer handshake should establish:

- protocol version;
- network/chain identity;
- node identity;
- supported capabilities;
- session parameters.

A node must reject peers with incompatible chain/network identity where required.

## 6. Message Classes

Candidate message classes:

- transaction gossip;
- block/header gossip;
- consensus messages;
- block synchronization;
- state/snapshot synchronization;
- peer discovery/control.

## 7. Message Framing

Messages require deterministic framing, type identification, length limits, and malformed-input rejection.

The exact wire encoding remains TBD.

## 8. Gossip

Transactions and blocks may propagate using bounded gossip. Consensus messages require stricter validation and rate controls.

Duplicate messages should be suppressed using message identifiers or content hashes.

## 9. Peer Scoring

Nodes may maintain peer reputation based on behavior such as invalid messages, repeated failures, excessive requests, and successful service.

Exact scoring is TBD.

## 10. DoS Protection

Required controls include:

- connection limits;
- message-size limits;
- request rate limits;
- per-peer quotas;
- malformed-message penalties;
- backpressure.

## 11. Synchronization

P2P sync must verify block linkage, chain identity, consensus evidence, and state commitments according to the active protocol profile.

## 12. Security

The protocol must consider eclipse attacks, Sybil behavior, resource exhaustion, malicious peers, replayed messages, and protocol-version confusion.

## 13. Test Requirements

Test malformed frames, invalid identities, wrong networks, duplicate gossip, peer churn, rate limiting, partial sync, malicious peers, and reconnect/recovery.

## 14. Open Items

- transport;
- node identity scheme;
- wire encoding;
- message IDs;
- discovery mechanism;
- peer scoring;
- sync protocol details.
