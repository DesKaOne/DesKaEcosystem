# IndoChain Architecture

> Status: Draft / Architecture Baseline

## 1. Purpose

Dokumen ini mendefinisikan pembagian komponen IndoChain sebelum implementasi besar dimulai. Arsitektur harus modular agar consensus, networking, storage, execution, RPC, wallet, dan tooling dapat berkembang tanpa mengikat core pada UI.

## 2. High-Level Architecture

~~~text
                    ┌─────────────────────┐
                    │   IndoChain Wallet  │
                    │   Flutter / Dart    │
                    └──────────┬──────────┘
                               │
                         JSON-RPC / WS
                               │
                    ┌──────────▼──────────┐
                    │      RPC Node       │
                    │        Go           │
                    └──────────┬──────────┘
                               │
                    ┌──────────▼──────────┐
                    │      Mempool        │
                    └──────────┬──────────┘
                               │
                    ┌──────────▼──────────┐
                    │     Consensus       │
                    │      PoS + BFT      │
                    └──────────┬──────────┘
                               │
                    ┌──────────▼──────────┐
                    │     Execution VM    │
                    │    EVM / WASM       │
                    └──────────┬──────────┘
                               │
              ┌────────────────▼────────────────┐
              │           State DB              │
              └────────────────┬────────────────┘
                               │
                    ┌──────────▼──────────┐
                    │     Block Store     │
                    └─────────────────────┘
                         ↕ P2P / Gossip
              ┌──────────┬──────────┬──────────┐
              │ Validator│ Validator│ Validator│
              └──────────┴──────────┴──────────┘
~~~

## 3. Core Modules

- core/block: block header, block body, hashing, validation.
- core/transaction: transaction format, encoding, signing payload, validation.
- core/state: account state, balances, nonce, state transitions.
- core/crypto: hashing, signatures, key abstractions.
- consensus: validator set, proposal, voting, finality, slashing hooks.
- network: peer discovery, handshake, gossip, synchronization.
- mempool: transaction admission, ordering, eviction, fee policy.
- vm: contract execution boundary; implementation choice remains open between EVM/WASM.
- storage: canonical chain, state, indexes/cache, snapshots.
- rpc: external API boundary.
- wallet: protocol-facing wallet primitives; Flutter/Dart remains the main UI/client stack.
- validator: staking, validator lifecycle, signing and operational state.
- governance: protocol parameter and upgrade governance.
- explorer: read-only/indexed ecosystem layer.

## 4. Dependency Rules

1. UI must never be a dependency of consensus.
2. RPC must not mutate canonical state outside validated protocol paths.
3. Consensus consumes deterministic block/state transition results.
4. VM must expose a deterministic execution interface.
5. Storage implementations must be replaceable behind interfaces.
6. Network transport must not define transaction semantics.
7. Wallet must sign protocol-defined payloads rather than inventing transaction formats.

## 5. Canonical vs Service Data

Canonical blockchain data belongs to node-controlled storage. PostgreSQL may be used for indexing, analytics, search, explorer data, or other service-layer workloads, but it is not the canonical consensus database.

## 6. Go Package Direction

Proposed package layout:

~~~text
indochain/
├── cmd/
├── internal/
│   ├── core/
│   ├── consensus/
│   ├── network/
│   ├── mempool/
│   ├── vm/
│   ├── storage/
│   ├── rpc/
│   ├── validator/
│   └── governance/
├── pkg/
│   └── api/
└── docs/
~~~

The exact package visibility and repository layout may change during implementation.

## 7. Architecture Goals

- Deterministic execution.
- Clear protocol boundaries.
- Testability at module level.
- Replaceable storage and transport implementations.
- Explicit versioning for wire/protocol formats.
- Security checks at trust boundaries.
- Observable node behavior.
- Upgradeability without making every module upgradeable by default.

## 8. Open Decisions

The following are intentionally not frozen here:

- EVM vs WASM execution direction.
- Final cryptographic suite.
- Concrete database engine.
- Exact consensus implementation and timing.
- Wire protocol encoding.
- Gas schedule.
- Tokenomics and monetary policy.
- Genesis allocation.
