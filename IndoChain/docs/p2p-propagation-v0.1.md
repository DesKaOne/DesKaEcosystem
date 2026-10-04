# P2P Propagation Policy v0.1

## Purpose

Define the first safeguards against unbounded gossip amplification.

## Duplicate suppression

Each propagator maintains a local set of message identifiers that have already been observed.

A repeated identifier is rejected as a duplicate and should not be forwarded again by the same propagator.

The identifier format is intentionally outside this boundary; transaction and block layers will provide canonical identifiers when those formats are frozen.

## Fan-out

Propagation uses a configurable maximum peer fan-out. The number of selected peers must not exceed `MaxPeers` and is also bounded by the number of currently eligible peers.

Peer selection itself is not implemented here.

## Boundary

This layer does not define:

- peer scoring;
- random or deterministic peer selection;
- transport reliability;
- rate limiting;
- message authentication;
- transaction or block validity;
- consensus semantics.

Those concerns remain separate so propagation policy can evolve without changing canonical state or consensus rules.
