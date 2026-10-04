# P2P Sync Response Message v0.1

## Purpose

Define the boundary for turning a development BlockResponse into a P2P MessageTypeBlockResponse envelope.

## Design

BuildBlockResponseMessage receives a SyncResponseEncoder rather than defining block serialization itself.

Flow:
1. Caller supplies a BlockResponse.
2. Encoder serializes the response according to the active development codec.
3. Result is wrapped as MessageTypeBlockResponse.
4. Existing message validation enforces the payload limit.

## Canonical serialization boundary

This layer intentionally does not define canonical block serialization. Block encoding remains a separate concern until the protocol serialization specification is frozen.

This prevents the P2P transport layer from silently becoming the source of truth for block wire format.

## Error behavior

- Nil encoder is rejected.
- Encoder errors are propagated unchanged.
- Payloads rejected by the message envelope are rejected.
- No network I/O occurs.

## Relationship to sync flow

The request side already has BuildBlockRequestMessage, SyncMessageHandler, and SyncService.

The response side now has the corresponding message-building boundary, while the concrete block-response codec remains pluggable.

## v0.1 limitation

This is a message construction boundary only. It does not yet provide a canonical BlockResponse codec or network transport implementation.