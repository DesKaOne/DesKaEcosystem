# IndoChain SDK

> Status: Draft / SDK Design Baseline

## Purpose

SDKs provide application developers with stable interfaces for interacting with IndoChain without depending on internal node implementation details.

## Initial SDK Direction

- Go: primary protocol/node SDK;
- Dart: wallet/mobile/desktop SDK;
- additional language SDKs may be generated or added later.

## SDK Responsibilities

- RPC client;
- WebSocket client;
- address utilities;
- transaction builder;
- transaction serialization;
- signing interface;
- block/transaction decoding;
- account queries;
- contract interaction where VM support exists.

## Compatibility

SDKs must expose protocol version and chain identity where relevant.

Breaking protocol changes should not silently change canonical serialization or signing behavior.

## Testing

Go and Dart SDKs must share protocol test vectors for serialization, hashing, addresses, signing, and transaction submission.

## Open Decisions

Package names, versioning policy, generated client strategy, ABI tooling, and release automation remain open.
