# IndoChain Cryptography Specification v0.1

> Status: Draft Specification

## 1. Purpose

Define the cryptographic interfaces used by transaction authorization, block identity, addresses, and consensus authentication.

## 2. Required Primitives

The final suite must specify:

- cryptographic hash;
- digital signature;
- public-key encoding;
- signature encoding;
- address derivation;
- domain separation.

## 3. Hashing

All consensus hashes must use one explicitly specified algorithm and encoding convention.

Hash input must be canonical bytes.

## 4. Signatures

The signature scheme must define:

- key generation;
- public-key format;
- signature format;
- deterministic/randomized signing behavior;
- verification rules;
- malformed signature handling.

The final algorithm remains TBD.

## 5. Domain Separation

Separate domains should be defined for at least:

- transaction signing;
- block/proposer authorization;
- consensus voting;
- network/session authentication where applicable.

A signature valid in one domain must not be accepted as another message type.

## 6. Address Derivation

The address specification must define how a public key or account identifier becomes the canonical human-readable address.

The current roadmap direction is an IndoChain address beginning with `iND1...`.

## 7. Randomness

Key generation and any protocol operation requiring randomness must use cryptographically secure randomness appropriate to the platform.

## 8. Key Separation

Wallet signing keys, validator keys, and network/session keys should be separable where protocol design requires different trust boundaries.

## 9. Test Vectors

Provide vectors for key generation/import, public-key encoding, address derivation, signing, verification, invalid signatures, wrong domains, and wrong-chain signing.

## 10. Open Items

- final hash algorithm;
- final signature algorithm;
- key sizes/curves;
- address checksum/encoding;
- validator key model;
- consensus aggregate-signature strategy if required.
