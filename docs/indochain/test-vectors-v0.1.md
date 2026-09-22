# IndoChain Protocol Test Vectors v0.1

> Status: Draft Test-Vector Specification
> Purpose: interoperability baseline for Go and Dart implementations

## 1. Purpose

Test vectors are deterministic fixtures used to prove that independent IndoChain implementations produce identical protocol results.

The same logical input must produce the same canonical bytes, hashes, signatures, and state commitments under the same protocol configuration.

## 2. Fixture Format

The final machine-readable fixture format is TBD. A fixture should conceptually contain:

~~~text
Vector
├── protocol_version
├── chain_id
├── input
├── expected_canonical_bytes
├── expected_hash
└── expected_result
~~~

Human-readable Markdown may document vectors, while machine-readable fixtures should be stored separately for automated tests.

## 3. Crypto Vectors

Required categories:

- hash input/output;
- key/public-key representation;
- address derivation;
- signature generation;
- signature verification;
- invalid signature;
- wrong signing domain.

The final algorithm values remain dependent on the crypto specification freeze.

## 4. Transaction Vectors

Each vector should include:

- logical transaction;
- unsigned canonical bytes;
- signing bytes;
- signature;
- signed canonical bytes;
- transaction hash;
- expected validation result.

Include valid and invalid cases for ChainID, nonce, signature, gas, fee, balance, and malformed encoding.

## 5. Block Vectors

Required fixtures:

- genesis block;
- first child block;
- block with one transaction;
- block with multiple transactions;
- invalid previous hash;
- invalid transaction root;
- invalid state root;
- invalid proposer authorization.

## 6. State Vectors

Required state transitions:

- initial account creation;
- native transfer;
- nonce increment;
- fee deduction;
- insufficient balance;
- invalid authorization;
- execution failure where VM is enabled;
- resulting state commitment.

## 7. Consensus Vectors

Once the consensus algorithm is frozen, vectors must cover:

- proposal;
- vote;
- duplicate vote;
- conflicting vote;
- quorum/finality evidence;
- invalid validator;
- wrong height/round;
- wrong chain;
- invalid signature.

## 8. Genesis Vectors

At minimum provide deterministic Devnet and Testnet genesis fixtures.

Each fixture must verify:

- chain identity;
- protocol version;
- initial validator set;
- initial accounts/allocations;
- initial state commitment;
- genesis identifier.

Mainnet genesis must be generated independently and must never reuse Devnet/Testnet private credentials.

## 9. Cross-Language Requirement

Go and Dart implementations must consume the same canonical fixture files.

A compatibility test should compare:

~~~text
Go result == expected result == Dart result
~~~

Any mismatch is a protocol compatibility failure, not a UI-only issue.

## 10. Failure Vectors

Negative tests are mandatory. They should cover malformed encodings, overflow, invalid signatures, wrong chain, invalid nonce, invalid roots, invalid consensus evidence, oversized messages, and unsupported protocol versions.

## 11. Freeze Gate

The test-vector suite becomes implementation-critical when the corresponding protocol component is frozen.

No protocol component should be declared interoperable until its positive and negative vectors pass in the reference Go implementation and are reproducible by Dart where applicable.

## 12. Open Items

- final fixture encoding;
- crypto algorithms;
- canonical serialization;
- final transaction schema;
- final consensus algorithm;
- exact genesis parameters.
