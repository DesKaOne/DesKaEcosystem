# P2P Sync Range Read v0.1

## Purpose

Provide a read-only adapter that turns a bounded BlockRequest into a contiguous BlockResponse.

## Behavior

ReadBlockRange:

1. validates the requested range;
2. reads blocks one height at a time through SyncReader;
3. verifies each returned block has the requested height;
4. stops cleanly when the remote reader reaches the end of its available range after at least one block;
5. returns an error when the first requested block cannot be read;
6. rejects a block whose header height does not match the requested height.

## Boundary

The range reader does not write storage, mutate canonical state, execute transactions, or select forks.

It deliberately uses the existing single-height SyncReader boundary so storage and node internals remain outside the P2P package.

## Relationship to the coordinator

ReadBlockRange prepares the block response. SyncCoordinator validates the response's ordering and parent linkage before forwarding blocks to the canonical import boundary.

BlockImporter remains responsible for canonical execution and commit.
