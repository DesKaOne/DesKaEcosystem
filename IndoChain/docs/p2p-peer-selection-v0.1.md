# P2P Peer Selection Boundary v0.1

## Purpose

Define a deterministic boundary for deciding which eligible peers may receive a propagated message.

## Selection inputs

Each candidate has:

- PeerID;
- local peer score.

The selector applies a minimum score threshold and a maximum peer count.

## Ordering

Eligible peers are ordered by descending score. Ties are resolved by ascending PeerID so the result is deterministic.

Duplicate PeerIDs are ignored.

## Boundary

This implementation does not define how scores are calculated. Future scoring inputs may include observed connectivity or protocol behavior, but those policy decisions remain outside this boundary.

The selector also does not perform transport, authentication, consensus, block validation, transaction validation, or canonical state mutation.
