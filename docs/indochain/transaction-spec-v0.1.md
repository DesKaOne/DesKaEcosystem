# IndoChain Transaction Specification v0.1

> Status: Draft Specification
> Scope: Devnet protocol baseline

## 1. Transaction Goals

Transactions must be deterministic, independently verifiable, replay-protected, and serializable identically by Go and Dart implementations.

## 2. Logical Structure

~~~text
Transaction
├── Version
├── ChainID
├── Nonce
├── Sender
├── Recipient / ExecutionTarget
├── Value / Asset
├── GasLimit
├── FeeFields
├── Data
└── Signature
~~~

This is the logical model. Exact binary field widths and canonical encoding remain subject to the serialization specification.

## 3. Nonce

Nonce provides per-account transaction ordering and replay protection within the account model.

The node must reject a transaction whose nonce violates the active account nonce rules.

Exact replacement and future-nonce policy is defined by the mempool specification.

## 4. Chain Binding

ChainID and the protocol-defined signing domain must be included in the transaction authorization model so that a valid signature on one network cannot be silently reused on another.

## 5. Sender

The sender is derived or verified from the transaction authorization data according to the final crypto specification.

The node must reject sender/signature inconsistencies.

## 6. Recipient / Execution Target

A transaction may target:

- another account;
- a contract/VM execution target;
- a protocol-defined system target where supported.

The exact target encoding is TBD.

## 7. Value and Asset

The transaction must distinguish native value transfer from contract/application data where required.

The exact representation of dIDR and future token transfers is defined by the VM/token layer rather than assumed by the base transaction schema.

## 8. Gas and Fees

Transactions carry the gas/fee fields required by the active fee model.

Current candidates include GasLimit, GasPrice, BaseFee, and PriorityFee.

Exact fee fields and validation formulas remain TBD.

## 9. Data

Data is an opaque protocol-defined byte sequence until interpreted by the execution layer.

Wallets may provide higher-level builders, but canonical signing must operate on protocol-defined bytes.

## 10. Signature

Signature encoding, key type, and signing domain are defined by the crypto specification.

A transaction is not valid for propagation/execution until signature validation passes all required protocol checks.

## 11. Validation Order

A node should conceptually validate:

1. network/chain identity;
2. transaction version;
3. canonical encoding;
4. signature/authentication;
5. sender consistency;
6. nonce rules;
7. gas/fee constraints;
8. balance/resource constraints;
9. execution-specific validity.

Exact ordering may be optimized only if observable protocol behavior remains deterministic.

## 12. Hashing

Transaction hashes must be calculated from canonical transaction bytes and a protocol-defined hash function.

Hashing must not depend on JSON formatting or UI representations.

## 13. Mempool Semantics

The mempool may hold valid future-nonce transactions subject to configured policy. It must not change canonical state.

Replacement, eviction, expiry, and per-account limits remain configuration/specification items.

## 14. Test Vectors

Before implementation freeze, provide vectors for:

- canonical unsigned transaction;
- canonical signing bytes;
- signed transaction;
- transaction hash;
- invalid signature;
- wrong ChainID;
- invalid nonce;
- insufficient balance;
- invalid gas/fee.

## 15. Open Items

- exact binary schema;
- exact integer widths;
- exact address encoding;
- final crypto suite;
- exact fee fields;
- contract target encoding;
- signature encoding.
