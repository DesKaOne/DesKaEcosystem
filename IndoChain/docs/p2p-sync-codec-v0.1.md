# P2P Sync Request Codec v0.1

## Purpose

Define the development-only byte representation used to carry a `BlockRequest` through the existing P2P message envelope.

## Encoding

The request is exactly 16 bytes:

- bytes 0-7: `FromHeight`, unsigned 64-bit big-endian;
- bytes 8-15: `Limit`, unsigned 64-bit big-endian.

The codec validates the request against the caller-provided maximum limit before encoding or after decoding.

## Protocol status

This is an implementation boundary for v0.1 development. It is **not** the canonical protocol serialization. Canonical serialization remains unfrozen until the serialization specification is finalized.

## Separation of concerns

The codec does not read storage, execute blocks, mutate state, or perform peer selection. It only translates a validated synchronization request to and from bytes.
