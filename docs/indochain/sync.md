# IndoChain Synchronization

> Status: Draft / Sync Design Baseline

## 1. Purpose

Synchronization allows a new or recovering node to obtain canonical chain data and state from peers.

## 2. Sync Modes

Candidate modes:

- full block sync;
- header-first sync;
- state/snapshot sync;
- fast sync;
- archive synchronization.

The selected mode must always verify canonical commitments.

## 3. Bootstrap Flow

~~~text
Start Node
   ↓
Discover Peers
   ↓
Verify Chain ID / Genesis
   ↓
Determine Local Height
   ↓
Exchange Peer Heights
   ↓
Download Headers / Blocks / Snapshot
   ↓
Verify Commitments
   ↓
Execute or Restore State
   ↓
Catch Up
   ↓
Enter Normal Operation
~~~

## 4. Snapshot Sync

Snapshots may accelerate bootstrap.

A snapshot must contain enough metadata to verify:

- chain identity;
- protocol version;
- snapshot height;
- block commitment;
- state commitment.

A node must not trust a snapshot merely because a peer supplied it.

## 5. Resumability

Downloads should support:

- chunking;
- integrity verification;
- retry;
- resume after interruption;
- bounded concurrency.

## 6. Peer Selection

Peers should be selected using chain compatibility, availability, reputation, and synchronization quality.

## 7. Safety

Sync must reject:

- wrong genesis;
- wrong chain ID;
- invalid headers;
- invalid parent links;
- invalid state commitments;
- malformed blocks;
- unsupported protocol versions.

## 8. Test Requirements

Test:

- empty/new node bootstrap;
- slow peer;
- disconnected peer;
- corrupt snapshot;
- interrupted download;
- conflicting peers;
- protocol-version mismatch;
- recovery after crash.
