# P2P Sync Message Handler v0.1

## Purpose

Connect the existing P2P message envelope with the development synchronization service without binding synchronization logic to a concrete network transport.

## Flow

`MessageTypeBlockRequest` → payload validation → `BlockRequest` decode → `SyncService.HandleBlockRequest` → `BlockResponse`.

## Boundary

The handler accepts only block-request messages. It does not open sockets, select peers, write storage, execute blocks, or decide finality.

The response remains an in-memory `BlockResponse` at this stage because canonical block serialization is still unfrozen. A wire response codec can be added after the canonical serialization boundary is finalized.
