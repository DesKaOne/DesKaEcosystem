# P2P Sync Coordinator Boundary v0.1

## Purpose

Define the coordinator between P2P block responses and the node's canonical block-import boundary.

## Responsibilities

The coordinator:

1. receives a block response associated with a starting height;
2. checks sequential block heights;
3. checks parent linkage between consecutive response blocks;
4. sends each accepted block to the `BlockImporter` boundary.

## Canonical import

The coordinator never writes storage or canonical state directly. `BlockImporter` is the only write boundary exposed to synchronization.

In the node implementation this boundary is provided by `Node.ImportBlock`, allowing the existing block execution, state-root, hash, and atomic commit checks to remain authoritative.

## Boundary

This v0.1 coordinator does not choose forks, determine finality, perform peer selection, or implement retry/backoff. It also does not replace the node's canonical block validation.

A production sync manager can build retries and peer failover around this coordinator without bypassing the canonical import path.
