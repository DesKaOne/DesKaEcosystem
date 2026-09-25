# IndoChain P2P Networking

> Status: Draft / P2P Design Baseline

## 1. Purpose

P2P networking transports transactions, blocks, consensus messages, peer metadata, and synchronization data between nodes.

## 2. Transport

The roadmap allows TCP/QUIC as candidate transports.

Transport selection must remain below the protocol message layer.

## 3. Peer Lifecycle

~~~text
Discover
  ↓
Connect
  ↓
Handshake
  ↓
Authenticate / Identify
  ↓
Exchange Capabilities
  ↓
Maintain
  ↓
Score / Rate Limit
  ↓
Disconnect / Ban if necessary
~~~

## 4. Peer Identity

The node should have a stable network identity distinct from account/validator signing keys where appropriate.

Handshake should communicate:

- protocol version;
- chain ID;
- node identity;
- supported features;
- software version;
- synchronization status.

## 5. Gossip

Candidate gossip classes:

- transactions;
- block proposals;
- finalized blocks;
- consensus votes;
- evidence.

Gossip must prevent uncontrolled amplification and duplicate processing.

## 6. Peer Discovery

Candidate mechanisms:

- static bootstrap peers;
- DNS/bootstrap endpoints;
- peer exchange;
- DHT/discovery layer.

Discovery mechanisms must never bypass chain-ID validation.

## 7. Peer Scoring

Peer reputation may consider:

- successful message delivery;
- invalid messages;
- repeated duplicates;
- timeout behavior;
- protocol violations;
- bandwidth abuse.

Scoring must not become a source of consensus divergence.

## 8. Rate Limits

Nodes should enforce limits for:

- connections;
- messages per peer;
- transaction propagation;
- block propagation;
- synchronization requests;
- memory consumption.

## 9. Sync

P2P must support:

- header/block synchronization;
- state/snapshot synchronization;
- peer capability negotiation;
- resumable downloads.

Detailed synchronization belongs in sync.md.

## 10. Security

The network layer must defend against:

- malformed messages;
- oversized payloads;
- connection exhaustion;
- gossip storms;
- duplicate spam;
- identity abuse;
- invalid chain/network messages.

## 11. Open Decisions

- TCP vs QUIC;
- message serialization;
- discovery protocol;
- encryption/authentication model;
- peer scoring formula;
- maximum peer count;
- bootstrap strategy.
