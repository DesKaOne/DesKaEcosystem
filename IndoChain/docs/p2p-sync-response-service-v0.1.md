# P2P Sync Response Service v0.1

## Purpose

Provide a small service boundary around `SyncResponseHandler` for future network/session integration.

## Responsibilities

- Hold the configured response handler.
- Forward the remote height and response message.
- Preserve handler errors and the handler's session-advance semantics.

## Boundary

This service does not decode payloads directly, mutate the chain directly, or perform network I/O.

The underlying handler remains responsible for envelope validation, response decoding, and `SyncSession` application.

## Error behavior

A missing service handler is rejected before any response processing.

## v0.1 limitation

This is an orchestration boundary only. Transport wiring and canonical response serialization remain separate concerns.