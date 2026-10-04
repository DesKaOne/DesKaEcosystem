# P2P Sync Response Handler v0.1

## Purpose

Connect a validated `MessageTypeBlockResponse` envelope to the existing `SyncSession` without defining canonical block serialization or network transport.

## Flow

1. Require a sync session and response decoder.
2. Require `MessageTypeBlockResponse`.
3. Validate the existing P2P envelope and payload limit.
4. Decode the payload through `SyncResponseDecoder`.
5. Pass the decoded `BlockResponse` to `SyncSession.ApplyResponse`.
6. Advance the session cursor only when the sync application succeeds.

## Serialization boundary

`SyncResponseDecoder` is deliberately pluggable. This handler does not define a canonical `BlockResponse` wire format.

## Error behavior

- Nil session uses the existing sync-session error.
- Nil decoder is rejected.
- Wrong message type is rejected.
- Invalid envelope is rejected before decoding.
- Decoder errors are propagated.
- No network I/O occurs.

## v0.1 limitation

The handler is an orchestration boundary. A concrete canonical response codec and transport adapter remain separate work.