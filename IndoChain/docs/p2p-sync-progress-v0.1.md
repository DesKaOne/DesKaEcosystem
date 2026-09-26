# P2P Sync Progress v0.1

## Purpose

Expose the result of a successfully applied synchronization batch so a higher-level sync manager can plan the next request without reading internal coordinator state.

## Result

`SyncProgress` contains:

- `Applied`: number of blocks imported from the response;
- `LastHeight`: height of the final imported block;
- `LastBlockHash`: development hash of the final imported block.

An empty response returns an empty progress value. A failed response returns no progress, so callers must not advance their local sync cursor on an unsuccessful batch.

## Boundary

Progress is informational. It does not choose peers, determine finality, or mutate state independently of the canonical `BlockImporter` boundary.
