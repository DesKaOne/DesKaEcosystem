# P2P Sync Planned Response Boundary v0.1

## Purpose

This boundary composes the existing sync planner and cursor-bound coordinator for one synchronization round.

It does not perform network I/O. A caller provides the response received for the request planned from the current cursor.

## Flow

1. derive the next bounded request from the cursor and remote height;
2. stop cleanly when the local cursor has reached the remote height;
3. reject an empty or oversized response;
4. require the first response block to start at the planned height;
5. apply the batch through cursor parent validation and block coordination;
6. return the advanced cursor only after successful application.

## Boundary

`ApplyPlannedResponse`:

- uses `SyncPlanner` for bounded planning;
- uses `SyncCoordinator.ApplyResponseFromCursor` for parent, ordering, block-hash, and importer validation;
- does not bypass transaction authentication requirements in the concrete node importer;
- does not define the final wire protocol or canonical serialization.

## v0.1 limitation

This is a single-round orchestration boundary. A future network session can call it repeatedly after receiving each response, while keeping transport and authentication policy outside this package.
