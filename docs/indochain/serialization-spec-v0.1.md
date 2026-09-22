# IndoChain Serialization Specification v0.1

> Status: Draft Specification

## 1. Purpose

Consensus-critical objects require deterministic serialization. Two conforming implementations must produce identical bytes for identical logical values.

## 2. Scope

Serialization applies to:

- transactions;
- block headers;
- blocks;
- consensus messages;
- P2P messages where protocol compatibility requires canonical encoding;
- genesis configuration/identity inputs.

## 3. Requirements

Canonical serialization must be:

- deterministic;
- unambiguous;
- length-safe;
- version-aware;
- independent of map ordering;
- independent of locale;
- independently implementable in Go and Dart.

## 4. Candidate Encoding

The exact encoding format is **TBD**. Candidates may include a custom deterministic binary encoding, CBOR-like encoding, protobuf-style encoding, or another explicitly specified format.

No implementation may treat ordinary JSON serialization as canonical consensus bytes unless this specification is explicitly changed.

## 5. Integer Encoding

The final specification must define signedness, width/range, byte order, and overflow behavior for every integer field.

## 6. Bytes and Strings

The final specification must define length prefixes, UTF-8 requirements where strings exist, maximum lengths, and rejection behavior for malformed values.

## 7. Optional Fields

Optional values must have one unambiguous representation. Missing, null, empty, and zero must not become interchangeable unless explicitly specified.

## 8. Hashing and Signing

Hashing/signing must consume canonical bytes only.

The serialization layer must expose separate functions for unsigned signing bytes and fully serialized signed transactions where needed.

## 9. Versioning

A protocol version must identify changes that can alter canonical bytes.

Backward-compatible decoding must never create multiple canonical encodings for the same logical object.

## 10. Test Vectors

For every consensus-critical type, maintain:

- logical input;
- canonical hex bytes;
- hash;
- expected decode result.

## 11. Open Items

The encoding format, field ordering, widths, byte order, maximum sizes, and version envelope are not yet frozen.
