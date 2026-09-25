# IndoChain Cryptography

> Status: Draft / Crypto Design Baseline

## 1. Goals

Cryptographic design must provide:

- collision-resistant hashing;
- authenticated transactions;
- validator authentication;
- replay protection;
- domain separation;
- deterministic verification;
- implementation portability across Go and Flutter/Dart clients.

## 2. Hashing

The roadmap mentions SHA-256 and Keccak as candidate hash primitives.

Hash usage must be assigned explicitly by protocol context, for example:

- block identifiers;
- transaction identifiers;
- Merkle/tree nodes;
- state commitments;
- signing-domain hashes.

The final primitive and exact encoding for each context must be frozen before mainnet.

## 3. Digital Signatures

The roadmap mentions secp256k1 and Ed25519.

The protocol must define:

- supported key type(s);
- public-key encoding;
- signature encoding;
- canonical signing bytes;
- domain separation;
- signature malleability rules;
- key rotation/recovery rules where supported.

Wallet implementations must produce exactly the bytes expected by the protocol.

## 4. Domain Separation

Signing different objects must not accidentally share an ambiguous signing domain.

Conceptual pattern:

~~~text
Domain || ChainID || ProtocolVersion || MessageType || CanonicalMessage
~~~

The concrete encoding remains to be specified.

## 5. Address Derivation

The current wallet design references a human-readable IndoChain address beginning with iND1... and a Base58-oriented address concept.

Before implementation, specify:

- version byte;
- network identifier;
- payload;
- checksum;
- canonical encoding;
- key-type marker if multiple key types are supported.

If EVM compatibility is enabled, 0x... addresses would be an additional representation with explicit derivation rules.

## 6. Key Management

Node and wallet key material must be separated by purpose.

Potential categories:

- account/transaction signing keys;
- validator consensus keys;
- node identity keys;
- optional session/network keys.

Private keys must never be serialized into logs, metrics, RPC responses, or crash reports.

## 7. Wallet Compatibility

The Flutter/Dart wallet needs protocol libraries that implement:

- key generation;
- mnemonic handling if adopted;
- address derivation;
- transaction encoding;
- signing;
- signature verification;
- chain/domain separation.

Go remains the reference implementation target for node-side protocol behavior.

## 8. Security Requirements

- No unauthenticated cryptographic shortcuts.
- Constant-time primitives where required.
- Strict canonical encoding.
- Reject malformed signatures.
- Reject unexpected key types.
- Fuzz parsers and signature/encoding boundaries.
- Test vectors must be shared between Go and Dart.

## 9. Open Decisions

- SHA-256 vs Keccak allocation by protocol component.
- secp256k1 vs Ed25519 as default account/validator key type.
- Multi-key support.
- Address checksum format.
- Hardware-wallet signing protocol.
- Threshold/multisignature encoding.

No choice should be treated as final until test vectors and interoperability tests exist.
