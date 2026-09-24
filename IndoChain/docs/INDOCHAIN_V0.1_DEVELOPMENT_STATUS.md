# IndoChain v0.1 — Development Status Summary

> Snapshot: 2026-09-23  
> Branch: `dev/indochain-v0.1`  
> Repository: `DesKaOne/DesKaEcosystem`  
> Purpose: handover / continuity document for future development chats.

## 1. Executive Summary

IndoChain v0.1 sudah melewati tahap architecture-only dan telah masuk ke **core protocol implementation / pre-consensus integration**.

Fondasi deterministic blockchain mulai tersedia: transaction, cryptography, state transition, block execution, genesis/devnet, storage, node recovery, mempool, P2P dan sync foundation. Dokumentasi protocol juga sudah bergerak menuju freeze/test-vector driven development.

Namun IndoChain **belum merupakan blockchain production-ready**. Pekerjaan besar berikutnya adalah consensus engine yang operasional, validator/block production runtime, native fee/gas integration, multi-node devnet/testnet validation, lalu EVM execution layer.

> Catatan: status di dokumen ini adalah ringkasan hasil review branch pada 2026-09-23. Status CI run tertentu harus diverifikasi terhadap commit HEAD saat akan digunakan sebagai release gate.

---

## 2. Status Area

| Area | Status | Catatan |
|---|---|---|
| Go project / module structure | 🟢 | Core implementation tersedia |
| Block model | 🟢 | Block/hash/header/state-root related components tersedia |
| Transaction model | 🟢 | Transaction validation, encoding/hash/signing tersedia |
| Cryptography | 🟢 | Ed25519/address/hash components + tests |
| Account state | 🟢 | Account/balance/nonce state model |
| State transition | 🟢 | Deterministic transfer execution + snapshot/atomic replacement |
| State root | 🟢 | State root calculation/verification |
| Transactions root | 🟢 | Root validation/execution boundary |
| Genesis / devnet | 🟢 | Genesis/devnet creation and tests |
| Persistent storage | 🟢 | Chain/state storage abstractions and file-backed implementation |
| Node open/recovery | 🟢 | Stored history/state integrity validation |
| Mempool | 🟢 | Development implementation + deterministic candidate ordering boundary |
| P2P | 🟢 | Handshake, messages, gossip/propagation, peer controls |
| Block sync | 🟢 | Sync protocol components and coordinator/session machinery |
| Read RPC | 🟢 | Native read-side endpoints/components |
| CI | 🟢 | Go test/vet workflow exists; latest run must be checked separately |
| PoS | 🟡 | Protocol/design direction; runtime not yet complete |
| BFT consensus | 🟡 | Message/round-state, validator, voting-power, proposer, vote-aggregation, and finality boundaries implemented; production algorithm still pending |
| Validator runtime | 🟡 | Development orchestration boundary implemented; production consensus loop not yet complete |
| Block production | 🟡 | Not yet a complete production loop |
| Native gas/fee | 🟡 | Design direction exists; execution integration remains |
| EVM | 🔴 | Not yet implemented as execution runtime |
| EVM JSON-RPC | 🔴 | Target only; not yet Ethereum-compatible runtime |
| Smart contracts | 🔴 | Depends on EVM execution |
| Fee sponsorship | 🔴 | Protocol design direction only |
| Token/NFT layer | 🔴 | Ecosystem roadmap |
| Explorer | 🔴 | Roadmap |
| Wallet | 🔴 | Roadmap |
| Mainnet | 🔴 | Far future; security/testnet gates required |

Legend:
- 🟢 implemented/foundation available
- 🟡 design or partial implementation / integration pending
- 🔴 roadmap or not yet implemented

---

## 3. Current Core Pipeline

The current architecture is moving toward:

```text
Transaction
    ↓
Validate
    ↓
Verify Signature
    ↓
State Snapshot
    ↓
Apply State Transition
    ↓
Calculate State Root
    ↓
Block Execution
    ↓
Commit Canonical State
```

Node-level direction:

```text
Client / RPC
    ↓
Transaction
    ↓
Mempool
    ↓
Block Production
    ↓
Consensus
    ↓
Block
    ↓
Execute Block
    ↓
State + Block Store
    ↓
P2P / Sync
```

The first pipeline is substantially implemented. The consensus/block-production part is the major missing integration.

---

## 4. Implemented Foundations

### 4.1 Transaction and cryptography

Transaction handling already has implementation around:

- encoding/serialization
- hashing
- validation
- signing
- signature verification
- nonce/value/sender/recipient related checks

Cryptographic components include Ed25519, address encoding/Base58 and hashing.

A recent implementation commit added a development Ed25519 signer.

### 4.2 State

The state model is account-based.

Conceptually:

```text
Account
├── Balance
└── Nonce
```

State transition uses validation and snapshot-style application so failed execution does not partially mutate canonical state.

### 4.3 Block execution

Block execution validates block-related roots, executes transactions, calculates the resulting state root, and verifies the resulting state before canonical commit.

This is a critical boundary and should remain deterministic across all nodes.

### 4.4 Genesis and devnet

Genesis/devnet creation and tests exist.

Node opening/recovery also validates stored chain/state history instead of blindly trusting persisted data.

### 4.5 Storage

Storage is abstracted behind chain/state store interfaces, with memory and file-backed implementations.

Principle:

```text
Node
 ↓
ChainStore
 ├── MemoryStore
 └── FileStore
```

The node's canonical state should remain in node storage. PostgreSQL is intended for external/indexing/service layers, not canonical blockchain state by default.

### 4.6 Mempool

Mempool components and node integration exist.

Target flow:

```text
Wallet / RPC
    ↓
Mempool
    ↓
Validator / Block Producer
    ↓
Block
```

### 4.7 P2P and synchronization

P2P development has gone beyond a basic networking skeleton. Components cover areas such as:

- peer handling
- handshake
- message protocol
- gossip/propagation
- rate limiting / peer controls
- synchronization
- sync coordination
- sync sessions/cursors/ranges
- request/response handling

This is a foundation, not yet proof of a production-ready multi-node network.

### 4.8 Native read RPC

Native read-side RPC functionality exists for chain/head/block/transaction-oriented queries.

EVM `eth_*` compatibility is still future work.

### 4.9 Consensus Message Boundary

The first executable consensus-message boundary is now implemented under `IndoChain/internal/consensus`.

Current development message model contains protocol version, chain ID, epoch, height, round, sender/validator identifier, message type, payload, and signature.

Supported logical message types are proposal, vote, finality evidence, and validator-set update.

The boundary validates chain/version context, message type, sender/signature requirements, and payload size. Consensus signing uses the dedicated INDOCHAIN-CONSENSUS domain and deterministic development signing bytes.

This milestone deliberately does not implement proposer selection, quorum, validator-set transitions, finality, or P2P transport.

### 4.10 Consensus Round State Boundary

The next consensus foundation is now implemented under `IndoChain/internal/consensus/state.go`.

`RoundState` tracks protocol version, chain ID, epoch, height, round, and development consensus phase. The current phase model is Proposal → Prevote → Precommit → Finalized.

The implementation enforces monotonic phase, round, and height transitions. Increasing the round resets phase to Proposal; increasing height resets round to zero and phase to Proposal. Invalid regressions are rejected without mutating the source value.

This remains a development abstraction. It does not yet implement proposer selection, validator sets, voting power, quorum, timeouts, vote aggregation, evidence, finality certificates, or persistence.

### 4.11 Consensus Message ↔ Round State Context Boundary

The consensus message and round-state foundations are now connected by `IndoChain/internal/consensus/state_message.go`.

`ValidateMessageAgainstState` requires exact equality for protocol version, chain ID, epoch, height, and round before a message is considered to belong to the current consensus context.

The boundary validates the round state first and rejects context mismatches without mutating either value. This provides the next deterministic guard before message-type semantics, validator authority, voting power, quorum, and finality are introduced.

This remains a development abstraction. It does not yet validate proposer eligibility, validator membership, vote power, quorum, timeout behavior, evidence, finality certificates, or validator-set transitions.

### 4.13 Consensus Message Validation Pipeline Boundary

A composed consensus-message validation pipeline is now implemented under `IndoChain/internal/consensus/message_pipeline.go`.

`ValidateConsensusMessage` applies the existing boundaries in dependency order: message structure/protocol validation, exact round-state context validation, then validator membership/sender authorization.

The pipeline is intentionally limited to existing invariants. Cryptographic signature verification remains a separate boundary through `VerifyMessageSignature`, because the current membership model does not define a public-key-to-validator authority mapping beyond the supplied validator identifier.

Tests cover valid messages, context mismatch, unauthorized sender, input purity, and follow-on signature verification. Detailed scope is documented in `IndoChain/docs/consensus-message-validation-pipeline-v0.1.md`.

This milestone does not define proposer selection, voting power, quorum, vote aggregation, locking, timeouts, finality certificates, validator-set transitions, staking, rewards, or slashing.

---


### 4.14 Consensus Voting Power & Quorum Boundary

A deterministic voting-power boundary is now implemented under `IndoChain/internal/consensus/voting_power.go`.

`VotingPowerSet` binds validator identifiers to externally supplied positive `uint64` voting power, clones identifiers, sorts them deterministically, rejects duplicates/invalid entries, supports lookup, and calculates checked total voting power.

A generic `QuorumThreshold` and `QuorumReached` helper are also implemented. The threshold is caller-supplied as numerator/denominator, so this milestone does not freeze a production quorum fraction. Quorum comparison uses arbitrary-precision arithmetic to avoid `uint64` multiplication overflow.

Tests cover deterministic ordering/cloning, invalid and duplicate entries, lookup/total power, threshold validation, and below/exact/above quorum cases.

This milestone intentionally does not define how stake maps to voting power, delegation, validator activation, proposer selection, vote aggregation, locking, timeouts, finality certificates, rewards, slashing, or validator-set transitions. Detailed scope is documented in `IndoChain/docs/consensus-voting-power-quorum-boundary-v0.1.md`.

---


### 4.16 Consensus Vote Aggregation Boundary

A development-only vote aggregation boundary is now implemented under `IndoChain/internal/consensus/vote_aggregator.go`.

`VoteAggregator` binds one exact round-state context to the existing message validation, validator membership, voting-power, and quorum primitives. It accepts only vote messages, prevents duplicate votes from the same validator, requires the sender to have voting power, and aggregates voting power for an exact opaque vote payload.

Quorum evaluation remains caller-supplied through the existing `QuorumThreshold` and overflow-safe `QuorumReached` helper. Signature verification remains a separate boundary because the current v0.1 validator model does not define a canonical validator-identifier-to-public-key mapping.

Tests cover non-vote rejection, duplicate sender rejection, missing voting power, payload-specific power/quorum calculation, input cloning, and context mismatch. Detailed scope is documented in `IndoChain/docs/consensus-vote-aggregation-boundary-v0.1.md`.

This milestone intentionally does not define prevote/precommit semantics, locking, timeouts, round advancement, finality certificates, validator-set transitions, production quorum fraction, canonical vote payload serialization, persistence, or P2P vote transport.

### 4.15 Consensus Proposer Selection Boundary

A deterministic proposer-selection boundary is now implemented under `IndoChain/internal/consensus/proposer.go`.

`ProposerSelector` defines the abstraction, while `RoundRobinProposer` provides a development-only deterministic selector. It validates round state and validator membership, rejects an empty validator set, uses the canonical byte-sorted validator order, selects `round mod validator_count`, and returns a cloned validator identifier.

This selector intentionally does not model stake, voting power, proposer priority, randomness/VRF, validator performance, rewards, or slashing. It therefore does not freeze the production PoS proposer algorithm. Detailed scope is documented in `IndoChain/docs/consensus-proposer-selection-boundary-v0.1.md`.

### 4.17 Consensus Finality Certificate Boundary

A development-only finality certificate boundary is now implemented under `IndoChain/internal/consensus/finality.go`.

`FinalityCertificate` binds one exact protocol/chain/epoch/height/round context to an opaque payload, a caller-supplied quorum threshold, and unique validator votes. `NewFinalityCertificate` reuses the existing consensus message validation, validator membership, voting-power, quorum, and vote-aggregation boundaries and refuses to create a certificate unless the supplied votes reach quorum.

`ValidateFinalityCertificate` independently reconstructs the aggregation context and validates context equality, threshold, validator authorization, voting-power membership, duplicate-vote rejection, payload-specific voting power, and quorum. Validation is non-mutating and does not advance `RoundState`.

Tests cover quorum failure, input cloning, context mismatch, mixed-payload evidence, duplicate senders, and non-mutating validation. Detailed scope is documented in `IndoChain/docs/consensus-finality-boundary-v0.1.md`.

This milestone intentionally does not define the production BFT algorithm, prevote/precommit semantics, locking, timeouts, proposer priority/randomness, validator-set transitions, canonical certificate encoding, signature authority registry, P2P finality transport, block-production integration, or multi-height finality gadget.

---

### 4.18 Validator Runtime Boundary

A deterministic development validator runtime is now implemented under `IndoChain/internal/consensus/runtime.go`.

`ValidatorRuntime` composes the existing proposer-selection, consensus-message validation, vote-aggregation, quorum, and finality-certificate boundaries into a controlled lifecycle:

`Proposal → Prevote → Precommit → Finalized`

Proposal acceptance requires the expected proposer for the current round and a valid consensus context. Votes are accepted through the existing aggregation boundary. When the caller-supplied quorum is reached for the accepted opaque payload, the runtime advances to Precommit. Finalization then creates a `FinalityCertificate` and advances the development phase to Finalized.

Rejected proposal/vote paths do not advance the runtime. Finalization does not commit a block or mutate canonical chain state.

Tests cover expected/unexpected proposer behavior, phase safety, quorum-driven progression, finality certificate creation, and refusal to finalize before quorum. Detailed scope is documented in `IndoChain/docs/consensus-validator-runtime-boundary-v0.1.md`.

This milestone intentionally does not define the production BFT algorithm, timeout/round-change behavior, validator-set lifecycle, canonical proposal/finality encoding, block execution/commit, persistence, P2P transport, signature authority registry, rewards, or slashing.

---


### 4.19 Consensus ↔ Block Production Boundary

A development block-production interface is now implemented under `IndoChain/internal/consensus/block_production.go`.

`BlockProductionContext` supplies the current `RoundState`, previous block hash, and proposer identifier. `BlockProducer` defines the integration point for constructing the next candidate block without prescribing transaction selection or fee policy.

`ValidateProducedBlock` verifies protocol/chain context, next height, previous hash, proposer identity, and the existing development transactions-root commitment. It returns the deterministic development block hash as the opaque proposal payload that can be carried by the existing consensus proposal/vote boundaries.

Tests cover deterministic proposal payload generation and rejection of height/proposer context mismatches. Detailed scope is documented in `IndoChain/docs/consensus-block-production-boundary-v0.1.md`.

This milestone intentionally does not implement transaction selection, fee/gas accounting, state execution, persistence, canonical serialization, P2P proposal transport, timeout/round-change behavior, or production BFT semantics.

### 4.20 Consensus ↔ Mempool Deterministic Ordering Boundary

A deterministic development ordering boundary is now implemented in `IndoChain/internal/mempool/mempool.go` through `Pool.SnapshotSorted()`.

The existing mempool stores transactions in a Go map, so raw `Snapshot()` iteration order is not suitable for canonical block construction. `SnapshotSorted()` copies admitted transactions and orders them by transaction hash before returning the candidate sequence.

This closes an important determinism gap between the mempool and the existing block-production boundary without freezing the final transaction-selection policy. Fee/gas priority, nonce sequencing across senders, block limits, stale-transaction eviction, fee sponsorship ordering, and other economic selection rules remain open.

Tests cover repeatable ordering, strict transaction-hash ordering, and the non-mutating nature of the sorted snapshot. Detailed scope is documented in `IndoChain/docs/consensus-mempool-ordering-boundary-v0.1.md`.

This milestone intentionally does not implement block production, state execution, gas/fee accounting, persistence, P2P mempool propagation, or consensus finality.

### 4.21 Consensus ↔ Block Candidate Construction Boundary

A deterministic block-candidate construction primitive is now implemented in `IndoChain/internal/consensus/block_candidate.go`.

`BuildBlockCandidate` takes an explicit transaction sequence, snapshots canonical state, executes the supplied development transactions, calculates the deterministic transactions root and resulting state root, and constructs the next block candidate. The candidate is then checked through the existing `ValidateProducedBlock` boundary.

The builder preserves canonical-state immutability and validates consensus/execution context alignment. Transaction selection remains outside the builder so the mempool ordering boundary and future economic selection policy remain separate from block construction.

Tests cover deterministic empty-block construction, next-height calculation, state-root preservation, canonical-state non-mutation, execution-context mismatch, and nil-state rejection. Detailed scope is documented in `IndoChain/docs/consensus-block-candidate-construction-boundary-v0.1.md`.

The existing v0.1 execution rule still exposes one explicit public key. This milestone therefore does not invent a sender-to-public-key registry for multi-sender blocks. Production multi-sender block construction remains dependent on a canonical authority/key-resolution design.

This milestone intentionally does not freeze fee/gas accounting, block limits, canonical serialization, public-key registry, proposer policy, production BFT semantics, persistence, or P2P block-proposal transport.
### 4.18.1 CI Fix — Validator Runtime Aggregator Ownership

IndoChain CI run `610` failed during compilation in `internal/consensus/runtime.go:79`: `NewVoteAggregator` returns a `VoteAggregator` value, while `ValidatorRuntime.votes` is a `*VoteAggregator`. The runtime constructor now stores `&aggregator` so the field type and assignment agree. The proposer comparison was also normalized to `bytes.Equal` for explicit byte-slice equality.

Fix commit: `ac27d456622a6d8b3751832e7a73715b3c807adf`.

This is a compile-level CI fix only; no production BFT semantics are introduced.

### 4.22 Block Candidate Native Transaction Execution Coverage

The block-candidate construction boundary has now been exercised with a real development Ed25519-signed native transaction, not only an empty-block case.

The new test builds a signed transaction, seeds the sender account, runs `BuildBlockCandidate`, verifies the canonical state remains unchanged, and independently confirms that the candidate state root matches the deterministic state produced by applying the same transaction to a snapshot.

This strengthens the existing boundary without introducing a sender-to-public-key registry: the v0.1 execution rule still receives one explicit public key, so this test intentionally covers the supported single-signer execution path only.

The block-candidate documentation has been updated to record this coverage in `IndoChain/docs/consensus-block-candidate-construction-boundary-v0.1.md`.

This milestone does not freeze multi-sender key resolution, canonical transaction/block serialization, fee/gas policy, block limits, or production consensus semantics.

### 4.23 Consensus ↔ Block Candidate Proposal Bridge

A development-only in-memory bridge is now implemented under `IndoChain/internal/consensus/block_proposal.go`.

`BlockProposal` binds a validated block candidate to the deterministic development block hash returned by `ValidateProducedBlock`. `MessagePayload()` exposes a cloned 32-byte opaque payload suitable for the existing consensus proposal/vote messages, while `SamePayload()` provides deterministic identity comparison.

This connects the output of `BuildBlockCandidate` to the existing `ValidatorRuntime` payload lifecycle without making consensus parse arbitrary block bytes. The candidate remains available to the block-production/validation caller, while consensus currently carries only the development block hash as opaque payload.

Tests cover candidate-to-payload bridging, invalid-candidate rejection, payload round-trip, and payload cloning. Detailed scope is documented in `IndoChain/docs/consensus-block-candidate-proposal-bridge-v0.1.md`.

This milestone intentionally does not freeze canonical block serialization, consensus wire encoding, P2P proposal transport, block execution/commit during finalization, timeout/round-change behavior, production BFT semantics, validator-set lifecycle, or proposer priority/randomness.

## 5. Protocol Documentation Progress

Documentation has advanced into more formal protocol specification work, including:

- consensus design
- validator design
- P2P design
- mempool design
- storage design
- genesis design
- sync design
- mining/production design
- faucet design
- RPC design
- WebSocket design
- VM design
- gas design
- token design
- economics specification
- governance specification
- security specification
- wallet architecture
- explorer design
- SDK design
- indexer design
- oracle design
- DEX design
- DeFi design
- bridge design
- README specification

Recent protocol-documentation milestones include a **protocol freeze v0.1 candidate** and a **protocol test vector specification**.

The purpose of test vectors is to make protocol behavior deterministic and independently verifiable.

---

## 6. Current Architecture Decision: Native Blockchain + EVM

The latest architecture direction is:

```text
                    IndoChain
                        │
        ┌───────────────┼────────────────┐
        │               │                │
   Consensus Layer  Native Asset     EVM Layer
        │               │                │
      PoS+BFT           dIDR         Solidity
      Validator         Native       Smart Contract
      P2P/Block         Asset         ABI
                                      JSON-RPC
                                      Tooling
                        │                │
                        └───────┬────────┘
                                │
                         Fee Sponsorship
```

IndoChain is a **native blockchain**, not an Ethereum clone.

EVM is the planned **execution layer / developer compatibility boundary**.

Target developer flow:

```text
Solidity
   ↓
Hardhat / Foundry / Remix
   ↓
IndoChain JSON-RPC
   ↓
EVM Execution
   ↓
IndoChain State
```

The native protocol remains independent from EVM tooling.

---

## 7. Native Asset: dIDR

`dIDR` is the planned native asset of IndoChain.

Potential protocol uses:

- transaction fee
- staking
- validator reward
- governance
- smart contract execution
- resource/storage fee

Final tokenomics, supply, emission, monetary policy and genesis allocation are not considered production-final until their dedicated specifications are finalized.

---

## 8. Fee Sponsorship Direction

Fee sponsorship is planned as a **native IndoChain protocol feature**, not assumed to be ERC-4337/Paymaster.

Conceptual transaction:

```text
Transaction
├── from
├── to
├── amount
├── nonce
├── fee
├── signature
└── optional fee_payer / sponsor
```

The exact authorization, signatures, limits, replay protection, refund behavior and anti-abuse rules still require a dedicated technical specification and implementation.

---

## 9. Native Account vs EVM Account

The conceptual wallet model is:

```text
IndoChain User
│
├── Native Account
│      └── native dIDR balance
│
└── EVM Account
       └── smart contract / token interaction
```

The exact account/address binding and final address specification remain open technical work.

---

## 10. DesKaCash Boundary

DesKaCash and IndoChain must keep separate sources of truth.

```text
DesKaCash
└── PostgreSQL
    └── Fiat IDR Ledger

IndoChain
└── Blockchain State
    └── Native dIDR / EVM State
```

DesKaCash can store integration mapping such as:

- `user_id`
- `blockchain_address`
- `chain_id`
- wallet/account type
- integration status

DesKaCash must not become the canonical database for on-chain balance.

IDR ↔ dIDR conversion is a separate settlement/conversion process.

---

## 11. Recommended Next Development Sequence

The next work should follow the dependency order rather than jumping directly into DEX/DeFi/bridge:

```text
1. Lock core protocol invariants
        ↓
2. Complete consensus engine
        ↓
3. Validator runtime
        ↓
4. Block production
        ↓
5. Finality
        ↓
6. Multi-node devnet
        ↓
7. Native gas/fee integration
        ↓
8. Protocol test vectors + cross-node tests
        ↓
9. EVM execution layer
        ↓
10. EVM JSON-RPC compatibility
        ↓
11. Contract deployment/execution tests
        ↓
12. SDK / explorer / wallet integration
        ↓
13. Testnet hardening
        ↓
14. Security review / audit
        ↓
15. Mainnet preparation
```

Do not treat DEX, DeFi, bridge, oracle, NFT or advanced ecosystem features as blockers for the core protocol.

---

## 12. Architecture Guardrails for Future Chats

Future implementation chats should preserve these rules:

1. IndoChain is a native blockchain.
2. Go is the primary protocol/node language.
3. dIDR is the planned native asset.
4. EVM is an execution layer, not the base blockchain.
5. Consensus, P2P, state, storage and block logic must not depend on wallet/UI services.
6. Canonical blockchain state belongs to the node state/storage layer.
7. PostgreSQL is for indexer/service layers where appropriate.
8. Protocol-critical behavior must be deterministic.
9. Failed execution must not partially mutate canonical state.
10. Protocol changes should be accompanied by tests/test vectors where applicable.
11. Do not silently treat roadmap documentation as implemented functionality.
12. Do not claim EVM compatibility until an actual EVM execution/runtime and compatibility tests exist.
13. Do not claim production/mainnet readiness before multi-node testing, observability, backup/sync strategy and security review are complete.
14. Fee sponsorship must be specified at protocol level before implementation.
15. Keep native and EVM account/address responsibilities explicit.

---

## 13. Current Development Classification

**Current stage:**

> **Core Protocol Implementation / Pre-Consensus Integration**

Meaning:

- architecture: established
- protocol specification: substantially documented and moving toward freeze
- deterministic state/block foundation: implemented
- storage/recovery: implemented foundation
- mempool/P2P/sync: substantial foundation
- consensus message boundary: implemented foundation
- consensus round-state boundary: implemented foundation
- consensus message ↔ round-state context boundary: implemented foundation
- consensus validator membership boundary: implemented foundation
- consensus message validation pipeline: implemented foundation
- consensus voting-power/quorum boundary: implemented foundation
- consensus proposer-selection boundary: implemented development foundation; production proposer policy remains open
- consensus vote aggregation boundary: implemented development foundation
- consensus finality certificate boundary: implemented development foundation; production finality semantics remain open
- EVM: future execution milestone
- production network: not yet

This classification should be updated whenever a major milestone is completed.

---

## 14. Handover Checklist for a New Chat

When a future chat is opened because the previous development chat is full, start by reading:

```text
IndoChain/docs/INDOCHAIN_V0.1_DEVELOPMENT_STATUS.md
```

Then inspect the current branch:

```text
dev/indochain-v0.1
```

After reading this document, the new chat should:

1. verify the current branch HEAD;
2. inspect recent commits;
3. verify which items above are still current;
4. check current CI status when relevant;
5. inspect the relevant implementation before proposing code changes;
6. preserve the architecture guardrails above;
7. update this status document after a major milestone.

---

## 15. Reference Documentation

Primary architecture document:

```text
IndoChain/docs/indochain.md
```

This status document is a **development snapshot**, not a replacement for detailed protocol specifications.

When there is a conflict, the implementation and dedicated protocol specification for the affected component must be reviewed before changing architecture.

---

## Status

**Snapshot date:** 2026-09-23  
**Latest verified green CI:** `21e43032f7ed22dc07216f73b173166470c48f08` (IndoChain CI run 556)
**Latest consensus round-state implementation:** `de59cb9d6485165613c5e7111521e27a639f69f6`
**Latest consensus message ↔ round-state context implementation:** `fbd3e3a7f88cc0fc16e6a73671bb1a02a6454041`
**Latest consensus validator membership implementation:** `f49385037bf7e22a816eab29a1ceacb49e0a9b4b`  
**Latest consensus message validation pipeline implementation:** `b3775b69552576777ba14c2c441d3e7fc2251c9a`  
**Latest consensus voting power/quorum boundary implementation:** `9e59a2286ed936e823d8b04d3a71dc78c4d987a9`  
**Latest consensus proposer-selection boundary implementation:** `4564272ddd061a80daf08dce0f2f0b6e726d58ca`  
**Latest consensus vote aggregation boundary implementation:** `631dfe9b4f66b557e6fe0c5d95d51e0773da1489`  
**Latest consensus vote aggregation CI fix:** `705a2cbb5c31c78fa43e5c8362978e4094b469de`  
**Latest consensus finality certificate implementation:** `2764fc835db1c396f102e3e88a550e85c8850e38`  
**Latest consensus finality certificate tests:** `816b9549175d20a5da2731beb1aea9434b3b1858`  
**Latest validator runtime implementation:** `ac27d456622a6d8b3751832e7a73715b3c807adf`  
**Latest deterministic mempool ordering implementation:** `ceecf3311b029ccb2d697bf201541fc1a69dc115`  
**Deterministic mempool ordering tests:** `1b2fc2cbddf2b48ff45702d76dd27ba9570772ce`  
**Deterministic mempool ordering documentation:** `16b7cbbe891f4ae765343684d736018d8c52c56f`  
**Latest validator runtime tests:** `810bbaab2b009ab7ba5bcc5f87cd9f8c18291385`  
**Validator runtime boundary documentation:** `523af557240c5e81c9b54249ccf9f3432c97c010`  
**Consensus finality boundary documentation:** `72bb276ca17848129d0f9bec0c66554aa604a6b`  
**Latest status-document update:** `fb63fba4e81ca6e9b822f52b90116694a222fb72`    
**Current branch CI after vote aggregation:** previous CI run `584` failed in `TestVoteAggregatorCalculatesPayloadPowerAndQuorum`; the test assertion has been corrected in `705a2cbb5c31c78fa43e5c8362978e4094b469de`.  
**Current branch CI after finality boundary:** no pull-request workflow run was associated with HEAD `146c2225be2dcaf787bc3f724b50d70e214dfcfc`.  
**Current branch CI after validator runtime:** IndoChain CI run `610` failed on the pull-request merge ref because `ValidatorRuntime` assigned a `VoteAggregator` value to a `*VoteAggregator` field. The runtime fix is `ac27d456622a6d8b3751832e7a73715b3c807adf`; the resulting branch HEAD later passed IndoChain CI run `622`.  
**Branch:** `dev/indochain-v0.1`  
**Stage:** Core Protocol Implementation / Pre-Consensus Integration  
**Latest block-candidate construction implementation:** `5d57d41ec1a2b901cdb63b38aa8171f07b27e754`  
**Latest block-candidate execution coverage:** `29de6a6fa35d791bbc6ae6cb3d812b421a7dd553`  
**Latest block-candidate documentation:** `239796fd04ba50378572b81a48faa07e63043f16`  
**Latest block-candidate proposal bridge:** `efdd5b3fd78ba05434a434d00b3cfbc37ccd009a`  
**Block-candidate proposal bridge documentation:** `88b139f1610921b39680301d4f24e172685a27bc`  
**Next major boundary:** Integrate validated block proposal payloads with ValidatorRuntime and finality without committing canonical state  

### 4.12 Consensus Validator Membership Boundary

A deterministic validator-membership boundary is now implemented under `IndoChain/internal/consensus/validator_set.go`.

The development `ValidatorSet` rejects empty and duplicate validator identifiers, stores identifiers in deterministic byte-sorted order, clones input identifiers, and exposes membership validation.

Consensus message sender authorization is also connected through `ValidateMessageSender`. A message sender must be present and belong to the supplied validator set before it can be treated as an active validator message.

This milestone intentionally does not define stake, voting power, delegation, registration, activation/deactivation, epoch transitions, proposer selection, rewards, or slashing. Those remain open protocol/validator decisions.

### 4.13 Consensus Message Validation Pipeline Boundary

A composed consensus-message validation pipeline is now implemented under `IndoChain/internal/consensus/message_pipeline.go`.

`ValidateConsensusMessage` applies the existing boundaries in dependency order: message structure/protocol validation, exact round-state context validation, then validator membership/sender authorization.

The pipeline is intentionally limited to existing invariants. Cryptographic signature verification remains a separate boundary through `VerifyMessageSignature`, because the current membership model does not define a public-key-to-validator authority mapping beyond the supplied validator identifier.

Tests cover valid messages, context mismatch, unauthorized sender, input purity, and follow-on signature verification. Detailed scope is documented in `IndoChain/docs/consensus-message-validation-pipeline-v0.1.md`.

This milestone does not define proposer selection, voting power, quorum, vote aggregation, locking, timeouts, finality certificates, validator-set transitions, staking, rewards, or slashing.
