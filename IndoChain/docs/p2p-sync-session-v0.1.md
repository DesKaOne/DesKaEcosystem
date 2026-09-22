# P2P Sync Session v0.1

## Purpose

`SyncSession` is the pure orchestration state for repeated bounded synchronization rounds.

It keeps:

- the `SyncPlanner` configuration;
- the `SyncCoordinator` used to apply blocks;
- the current `SyncCursor`.

## Flow

1. `NextRequest(remoteHeight)` plans the next bounded range from the current cursor.
2. The caller obtains a response from the peer or transport layer.
3. `ApplyResponse(remoteHeight, response)` first checks whether the session still needs synchronization, then validates and applies that round.
4. The cursor advances only after the coordinator successfully imports the response.
5. The caller repeats until `NextRequest` reports that the local cursor is caught up.

## Failure behavior

Rejected or failed responses do not update the session cursor. If the session is already caught up, applying a response is a no-op.

A nil session or missing coordinator is rejected before state mutation.

## Boundary

`SyncSession` does not:

- perform network I/O;
- discover or select peers;
- define canonical serialization;
- bypass the transaction authentication context required by node block import.

It is an orchestration boundary above the existing planner, cursor validation, and coordinator layers.

## v0.1 status

This session model supports repeated bounded sync rounds. Network transport and canonical protocol message serialization remain separate concerns.
