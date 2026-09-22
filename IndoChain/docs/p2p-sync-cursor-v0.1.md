# P2P Sync Cursor v0.1

## Purpose

Track the canonical tip known by a higher-level synchronization driver without reading coordinator internals.

## Boundary

`SyncCursor` contains:

- `Height`: the last successfully applied block height;
- `BlockHash`: the development hash of that block.

A cursor advances only from a `SyncProgress` result. Empty progress leaves the cursor unchanged. Non-contiguous progress is rejected, so a caller cannot silently skip heights when advancing its local sync position.

## Relationship to the sync flow

The intended flow is:

1. `SyncPlanner` creates the next bounded request from the cursor height and remote height.
2. The transport/service returns a `BlockResponse`.
3. `SyncCoordinator` validates and applies the response.
4. `SyncProgress` reports the successfully applied batch.
5. `SyncCursor.Advance` records the new canonical tip.
6. The next planning step starts after the new cursor height.

The cursor does not select peers, establish finality, or replace canonical block import validation.

## v0.1 limitation

This remains a coordination boundary. The existing generic `BlockImporter` does not yet encode the transaction authentication context required by the concrete node import path, so this cursor must not be treated as proof of end-to-end node synchronization.


## Planning helper

`SyncPlanFromCursor` connects the cursor boundary to `SyncPlanner`. It starts the next request at `cursor.Height + 1` and applies the planner's existing batch limit. When the cursor is already at the remote height, it produces no request.

This helper is only composition logic; it does not perform network I/O or import blocks.
