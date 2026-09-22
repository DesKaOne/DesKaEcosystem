# IndoChain Protocol Freeze v0.1

> Status: Draft Freeze Candidate
> Scope: Devnet protocol baseline

## 1. Purpose

This document converts the current IndoChain documentation into a reviewable protocol baseline before large-scale Go implementation.

This is a **Devnet v0.1 freeze candidate**, not a Mainnet specification. Items explicitly marked TBD remain open and must not be invented by implementation code.

## 2. Frozen Baseline

The following architectural directions are currently treated as the v0.1 baseline:

- primary node/core language: Go;
- primary wallet stack: Flutter/Dart;
- account-based state model;
- modular consensus architecture;
- PoS+BFT as the production direction;
- optional PoW/mining module for development, experiments, and benchmark profiles;
- P2P networking separated from consensus logic;
- mempool separated from canonical state;
- canonical node state stored in node storage;
- PostgreSQL reserved for service/indexer/application data;
- Devnet/Testnet/Mainnet as explicit network profiles;
- faucet restricted to Devnet/Testnet;
- JSON-RPC and WebSocket as primary external node interfaces;
- VM architecture remains modular with EVM/WASM candidates;
- token standards remain protocol design targets rather than fully frozen implementations.

## 3. Chain Identity

Every network instance must have an explicit chain identity.

Required identity fields:

- chain ID;
- network name/profile;
- protocol version;
- genesis identifier;
- genesis timestamp;
- network-specific configuration.

A transaction must not be accepted when its replay-protection/domain information is incompatible with the selected chain.

Exact production chain IDs and final genesis allocations remain subject to the genesis specification.

## 4. Block Contract

The current block model contains:

~~~text
Block
 ├── Height
 ├── Timestamp
 ├── PreviousHash
 ├── MerkleRoot
 ├── StateRoot
 ├── TransactionsRoot
 ├── Validator
 ├── Signature
 └── Transactions[]
~~~

The implementation must define one canonical serialization for the block header and transaction list before consensus-critical code is finalized.

Exact field widths, encoding, ordering, and signature domain remain implementation-freeze items.

## 5. Transaction Contract

The transaction layer must support at minimum:

- sender/account identity;
- nonce;
- recipient or execution target where applicable;
- value/asset information;
- gas/fee information;
- payload/data;
- chain/replay protection;
- signature/authentication data.

Transactions must be deterministically serializable and independently verifiable.

## 6. Canonical Serialization

A single canonical encoding is required for consensus-critical objects.

The following must not depend on UI formatting, JSON field ordering, locale, or platform-specific serialization:

- transaction hash;
- signing bytes;
- block hash;
- state commitment inputs;
- consensus messages.

The exact encoding format is **TBD** and must be frozen before interoperability testing.

## 7. Cryptography

The crypto layer must be isolated behind interfaces so that protocol-critical code does not depend directly on wallet/UI implementations.

The final v0.1 crypto suite is **TBD**.

Before freeze, specify:

- hash function;
- signature algorithm;
- public-key encoding;
- signature encoding;
- address derivation;
- domain separation rules;
- randomness requirements where applicable.

## 8. State Transition

The account model is the current primary direction.

A valid state transition must be deterministic across all nodes.

Conceptually:

~~~text
State(n) + ValidTransaction(s) + ProtocolRules
                    ↓
                State(n+1)
~~~

State transition logic must validate authorization, nonce, balances, fees/gas, execution results, and state commitments according to the final protocol rules.

## 9. Gas and Fees

The current design includes:

- GasLimit;
- GasUsed;
- GasPrice;
- BaseFee;
- PriorityFee;
- minimum gas price policy;
- optional burn mechanism.

Exact fee calculation, base-fee adjustment, gas schedule, refund behavior, and burn percentage remain **TBD**.

## 10. Consensus

Consensus is modular.

The architecture contains:

~~~text
consensus/
├── pos/
├── bft/
└── pow/
~~~

PoS+BFT is the current production direction.

PoW/mining is an optional profile/module and is not assumed to be the production consensus.

The final validator selection, proposer selection, voting phases, quorum, epoch rules, finality rules, reward rules, and slashing rules remain **TBD**.

## 11. Validator Lifecycle

The protocol must represent a validator lifecycle conceptually covering:

1. registration;
2. stake;
3. activation;
4. participation;
5. rewards/slashing;
6. exit;
7. withdrawal where applicable.

Exact staking amounts, activation thresholds, unbonding period, reward calculation, and slashing percentages remain open.

## 12. Mempool

The mempool is non-canonical and must not be treated as blockchain state.

Admission must verify basic transaction validity before a transaction is propagated or queued.

Required policy areas:

- nonce handling;
- fee ordering;
- replacement rules;
- per-account limits;
- global limits;
- eviction;
- expiration;
- DoS protection.

Exact numeric limits remain **TBD**.

## 13. P2P

The node networking layer must support:

- peer discovery;
- authenticated/identified handshakes;
- block propagation;
- transaction propagation;
- consensus-message propagation;
- synchronization;
- peer scoring;
- rate limits.

TCP and QUIC remain candidates. The final transport is not frozen.

Consensus messages must remain distinguishable from ordinary transaction/block gossip.

## 14. Synchronization

The node must support a synchronization strategy capable of recovering:

- headers/blocks;
- transactions as required;
- canonical state;
- snapshots where enabled.

Every synchronization path must verify chain identity, genesis identity, block linkage, signatures/consensus evidence, and state commitments according to the selected consensus model.

Snapshot format and fast-sync protocol remain **TBD**.

## 15. VM Interface

The VM remains modular.

Current candidates:

- EVM;
- WASM;
- custom VM.

The implementation must avoid coupling core state-transition code to one VM until the VM decision is frozen.

The VM interface should expose deterministic execution inputs/outputs, gas accounting, state access, logs/events, and execution failure semantics.

## 16. RPC and WebSocket

Primary external interfaces:

- JSON-RPC over HTTP;
- WebSocket subscriptions.

Optional future interface:

- gRPC.

Administrative APIs must be isolated and access-controlled.

RPC responses must expose enough chain identity information for clients to detect wrong-network configuration.

## 17. Genesis

Genesis defines the initial network identity and state.

Required categories include:

- chain ID;
- network profile;
- protocol version;
- genesis timestamp;
- initial state;
- initial validators where applicable;
- initial allocations;
- protocol parameters.

Devnet, Testnet, and Mainnet genesis configurations must be separate artifacts.

## 18. Mining and Faucet

Mining is an optional PoW module for appropriate development/experimental profiles.

The faucet is **Devnet/Testnet only**.

Faucet infrastructure must fail closed if network identity does not match an allowed non-production profile.

Mainnet code paths must not silently enable faucet functionality.

## 19. Wallet Contract

The wallet must:

- keep private keys local;
- build canonical transactions;
- sign canonical signing bytes;
- bind signatures to chain/network domain information;
- submit signed transactions through RPC;
- support explicit network profiles.

The Go node and Dart wallet must share protocol test vectors.

## 20. Token and Ecosystem Layer

IND-20, IND-721, and IND-1155 are current design targets.

Explorer, SDK, indexer, oracle, DEX, DeFi, and bridge components are ecosystem layers and must not become implicit consensus-critical dependencies without an explicit protocol decision.

## 21. Versioning and Upgrades

Every consensus-critical protocol change must have an explicit protocol version.

Upgrade mechanisms should define:

- activation condition;
- compatibility window;
- rollback/impossibility assumptions;
- node upgrade requirements;
- governance authorization where applicable.

Exact governance and activation mechanics remain **TBD**.

## 22. Required Test Vectors

Before Go implementation is considered protocol-compatible, create deterministic vectors for:

- hashing;
- key/address derivation;
- signatures;
- transaction serialization;
- transaction hashes;
- block serialization;
- block hashes;
- state transitions;
- gas calculations;
- genesis identity;
- RPC request/response examples;
- P2P message encoding.

Go and Dart implementations must consume the same vectors.

## 23. Freeze Gate

Protocol v0.1 can move from draft to implementation freeze only after these decisions are explicitly recorded:

- canonical serialization;
- final v0.1 crypto suite;
- transaction exact schema;
- block header exact schema;
- chain ID/genesis identity format;
- state commitment format;
- gas/fee formula;
- consensus message schema;
- P2P message schema;
- VM interface;
- validator/finality rules for the selected Devnet profile.

Anything not listed as frozen remains a documented open decision and must not be silently chosen by implementation.
