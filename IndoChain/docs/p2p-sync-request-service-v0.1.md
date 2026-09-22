# P2P Sync Request Service v0.1

## Purpose

Provide a small orchestration boundary around SyncMessageHandler for future network integration.

## Responsibilities

- Hold the configured request handler.
- Forward block-request messages.
- Preserve handler validation and service errors.

## Boundary

The service does not perform network I/O or define wire serialization.

The underlying SyncMessageHandler remains responsible for message validation, request decoding, and bounded block-range serving.

## v0.1 limitation

This is an orchestration boundary only. Transport and canonical P2P serialization remain separate concerns.
