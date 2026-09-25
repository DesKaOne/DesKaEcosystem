# IndoChain Protocol

> Status: Draft / Protocol Design Baseline

## 1. Scope

Dokumen ini mendefinisikan kontrak dasar antara node IndoChain. Tujuannya adalah menyediakan spesifikasi yang cukup jelas untuk implementasi, testing, wallet integration, RPC, dan future compatibility.

## 2. Protocol Layers

~~~text
Application
    │
RPC / WebSocket
    │
Transaction / Wallet Protocol
    │
Execution / State Transition
    │
Block / Consensus
    │
P2P / Synchronization
    │
Storage
~~~

Setiap layer harus memiliki boundary yang eksplisit.

## 3. Block Contract

Minimum block information:

- Height
- Timestamp
- PreviousHash
- MerkleRoot
- StateRoot
- TransactionsRoot
- Validator
- Signature
- Transactions

A block is accepted only after structural, cryptographic, consensus, and state-transition validation succeeds.

## 4. Transaction Contract

A transaction must have:

- deterministic encoding;
- sender/account identification;
- nonce or equivalent replay-prevention field;
- payload/action;
- fee/gas parameters where applicable;
- signature;
- protocol version or domain-separation context where required.

The exact binary encoding and field order will be frozen in the transaction specification before mainnet.

## 5. State Transition

Nodes must produce the same resulting state from the same valid parent state, block, and protocol version.

Conceptually:

~~~text
State(n+1) = Apply(State(n), Block(n+1), ProtocolRules)
~~~

Execution must be deterministic. Local wall-clock time, random local state, network arrival order, or external mutable services must not alter canonical execution unless explicitly represented by protocol data.

## 6. Validation Pipeline

Suggested order:

1. Decode and version-check.
2. Validate basic structure.
3. Validate hashes and references.
4. Validate signatures.
5. Validate nonce/replay rules.
6. Validate fee/gas constraints.
7. Validate authorization.
8. Execute state transition.
9. Verify resulting roots.
10. Apply consensus/finality rules.

The exact order may be optimized, but consensus-visible semantics must remain deterministic.

## 7. Chain Identity

The protocol must distinguish:

- network/chain ID;
- protocol version;
- genesis identity;
- transaction signing domain.

This prevents accidental cross-network replay and makes upgrades explicit.

## 8. Native Asset

The current design uses **dIDR** as the proposed native asset name, with the documented concept of an asset inspired by Rupiah denomination. Monetary policy, supply, emission, allocation, and other tokenomics remain separate design decisions.

## 9. Fees and Gas

The protocol direction includes:

- GasLimit
- GasUsed
- GasPrice
- BaseFee
- PriorityFee
- minimum gas price
- congestion-aware fee policy
- optional fee burn

Exact fee semantics and gas schedule must be specified before production deployment.

## 10. Protocol Versioning

Protocol changes should be explicit and testable. Candidate mechanisms include:

- versioned block/transaction formats;
- activation heights or epochs;
- feature flags only where deterministic;
- governance-approved upgrade metadata;
- compatibility windows for network software.

No upgrade mechanism should permit ambiguous state-transition rules.

## 11. Compatibility Requirements

Wallets, SDKs, RPC clients, explorers, and validators should consume versioned protocol definitions rather than duplicating undocumented assumptions.

## 12. Freeze Gate

Before large-scale implementation, freeze at minimum:

- block schema;
- transaction schema;
- canonical serialization;
- hashing/signing domains;
- chain ID/genesis identity;
- state transition rules;
- consensus message semantics;
- P2P message types;
- gas/fee semantics;
- VM interface;
- genesis format.

After the freeze, changes should be tracked as protocol changes rather than casual implementation edits.
