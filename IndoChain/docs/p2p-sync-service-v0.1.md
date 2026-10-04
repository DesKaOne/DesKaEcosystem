# P2P Sync Service Boundary v0.1

## Purpose

Define the request-facing P2P synchronization service above the read-only range adapter.

## Responsibilities

`SyncService.HandleBlockRequest`:

1. validates the requested limit against the configured maximum;
2. reads the requested range through `SyncReader` and `ReadBlockRange`;
3. verifies response height continuity;
4. returns a bounded `BlockResponse` without mutating canonical state.

## Boundary

The service does not write blocks, execute transactions, select forks, or decide finality. Block import remains the responsibility of the canonical node import boundary used by `SyncCoordinator`.

## Error handling

Invalid request limits are rejected before any read. Read failures are wrapped as `ErrSyncReadFailure` so a transport layer can map them without depending on storage or node internals.
