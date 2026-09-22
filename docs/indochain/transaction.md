# IndoChain Transactions

> Status: Draft / Transaction Design Baseline

## 1. Purpose

Transactions are the authenticated requests that ask the network to change canonical state.

## 2. Conceptual Structure

~~~text
Transaction
├── Version
├── ChainID / Domain
├── Nonce
├── Sender
├── Action / Payload
├── GasLimit
├── Fee fields
├── Signature
└── Optional metadata
~~~

The exact binary layout remains to be frozen.

## 3. Transaction Lifecycle

~~~text
Wallet
  ↓
Encode
  ↓
Sign
  ↓
Local validation
  ↓
RPC submission
  ↓
Node validation
  ↓
Mempool
  ↓
Gossip
  ↓
Block proposal
  ↓
Execution
  ↓
Commit
~~~

## 4. Validation

A node should reject transactions that fail:

- decoding;
- version compatibility;
- chain/domain validation;
- signature verification;
- nonce/replay checks;
- sender authorization;
- fee/gas checks;
- intrinsic execution requirements.

## 5. Nonce and Replay Protection

The transaction model must prevent:

- duplicate execution;
- cross-chain replay;
- stale transaction replacement ambiguity;
- accidental re-execution after reorganization.

The final nonce semantics must be compatible with the selected account model and mempool replacement policy.

## 6. Fees

The design references:

- GasLimit;
- GasUsed;
- GasPrice;
- BaseFee;
- PriorityFee.

The transaction must commit to the fee parameters needed for deterministic admission and execution.

## 7. Signing

The signed payload must be generated from canonical serialization and include chain/domain information.

Never sign a UI-rendered or JSON-formatted representation directly unless that exact representation is itself the canonical protocol format.

## 8. Transaction Types

Initial transaction families may include:

- native asset transfer;
- validator/staking operations;
- governance operations;
- contract invocation;
- contract deployment;
- token operations.

The exact type system should remain compact and versioned.

## 9. Mempool Interaction

A transaction can enter the mempool only after inexpensive validation passes. Expensive execution checks should be controlled to prevent denial-of-service amplification.

Mempool ordering, replacement, eviction, TTL, and capacity are protocol-adjacent policies and must be documented separately.

## 10. Wallet Requirements

The Flutter/Dart wallet must be able to:

- construct canonical transactions;
- estimate or request gas;
- display human-readable intent;
- sign locally;
- submit through RPC;
- track inclusion/finality;
- handle replacement or failure states.

## 11. Test Vectors

Before protocol freeze, provide fixtures for:

- canonical encoding;
- transaction hash;
- signing bytes;
- valid signature;
- invalid signature;
- wrong chain ID;
- wrong nonce;
- invalid fee;
- malformed payload;
- contract transaction if VM is enabled.
