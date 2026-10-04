# P2P Sync Coordinator Boundary v0.1

## Purpose

Define the coordinator between P2P block responses and a canonical block-import adapter.

## Responsibilities

The coordinator:

1. receives a block response associated with a starting height;
2. checks sequential block heights;
3. checks the parent hash of the first block against the caller-provided canonical parent;
4. checks parent linkage between consecutive response blocks;
5. sends structurally accepted blocks to the `BlockImporter` boundary.

## Canonical import

The coordinator never writes storage or canonical state directly. `BlockImporter` is the only write boundary exposed to synchronization.

The current generic interface is intentionally separate from `Node.ImportBlock`. The node's concrete import method also requires transaction-authentication context because the current development transaction schema does not carry a canonical public key. An adapter must not bypass that requirement.

## Boundary

This v0.1 coordinator does not choose forks, determine finality, perform peer selection, or implement retry/backoff. It also does not replace the node's canonical block validation.

A production sync manager can build retries and peer failover around this coordinator after the transaction authentication and canonical wire serialization boundaries are finalized.
