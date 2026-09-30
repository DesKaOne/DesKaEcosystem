# IndoChain v0.1 — Development Status Summary

> Snapshot: 2026-09-24  
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

### 4.22 Finalized Runtime → Node Replay/Re-commit Guard

The finalized consensus-to-node handoff now has an explicit append-only replay guard in `IndoChain/internal/node/node.go`.

`CommitFinalizedBlock` rejects a finalized candidate whose height is already at or below the canonical node head before validator-authority resolution or transaction execution. This makes the node boundary explicitly reject re-commit of an already committed finalized block rather than relying only on downstream consensus/block validation.

Regression coverage was added for committing a finalized block successfully and then replaying the same finalized candidate/certificate. The second commit is rejected with `ErrFinalizedBlockAlreadyCommitted` and canonical head/state remain unchanged.

This remains a local execution/commit safety boundary. It does not define cross-node replay protection, persistent finality indexing, validator-set lifecycle, or production BFT replay/evidence semantics.

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

### 4.24 ValidatorRuntime ↔ Validated Block Proposal Integration

The validated block-candidate bridge is now consumed directly by `ValidatorRuntime` through `AcceptBlockProposal`.

The runtime checks the candidate protocol version, chain ID, next height, and expected proposer before converting the validated candidate into the existing opaque consensus proposal payload. Accepted candidates move the runtime from Proposal to Prevote through the same proposal lifecycle already used by `AcceptProposal`.

The runtime does not execute or commit the candidate. Canonical state mutation remains outside consensus runtime finalization, preserving the existing separation between consensus authority and block execution/commit.

Tests cover successful validated-candidate acceptance and rejection when the candidate proposer does not match the runtime's expected proposer. The proposal-bridge documentation has been updated accordingly.

This milestone still does not define canonical serialization, P2P proposal transport, block execution during finalization, timeout/round-change behavior, or production BFT semantics.

### 4.25 Consensus Finality ↔ Block Execution/Commit Authority Boundary

A development-only finality-to-block authority boundary is now implemented under `IndoChain/internal/consensus/finality_block.go`.

`ValidateFinalizedBlock` validates the block-production context, independently validates the `FinalityCertificate`, and requires the certificate payload to equal the deterministic development hash of the exact candidate block. This prevents a certificate for one payload from being presented as authority for a different block candidate.

`FinalizedBlockCommitter` defines the narrow responsibility boundary after this authority check: consensus establishes that validator authority finalized the candidate payload, while the execution/commit owner remains responsible for executing the block against canonical state and committing block/state atomically.

Tests cover successful certificate-to-candidate binding and rejection when the candidate hash differs from the certificate payload. Detailed scope is documented in `IndoChain/docs/consensus-finality-block-commit-boundary-v0.1.md`.

A concrete `Node.CommitFinalizedBlock` experiment was deliberately removed immediately after review because the current node API still requires explicit execution public-key authority and the consensus fixture does not define a canonical validator-set/public-key mapping. No synthetic validator set or nil public key is retained in the node path. The repository therefore preserves the architecture guardrail that consensus must not invent execution authority context.

This milestone intentionally does not define canonical block/certificate serialization, P2P finality transport, production BFT semantics, timeout/round-change behavior, validator-set lifecycle, fee/gas accounting, or public-key authority resolution.

### 4.26 Consensus ↔ Execution Authority Handoff Boundary

An explicit dependency-injection boundary is now implemented under `IndoChain/internal/consensus/execution_authority.go`.

`ExecutionAuthorityResolver` makes the execution/node layer responsible for resolving a validator identity to the public key required by the existing transaction execution API. `FinalizedBlockAuthorization` binds the finalized block hash, proposer identity, and certificate payload before that handoff. `ResolveProposerAuthority` performs the lookup and returns a defensive copy without mutating consensus state or creating a protocol-level registry.

Tests cover successful resolution, missing resolver rejection, and certificate/block-payload mismatch. Detailed scope is documented in `IndoChain/docs/consensus-execution-authority-handoff-v0.1.md`.

This milestone deliberately stops at dependency injection. The current v0.1 transaction model does not contain a canonical sender public-key field, and block execution still accepts one explicit public key. Therefore no synthetic multi-sender registry or execution authority is introduced here.

### 4.28 Finalized Block → Node Commit Authority Boundary

The finalized-block authority path is now connected to the node commit boundary. `Node.CommitFinalizedBlock` requires the explicit consensus `BlockProductionContext`, candidate block, `FinalityCertificate`, `ValidatorSet`, `VotingPowerSet`, a validator authority resolver, and a separate transaction sender authority resolver.

The node first validates the candidate/certificate binding through `ValidateFinalizedBlock`, then performs the explicit proposer-validator authority handoff through `ResolveProposerAuthority`, and only then executes the block through `ImportBlockWithAuthority` using sender-level key resolution.

This deliberately keeps three identities/boundaries separate: consensus validator identity, proposer execution authority, and transaction sender authority. No canonical validator registry or validator-to-address assumption is introduced.

The commit path retains the existing atomic working-state → store commit → node-head advancement sequence. Tests cover a successful finalized-block path from certificate validation through authority resolution and canonical commit.

Detailed scope is documented in the finalized-block and execution-authority boundary documents plus the node integration implementation.
### 4.27 Consensus ↔ Node Execution Authority Integration Boundary

The explicit execution-authority handoff is now connected to the node execution path without inventing a validator registry.

`state.ExecutionRules` now supports an injected `PublicKeyResolver`, and `Node.ImportBlockWithAuthority` uses a node-owned sender authority resolver when executing transactions. The resolver path takes precedence over the legacy single `PublicKey` field and allows each transaction sender to resolve its own execution key.

The node still executes the entire candidate against a working state before committing the block/state pair, preserving the existing atomic commit boundary. A resolver failure or transaction validation failure therefore cannot advance the node head.

The implementation deliberately keeps consensus validator identity separate from transaction sender identity. No assumption is made that proposer/validator IDs are transaction addresses. Detailed scope is documented in `IndoChain/docs/node-execution-authority-integration-v0.1.md`.

Tests cover successful sender-key resolution through `Node.ImportBlockWithAuthority`. The legacy `ImportBlock(block, publicKey)` path remains available for the existing v0.1 development compatibility path.

### 4.29 Finalized Block Commit Failure-Path & Context Guard

The finalized-block node commit boundary has been tightened so consensus authorization cannot be applied against a stale or unrelated canonical node context.

Before finality validation or execution authority handoff, `Node.CommitFinalizedBlock` now validates the supplied `BlockProductionContext` and requires protocol version, chain ID, consensus height, and previous block hash to match the live node configuration/head. A mismatch returns `ErrConsensusContextMismatch` without mutating canonical state.

Failure-path coverage now explicitly checks missing authority resolvers and consensus-context mismatch, including preservation of node head, head hash, and state root. The existing atomic execution/commit path remains unchanged: candidate execution occurs against working state, store commit succeeds, then canonical node state/head advance.

Detailed scope is documented in `IndoChain/docs/node-execution-authority-integration-v0.1.md`.

### 4.30 Finalized Block Commit Failure Coverage

The finalized-block node commit boundary now has explicit failure-path coverage across the major execution/authority stages: validator authority resolution failure, finality certificate payload mismatch, transaction sender authority failure, transaction execution/signature failure, and storage commit failure.

Each rejection path asserts that canonical node head, head hash, and state root remain unchanged. The transaction execution failure test rebuilds the candidate commitment and finality certificate around an invalid transaction signature, proving that consensus finality validation can succeed while execution still rejects the block before canonical state advancement.

The store-failure path continues to exercise the node's atomic working-state → store commit → head advancement boundary. The tests therefore distinguish consensus authorization success from execution/commit success and keep failure semantics explicit.

Detailed implementation coverage is in `IndoChain/internal/node/node_test.go`. The next work remains tightening canonical consensus-to-execution context handling and then moving toward broader multi-node consensus integration.

### 4.31 Canonical Consensus-to-Execution Context Guard Tightening

The node consensus-to-execution boundary is now centralized through `validateCanonicalConsensusContext`. Finalized-block commit validates the supplied consensus context against the live node configuration and canonical head before finality or execution authority is resolved.

The guard explicitly rejects protocol-version mismatch, chain-ID mismatch, consensus height mismatch, and previous-hash mismatch. Tests cover each variant and assert that canonical node state remains unchanged on rejection.

This keeps consensus authorization tied to the exact canonical execution point: the consensus context must describe the node's current chain before a finalized candidate can cross into execution/commit. It does not yet define multi-node state synchronization or production validator authority storage.

### 4.34 Explicit Node-to-Node P2P Transport Boundary

The next multi-node integration boundary is now implemented in `IndoChain/internal/p2p/transport.go`.

A small `Transport` interface now separates node-to-node message delivery from consensus logic. The development `InMemoryTransport` provides deterministic point-to-point delivery between two node transport instances, preserves the sender identity, validates the existing P2P message envelope, and clones payloads at both send and receive boundaries.

Regression tests cover two-node routing, unknown-peer rejection without inbox mutation, and payload isolation. This is intentionally an in-process transport adapter: it does not yet provide sockets, peer discovery, connection lifecycle, backpressure, retransmission, authentication, or a canonical consensus-message codec.

The boundary is therefore ready to become the transport handoff for consensus messages without coupling the consensus engine directly to a concrete network implementation.


### 4.35 Consensus Message ↔ Explicit P2P Transport Binding

The consensus-message foundation is now bound to the explicit node-to-node transport boundary.

`IndoChain/internal/consensus/message_codec.go` now provides deterministic encode/decode for the existing consensus message fields (protocol version, chain ID, epoch, height, round, sender, message type, payload, and signature). Decoding re-validates the supplied consensus rules and rejects unsupported wire versions, malformed length fields, trailing bytes, and invalid messages.

`IndoChain/internal/p2p/consensus_transport.go` adds the `MessageTypeConsensus` transport envelope plus `SendConsensus` / `ReceiveConsensus` helpers. Consensus messages are encoded before entering the P2P transport and decoded only after the transport boundary, while the existing transport payload limit remains enforced.

Tests cover deterministic consensus codec round-trip/trailing-data rejection and end-to-end in-memory node-to-node consensus message delivery, including transport-size rejection.

This milestone intentionally does not implement network sockets, peer discovery/lifecycle, authenticated transport, retransmission/backpressure, canonical production consensus networking, or timeout/round-change behavior. The transport remains an in-process development boundary.


### 4.33 Multi-node Finalized-Commit Convergence Boundary

A first multi-node convergence test is now implemented in `IndoChain/internal/node/node_test.go`.

The test starts two independent nodes from the same Devnet genesis/state, creates and finalizes one development block through the consensus runtime, then submits the same finalized candidate and certificate to both node commit boundaries. Both nodes must converge on the same canonical head hash and state root, and both must apply the same transaction state transition.

This closes the first deterministic consensus-runtime → node → canonical-state convergence check across more than one node instance. It is still an in-process development test: it does not implement P2P transport, network message delivery, timeout/round-change behavior, persistent validator authority, or a production multi-node consensus loop.

### 4.32 Consensus Runtime → Node Finalized-Commit Handoff

The development consensus runtime now retains the finality certificate it creates at the `Finalized` phase through `FinalizedCertificate()`. The returned certificate is deep-cloned so execution/node layers cannot mutate consensus-owned evidence.

The node now exposes `CommitRuntimeFinalizedBlock`, an explicit handoff from the finalized consensus runtime into the existing canonical commit boundary. The node still owns canonical-context validation, finality/block binding, proposer authority resolution, transaction sender authority resolution, deterministic execution, and durable state/block commit.

Integration coverage exercises the complete development handoff: block proposal acceptance → quorum vote → runtime finalization → certificate retrieval → node finalized-block commit → canonical head/state advancement.

This milestone intentionally does not make the consensus runtime a production BFT loop and does not define replay protection, timeout/round-change, validator authority persistence, or multi-node transport.

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

**Snapshot date:** 2026-09-24  
**Latest verified green CI:** `f0e0be0c5ec38a157bfca658ef6ce78e83ba2622` (IndoChain CI run 702)
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
**Latest status-document update:** `e4be02e61c4a8eb06928b382bc26497cb5a90081`    
**Current branch CI after vote aggregation:** previous CI run `584` failed in `TestVoteAggregatorCalculatesPayloadPowerAndQuorum`; the test assertion has been corrected in `705a2cbb5c31c78fa43e5c8362978e4094b469de`.  
**Current branch CI after finality boundary:** no pull-request workflow run was associated with HEAD `146c2225be2dcaf787bc3f724b50d70e214dfcfc`.  
**Current branch CI after validator runtime:** IndoChain CI run `610` failed on the pull-request merge ref because `ValidatorRuntime` assigned a `VoteAggregator` value to a `*VoteAggregator` field. The runtime fix is `ac27d456622a6d8b3751832e7a73715b3c807adf`; the resulting branch HEAD later passed IndoChain CI run `622`.  
**Latest CI failure before finalized-runtime test fix:** IndoChain CI run `752` (merge ref `aa12c043bfabae2d2927e483349f87f09ef9c093`) failed during compilation in `IndoChain/internal/node/node_test.go:711` because `proposal.Payload` is a `types.Hash` array while the consensus vote message requires `[]byte`. The production runtime/node implementation was already compiling and the failure was isolated to the finalized runtime handoff test. The test now passes `proposal.Payload[:]` via commit `709215921ac191addeacf9a06549cf0dce244cb1`.\n**Verified CI after finalized-runtime test fix:** IndoChain CI run `754` completed successfully on commit `709215921ac191addeacf9a06549cf0dce244cb1` (test, go vet, and tidy checks passed).\n**CI regression found after replay/re-commit guard:** IndoChain CI run `761` failed in `TestCommitFinalizedBlockRejectsConsensusContextMismatch`. The initial replay guard ordering caused the wrong error. Fix `a610eba44e1c8321dfaecb2832ea86a33fbb560e` then exposed CI #763, where the replay test still received `ErrConsensusContextMismatch` because the stale consensus context is expected after the first commit. The attempted height-only guard fix `4dcf46e832660dc7c88e5e41688ea8f5c5fff578` still failed in CI #767 for the same ordering reason: canonical-context validation remained ahead of replay detection. The latest fix `7d471998b021b9caa91b5776be37d3aec0fe3e6f` now checks whether a positive-height candidate is exactly the already-canonical block at that height before validating the live consensus context. This preserves `ErrFinalizedBlockAlreadyCommitted` for exact replays while allowing different stale candidates to reach canonical-context validation. CI verification is pending.  

**Branch:** `dev/indochain-v0.1`  
**Stage:** Core Protocol Implementation / Pre-Consensus Integration  
**Latest block-candidate construction implementation:** `5d57d41ec1a2b901cdb63b38aa8171f07b27e754`  
**Latest block-candidate execution coverage:** `29de6a6fa35d791bbc6ae6cb3d812b421a7dd553`  
**Latest block-candidate documentation:** `239796fd04ba50378572b81a48faa07e63043f16`  
**Latest block-candidate proposal bridge:** `efdd5b3fd78ba05434a434d00b3cfbc37ccd009a`  
**Block-candidate proposal bridge documentation:** `88b139f1610921b39680301d4f24e172685a27bc`  
**Latest ValidatorRuntime block-proposal integration:** `dc648205aafffa119deea923386bdfe3e3f9575d`  
**Latest runtime integration implementation:** `885fbc210c4078534a13f22d621eabb5d4635e44`  
**Latest proposal-bridge documentation:** `b592cc9ca46aa6876daf438bfaacb77d3d206404`  
**Latest finality/block authority implementation:** `71604b7742067d0a170ad2432adecec44daaf614`  
**Latest finality/block authority tests:** `a8d090beafae65e7a13764ed26e443ac10a75a66`  
**Latest finality/block authority documentation:** `00600567f1e653350848eff959305fc4d7127fde`  
**Latest execution-authority handoff implementation:** `3466f7f3a18e286068c0391f00dc64d22d02159e`  
**Latest execution-authority handoff tests:** `26c58ed77e3b2a2dd50398fabcccb3e74ee84151`  
**Latest execution-authority handoff documentation:** `9e35299980a4dbe67d786de27fcde9361cdaa1cf`  
**Latest node execution-authority integration:** `3a37071d11f4d7e133f7c23736a3c6f81c091e31`  
**Latest node execution-authority documentation:** `3a0d109c1900e6bf74ad1e5094ce779bd8ef972d`  
**Latest finalized-block → node commit integration:** `3b7bc7e5f6cbe33bd72e84eecc1075cf792ad1e4`  \n**Latest finalized-block commit context guard:** `ec35299e4f6115f0be5e7f46b70962051cb20368`  \n**Latest finalized-block commit failure coverage:** `95c128af32005c53a303a45dc77101c9005506d9`  \
**Latest canonical consensus-context guard tightening:** `4e11821e796ea769bedbd7a48f6846712c41fffe`  \
**Latest test fix for finalized runtime handoff:** `709215921ac191addeacf9a06549cf0dce244cb1`  \
**Latest consensus-runtime → node finalized-commit handoff:** `a9c1a041f6bce7cdb29df2aff11cea4f64e9f508`  
**Latest finalized-block → node commit implementation:** `7d471998b021b9caa91b5776be37d3aec0fe3e6f`  
**Latest replay/re-commit guard fix:** `7d471998b021b9caa91b5776be37d3aec0fe3e6f`  
**Latest multi-node finalized-commit convergence test:** `c2d1de1bb23aff4017843a6cc58c05208b7f5c29`  
**Latest explicit node-to-node transport implementation:** `624a3ebf0a514dfd80afc6494c46f89f9bc0f497`  
**Latest explicit node-to-node transport tests:** `fc61f67ad93fcaf6d27155832e106ac12838b938`  
**Verified transport CI:** IndoChain CI run `788` completed successfully for `fc61f67ad93fcaf6d27155832e106ac12838b938`.  
**Latest consensus message codec implementation:** `6c41cc4e08401696cea94ad0997f4699d3b8c7da`  
**Latest consensus message codec tests:** `05f6a2e2fa6a7c45d0f5cac202324056fc7c9dc9`  
**Latest consensus ↔ P2P transport binding:** `72cff1d72d6bc98be7f44d875f865aa4966c8eb6`  
**Latest consensus transport integration tests:** `419266266ee3a84b207ad807139491b00fe31b0b`  
**Latest deterministic multi-node consensus exchange test:** `04984ecd4c6544dfbf1c6096c82d21874e6c70c1`  
**Latest multi-node ValidatorRuntime transport integration test:** `746a6431aafc087f4b07e56f42a2b7c4a27019f1`  
**Latest transport → runtime → finalized-node handoff integration test:** `b541821d4b2f9a11e5243e555c4c776518c4c1f5`  
**Verified finalized handoff integration CI:** IndoChain CI run `830` completed successfully for `b541821d4b2f9a11e5243e555c4c776518c4c1f5` (test, tidy, and vet gate passed).  
**Latest consensus transport CI status:** CI #800 failed on merge SHA `95c73f5ddb37a6252860ccd12ef827f336ad5dee` because `ValidateMessage` did not include `MessageTypeConsensus`; fixed in `334510f29671b224ba0d89c932051064ae961675`. CI #802 then failed on merge SHA `e785d4a8f5f9d1d54e8d8551e81b3d832fe7ac5e` because `MessageTypeConsensus` was declared twice; fixed in `cd69f80c5b19ff6cea071b1ea9afe7e4efce13b1`. IndoChain CI #806 completed successfully for `cd69f80c5b19ff6cea071b1ea9afe7e4efce13b1` (test, tidy, and vet gate passed). The new deterministic multi-node exchange test was added in `04984ecd4c6544dfbf1c6096c82d21874e6c70c1`; its earlier workflow was pending when first recorded. The follow-on ValidatorRuntime integration test initially failed in CI #814 because the test sent unsigned messages through the consensus codec path; the test was corrected to sign proposal/vote messages in `746a6431aafc087f4b07e56f42a2b7c4a27019f1`. IndoChain CI #818 completed successfully for `746a6431aafc087f4b07e56f42a2b7c4a27019f1` (test, tidy, and vet gate passed).  


### 4.36 Deterministic Multi-Node Consensus Message Exchange Boundary

The bound consensus transport is now exercised across two independent in-memory node transports in IndoChain/internal/p2p/consensus_multinode_test.go.

The test establishes bidirectional node-to-node connections and performs a deterministic proposal exchange from node A to node B followed by a vote exchange from node B back to node A. Each message crosses the consensus codec and P2P transport boundary, preserving protocol context, message type, payload, signature, and transport-level sender identity.

This milestone demonstrates deterministic multi-node consensus message delivery in-process. It does not implement real network sockets, peer discovery, connection lifecycle, retransmission, authentication/peer identity binding, timeout/round-change behavior, validator authority persistence, or a production consensus loop.

**Next major boundary:** Integrate the consensus transport exchange with consensus RoundState / ValidatorRuntime message processing in a deterministic multi-node integration test  

### 4.37 Consensus Transport ↔ ValidatorRuntime Multi-Node Processing Boundary

The explicit consensus transport is now connected to two independent development `ValidatorRuntime` instances in `IndoChain/internal/p2p/consensus_runtime_multinode_test.go`.

The deterministic integration test establishes the same round state, validator set, voting power, quorum threshold, and proposer selection on nodes A and B. Node A accepts the proposal locally and sends it through the consensus codec/P2P transport; node B receives the message and processes it through `ValidatorRuntime.AcceptProposal`. Node B then processes its vote locally and sends the vote back; node A receives the transported vote and processes it through `ValidatorRuntime.AddVote`. The resulting quorum advances node A to `Precommit`, after which the runtime creates a finality certificate for the shared proposal payload.

This milestone verifies the separation between transport delivery and consensus semantics across two in-process nodes. The transport carries encoded messages, while `ValidatorRuntime` remains responsible for proposal/vote validation, proposer checks, quorum progression, and development-only finalization.

This remains a deterministic in-process integration test. It does not implement signature-authority binding, real network sockets, peer authentication, retransmission, timeout/round-change, validator-set transitions, persistent consensus state, or a production BFT loop.

**Next major boundary:** Extend the multi-node runtime integration from proposal/vote exchange into finalized-block handoff, while preserving the explicit transport/runtime separation and canonical node commit guards.

### 4.39 Consensus Transport → Runtime → Finalized Node Handoff Negative-Path Matrix

The finalized handoff boundary now has deterministic negative-path coverage in `IndoChain/internal/p2p/consensus_runtime_multinode_test.go`.

The matrix covers four failure classes without weakening the existing canonical guards:

- **Mismatched proposal payload:** a block proposal whose opaque payload no longer matches its candidate is rejected by the validator runtime before phase advancement.
- **Stale canonical context:** a finalized candidate submitted with a stale previous-hash context is rejected by `Node.CommitFinalizedBlock` with `ErrConsensusContextMismatch`, and the canonical head remains unchanged.
- **Invalid finality evidence:** tampered certificate payload is rejected at the finalized-block validation boundary, and the node head remains unchanged.
- **Replayed finalized block:** after one successful finalized commit, replaying the exact canonical candidate/certificate is rejected with `ErrFinalizedBlockAlreadyCommitted`.

A shared deterministic finalized-handoff fixture keeps the negative tests aligned with the same Devnet, block-candidate, signed consensus-message, runtime-finality, validator-authority, and transaction-authority boundaries used by the positive integration path.

This remains development-only and does not introduce production BFT timing, round-change/timeout, persistent finality indexing, or cross-node replay semantics.

**Latest negative-path matrix implementation:** `066f1bff2788fb5fc719a12082efa2bf7d311c3b`  
**CI failure and root cause:** IndoChain CI run `836` / push run `835` failed for `066f1bff2788fb5fc719a12082efa2bf7d311c3b` during `go test ./...`. The negative-path test treated `BlockProposal.Payload` as a slice although it is the fixed-size `types.Hash`, and the shared fixture passed `n.Config.BlockRules(nil)` without unpacking its `(rules, error)` result. The same failing test also exposed that `ValidatorRuntime.AcceptBlockProposal` was not recomputing the candidate payload; it only compared the proposal payload to itself. These were corrected in `c5fe45b09df64d35e922e3e4de893db6aeb7a094` and `ff63ae7009370f572fac50d85f38eb53ce03c446`. CI run `840` then exposed a second fixture naming collision: the execution `block.ExecutionRules` variable and consensus `ValidationRules` used the same `rules` identifier. This was corrected in `22323dc2a5070fa47608425faef6933a97a12fa2`.

**Latest multi-height finalized handoff test:** `34e3b4e5880228cd38872d8db7d0454c3f10fc6e`. The new deterministic integration commits two consecutive finalized blocks at heights 1 and 2 through the same transport → runtime → certificate → node-commit boundary, verifying that each block uses the previous canonical head hash and that the persistent store head follows the node head after each commit.

**CI failure and root cause:** IndoChain CI run `848` / validation run `849` failed during `go test ./...` because the multi-height test passed `types.Height` directly to `consensus.NewRoundState`, whose height parameter is `uint64`. This was corrected in `717cb3acb35554eb2a8915fe63e81862d5a7ed12` with an explicit `uint64(height)` conversion. The status-doc follow-up also ran red as runs `850`/`851` because it inherited the failing implementation commit; the fix commit is now the CI gate.

**Next major boundary:** verify multi-height finalization in CI, then add a deterministic negative-path check for cross-height context/replay rejection so a height-2 handoff cannot be committed against the height-0/height-1 canonical context.

**Latest multi-height CI regression:** IndoChain CI run `853` failed in `TestInMemoryTransportRuntimeFinalizedBlockMultiHeight` at height 2 with `consensus execution context mismatch`. The test had been constructing `RoundState` with the loop height as the epoch argument and a fixed consensus height of zero. That happened to align with the genesis head for the first block, but after height 1 was committed the canonical node head became height 1 while the height-2 consensus context remained at height 0. The correction in `0dde30c6209656e1e7328ce948db61c64a8487e9` derives the consensus context height from `n.Head.Header.Height` and keeps epoch at zero, so the candidate remains the next height (`context height + 1`) while the node commit guard sees the current canonical context.

**Next CI gate:** verify `0dde30c6209656e1e7328ce948db61c64a8487e9` and its status-document follow-up workflow before proceeding to the cross-height negative-path coverage.

**Verified multi-height CI:** IndoChain CI run `857` completed successfully for `0dde30c6209656e1e7328ce948db61c64a8487e9`. The status-document follow-up run `859` also completed successfully. The multi-height finalized handoff is therefore green through the test/tidy/vet CI gate.

**Cross-height negative-path coverage:** Added `TestConsensusRuntimeNegativeCrossHeightStaleContext` in `IndoChain/internal/p2p/consensus_runtime_multinode_test.go`. The test first commits height 1, constructs and finalizes a valid height-2 candidate against the height-1 canonical head, then intentionally submits that height-2 candidate/certificate with the previous height-0 consensus state and previous hash. The node must reject the handoff with `ErrConsensusContextMismatch` and leave the height-1 canonical head unchanged. Implementation commit: `72f65ec00c3a314aad6ac88a515ea4439a30fdd5`.

**Verified cross-height stale-context CI:** IndoChain CI run `861` completed successfully for `72f65ec00c3a314aad6ac88a515ea4439a30fdd5`. The status-document follow-up run `863` also completed successfully.

**Cross-height replay candidate coverage:** Added `TestConsensusRuntimeNegativeCrossHeightReplayedCandidate`. The test commits a valid height-1 finalized block, commits a valid height-2 finalized block against the height-1 canonical hash, then replays the exact lower-height candidate/certificate from height 1 against the height-2 canonical head. The node must reject the lower-height replay with `ErrFinalizedBlockAlreadyCommitted` before stale-context validation, and the height-2 canonical head/hash must remain unchanged. Implementation commit: `54906e191f4edf36535e4a65a6ae5102d3b25656`.

**Verified cross-height replay CI:** IndoChain CI run `865` completed successfully for `54906e191f4edf36535e4a65a6ae5102d3b25656`. The status-document follow-up run `867` also completed successfully.

**Cross-height different-candidate coverage:** Added `TestConsensusRuntimeNegativeCrossHeightDifferentCandidate`. After height 2 is canonical, the test constructs a different but otherwise valid height-1 candidate against the original genesis context and finalizes its certificate. Because its hash differs from the canonical height-1 block, the replay guard must not classify it as an exact replay; canonical context validation must instead reject it with `ErrConsensusContextMismatch`, while the height-2 canonical head/hash remains unchanged. Implementation commit: `399d7311f0021575b3d221737bc79630fb451128`.

**Verified cross-height different-candidate CI:** IndoChain CI run `869` completed successfully for `399d7311f0021575b3d221737bc79630fb451128`.


**Cross-height future-candidate stale-context coverage:** Added `TestConsensusRuntimeNegativeCrossHeightFutureCandidateStalePreviousHash`. The test commits heights 1 and 2 normally, then constructs and finalizes a height-3 candidate using the height-2 consensus state but deliberately reuses the height-1 canonical hash as its previous hash. The candidate is therefore future-height relative to the committed chain but carries a stale cross-height parent context. `Node.CommitFinalizedBlock` must reject it with `ErrConsensusContextMismatch`, while the canonical height-2 head/hash remains unchanged. Implementation commit: `2f2e62ae642d85beb12c4840a14db312fe4be5b1`.

**Verified future-candidate CI:** IndoChain CI run `873` completed successfully for `2f2e62ae642d85beb12c4840a14db312fe4be5b1`.\n\n**Extended deterministic multi-height handoff:** Expanded `TestInMemoryTransportRuntimeFinalizedBlockMultiHeight` from heights 1–2 to heights 1–3. The same transport → runtime → finality certificate → canonical commit path now verifies three consecutive finalized transitions, with each next block derived from the current canonical head and the persisted store head checked after every commit. Implementation commit: `7141bd528478bcebe4af7339553611806c598754`.\n\n**Verified three-height handoff CI:** IndoChain CI run `877` completed successfully for `7141bd528478bcebe4af7339553611806c598754`. The status-document follow-up run `879` also completed successfully.\n\n**Cross-height invalid-finality coverage:** Added `TestConsensusRuntimeNegativeCrossHeightInvalidFinalityEvidence`. After committing height 1, the test constructs a valid height-2 candidate and finality certificate, tampers with the certificate payload, and verifies that the finalized handoff rejects the invalid evidence without changing the canonical height-1 head. Implementation commit: `0a3eafd02cde212f122613bb8b2c6dd4f3bde220`.\n\n**Verified cross-height invalid-finality CI:** IndoChain CI run `881` completed successfully for `0a3eafd02cde212f122613bb8b2c6dd4f3bde220` (test, tidy, and vet gate passed).\n\n**Cross-height invalid-finality assertion tightening:** Strengthened `TestConsensusRuntimeNegativeCrossHeightInvalidFinalityEvidence` so tampered certificate payloads must fail specifically with `consensus.ErrFinalityQuorumNotReached`, not merely return an unspecified error, while the canonical height-1 head remains unchanged. Implementation commit: `04f3f210765c67d0744299ae28bb9e11b6a718da`.\n\n**Next CI gate:** verify `04f3f210765c67d0744299ae28bb9e11b6a718da` and its status-document follow-up workflow before moving the finalized handoff negative matrix to the next protocol boundary.\n\n**Verified assertion-tightening CI:** IndoChain CI run `885` completed successfully for `04f3f210765c67d0744299ae28bb9e11b6a718da` (test, tidy, and vet gate passed).\n\n**Cross-height finality-certificate context coverage:** Extended `TestConsensusRuntimeNegativeCrossHeightInvalidFinalityEvidence` to mutate the certificate height across the canonical height boundary and require `consensus.ErrStateContextMismatch`. This verifies certificate context binding independently from payload/quorum tampering while preserving the canonical height-1 head. Implementation commits: `2dcd07856ed2882695056e1a4a3df68ed94fd96f`, `627257a5f20e438c85d904dbdd8414b440747d69`, `f2193e4db90d9a555231e49e54b40bcac5f28ee6`.\n\n**CI failure and root cause:** IndoChain CI run `893` (workflow run `35978181496`) failed during `go test ./...` because `IndoChain/internal/p2p/consensus_runtime_multinode_test.go` contained an accidental duplicated test-file block beginning with the malformed `ckage p2p` declaration. The failure was a compile-time parse error at line 507 (`expected declaration, found ckage`). The duplicate block was removed in `ab994f2a582fab9388e54f93da0ab2e6127599bd` (`fix(indochain): remove duplicated consensus runtime test block`).\n\n**Verified CI recovery:** IndoChain CI run `897` (workflow run `35978354118`) completed successfully for `ab994f2a582fab9388e54f93da0ab2e6127599bd` after removal of the duplicated test block.

**Next finalized-handoff boundary:** Added `TestConsensusRuntimeNegativeCrossHeightVoteContextMismatch`. After a valid height-1 commit and valid height-2 finality certificate, the test mutates one certificate vote's height to the prior canonical height. `Node.CommitFinalizedBlock` must reject the certificate with `consensus.ErrStateContextMismatch`, and the canonical height-1 head/hash must remain unchanged. Initial implementation commit: `2ea0d463a9101565543a4bd07038cddd2e47d64e2`. The canonical-head assertion was tightened in `025eeb255ede6615c248c3620ff56bfd3acaeae8`.

**CI failure and root cause:** IndoChain CI #903 (workflow run `35978524876`) failed in `TestConsensusRuntimeNegativeCrossHeightVoteContextMismatch`. The test expected `ErrStateContextMismatch`, but the mutated certificate vote is validated first by the consensus message/context boundary and correctly returns `ErrConsensusMessageContextMismatch` wrapped by finality validation. The implementation was corrected in `ea3040ccdf7fc830d92a934e7446efa82fd05361` to assert the actual message-context error without weakening the node-level canonical-context guard.\n\n**Next finalized-certificate context boundary:** Extended `TestConsensusRuntimeNegativeCrossHeightVoteContextMismatch` to restore the vote context and independently mutate the certificate round. The finalized handoff must reject this certificate-level context mismatch with `consensus.ErrStateContextMismatch`, while the canonical height-1 head/hash remains unchanged. Implementation commit: `1421864c953281d5b660cc6f19d47205a67bd219`.\n\n**Verified CI:** IndoChain CI #907 (workflow run `35978662192`) completed successfully for `ea3040ccdf7fc830d92a934e7446efa82fd05361`.\n\n**Verified certificate-round CI:** IndoChain CI #911 (workflow run `35978859501`) completed successfully for `1421864c953281d5b660cc6f19d47205a67bd219`.\n\n**Next finalized-certificate context boundary:** Extended the finalized handoff negative matrix to mutate the certificate epoch independently after restoring the vote and round contexts. The handoff must reject the certificate with `consensus.ErrStateContextMismatch`, while the canonical height-1 head/hash remains unchanged. Implementation commit: `c950ec3a49e8d61a730fccfed8a28ed759811c42`. Next CI gate is `c950ec3a49e8d61a730fccfed8a28ed759811c42`.

**Verified certificate-epoch CI:** IndoChain CI #915 (workflow run `35978959607`) completed successfully for `c950ec3a49e8d61a730fccfed8a28ed759811c42`.

**Next finalized-certificate context boundary:** Extended `TestConsensusRuntimeNegativeCrossHeightVoteContextMismatch` to independently mutate the certificate protocol version and chain ID after restoring the epoch/round context. Each mutation must fail with `consensus.ErrStateContextMismatch`, while the canonical height-1 head/hash remains unchanged. Implementation commit: `a16c1ff9608e10d7e1958a7a1ec05b118d5ce9ec`.

**Verified certificate protocol/chain-context CI:** IndoChain CI #919 (workflow run `35979166443`) completed successfully for `a16c1ff9608e10d7e1958a7a1ec05b118d5ce9ec`.

**Next finalized-certificate context boundary:** Completed the certificate context matrix by independently mutating certificate height after restoring protocol version and chain ID. The mutation must fail with `consensus.ErrStateContextMismatch`, while the canonical height-1 head/hash remains unchanged. Implementation commit: `3d5b34200dd196c9a6d146da76daf51bca3ab8ec`.

**CI failure and root cause:** IndoChain CI #923 (workflow run `35979425254`) failed during `go test ./...` for `3d5b34200dd196c9a6d146da76daf51bca3ab8ec`. The newly added certificate-height assertion block was accidentally placed outside `TestConsensusRuntimeNegativeCrossHeightVoteContextMismatch`, leaving `certificate2` at package scope and causing the Go parser error `expected declaration, found certificate2` at line 715. The correction in `7b2ad926d638c1c7f2bee66842bab601fad90b1d` moves the height-mismatch block back inside the test function without changing the intended assertion.

**Next CI gate:** verify `7b2ad926d638c1c7f2bee66842bab601fad90b1d` through the full IndoChain test/tidy/vet workflow before proceeding to the next finalized-handoff negative boundary.

**Verified CI recovery:** IndoChain CI #927 (workflow run `35979759960`) completed successfully for `7b2ad926d638c1c7f2bee66842bab601fad90b1d`; test, tidy, and vet all passed.

**Next finalized-handoff protocol boundary:** Extended `TestConsensusRuntimeNegativeCrossHeightVoteContextMismatch` to restore the completed certificate context and independently mutate the certificate quorum threshold to an invalid `2/1` ratio. `Node.CommitFinalizedBlock` must reject the certificate with `consensus.ErrInvalidQuorumThreshold`, and the canonical height-1 head/hash must remain unchanged. Implementation commit: `a3ec645018431cf444bfafb1f0e519edaf04479d`.

**Verified invalid-threshold CI:** IndoChain CI #931 (workflow run `35980062835`) completed successfully for `a3ec645018431cf444bfafb1f0e519edaf04479d`; test, tidy, and vet all passed.

**Next finalized-handoff certificate-integrity boundary:** Added `TestConsensusRuntimeNegativeInvalidFinalityCertificateStructure`. The test independently clears the certificate payload and then the certificate vote set, requiring `consensus.ErrInvalidFinalityCertificate` in both cases while preserving the canonical genesis head/hash. Implementation commit: `2875efe14d56e9031389c11c1a8999a65764c5da`.

**Verified certificate-structure CI:** IndoChain CI #937 (workflow run `35980379291`) completed successfully for `2875efe14d56e9031389c11c1a8999a65764c5da`; test, tidy, and vet all passed.

**Next finalized-handoff quorum boundary:** Added `TestConsensusRuntimeNegativeFinalityQuorumNotReached`. The test uses a structurally valid certificate but raises its threshold to `2/3` while the fixture contains only one unit of voting power, requiring `consensus.ErrFinalityQuorumNotReached` and preserving the canonical genesis head/hash. Implementation commits: `4812dfa3b7812e3253cb5fd893088c99d5a769d1`, refined to isolate this boundary in `ec7d81c03d545eb034af9db87ecfb52f928e7893`.

**CI failure and root cause:** IndoChain CI #943 (workflow run `35980739983`) failed in `TestConsensusRuntimeNegativeFinalityQuorumNotReached`: the test changed a one-validator certificate threshold to `2/3`, but that validator held 100% of the supplied voting power, so the quorum was correctly reached and the test received nil. The correction in `03aaedf70745e48f1dcd9e80ae3f86deffc76213` keeps the certificate's single vote at 1/2 of total voting power by adding a second validator with equal power, making the `2/3` threshold genuinely unreachable while preserving the same finalized candidate/context.\n\n**CI failure and root cause:** CI #947 (workflow run `35980908012`) failed at compile time in `internal/p2p/consensus_runtime_multinode_test.go:761` and `:765`: the newly added quorum fixture used `validators, err =` and `power, err =` without a local `err` declaration in that test. The first correction in `f554f6f0995c23d8b2fed4f991f470089769083d` declared both locally, but this exposed the opposite scope issue: `power, err :=` introduced no new variable because `power` and `err` were already in scope. CI #951 (workflow run `35981140261`) therefore failed at line 765 with `no new variables on left side of :=`. The correction in `e7c955a9a3e4a57562b784f767f1efc991c5feab` keeps `validators, err :=` for the first declaration and changes the subsequent voting-power assignment to `power, err =`, reusing the existing variables.\n\n**Next CI gate:** verify `e7c955a9a3e4a57562b784f767f1efc991c5feab` through the full IndoChain test/tidy/vet workflow before proceeding to the next finalized-handoff negative boundary.

### 4.38 Consensus Transport → Runtime → Finalized Node Handoff Boundary

The multi-node consensus integration now crosses the full development handoff from explicit P2P transport into `ValidatorRuntime` finalization and then into the canonical node commit boundary.

`IndoChain/internal/p2p/consensus_runtime_multinode_test.go` now constructs a deterministic block candidate against a Devnet node context, transports the proposal from node A to node B, processes the proposal and vote through the validator runtime, transports the vote back to node A, creates the finality certificate, and commits the finalized candidate through `Node.CommitFinalizedBlock`.

The test verifies that the canonical node head and persisted store head advance to the finalized block after the consensus runtime reaches `Finalized`. The execution/node layer continues to own canonical context validation, proposer-authority resolution, block execution, and durable commit; the consensus runtime remains responsible for proposal/vote progression and finality evidence.

The integration remains development-only and in-process. It does not implement production BFT timing, round-change/timeout, peer authentication, persistent consensus state, validator-set transitions, canonical certificate serialization, or real network sockets.

**Next major boundary:** add a deterministic negative-path matrix across the transport → runtime → finalized-node handoff, covering stale consensus context, mismatched proposal payload, invalid finality evidence, and replayed finalized-block commit without weakening the existing canonical guards.

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

**CI failure and root cause:** IndoChain CI #955 (workflow run `35981322831`) for `e7c955a9a3e4a57562b784f767f1efc991c5feab` failed during compilation of `internal/p2p/consensus_runtime_multinode_test.go`. The log exposed two scope issues: the finalized-handoff test used `power` without declaring it locally, producing `undefined: power` at lines 215, 226, 234, and 344; the quorum-negative test still used `power, err :=` after `power` was already a function parameter and `err` had been declared, producing `no new variables on left side of :=` at line 765. The correction in `374118ffd82511101f563f9a31db065d82fa7c0a` declares the handoff fixture's voting-power variable locally and reuses the existing `power, err` variables in the quorum-negative test.

**Next CI gate:** verify `374118ffd82511101f563f9a31db065d82fa7c0a` through the full IndoChain test/tidy/vet workflow before proceeding to the next finalized-handoff negative boundary.

**Verified CI recovery:** IndoChain CI #959 (workflow run `35981696453`) completed successfully for `374118ffd82511101f563f9a31db065d82fa7c0a`; the full test/tidy/vet gate passed. The finalized-handoff quorum fixture scope issue is therefore resolved.

**Next finalized-handoff negative boundary:** Added `TestConsensusRuntimeNegativeDuplicateFinalityVote`. The test keeps the certificate payload/context valid but duplicates the same validator vote, requiring `consensus.ErrDuplicateVote` and preserving the canonical genesis head/hash. Implementation commit: `e5283b45951305a5460fcc472fa56d36406a693a`.

**Next CI gate:** verify `e5283b45951305a5460fcc472fa56d36406a693a` through the full IndoChain test/tidy/vet workflow before proceeding to the next finalized-handoff negative boundary.

**Verified duplicate-finality-vote CI:** IndoChain CI #963 (workflow run `35981869554`) completed successfully for `e5283b45951305a5460fcc472fa56d36406a693a`; the full test/tidy/vet gate passed. The finalized handoff now explicitly rejects duplicate validator vote evidence with `consensus.ErrDuplicateVote` while preserving the canonical genesis head/hash.

**Next finalized-handoff voting-power boundary:** Added `TestConsensusRuntimeNegativeMissingVotingPower`. The test keeps the validator membership and finality certificate structurally valid but supplies an empty voting-power set, so the certificate vote sender has no voting power. The finalized handoff must reject it with `consensus.ErrVoteSenderNotInVotingPower` and leave the canonical genesis head/hash unchanged. Implementation commit: `0d2ccf56dded8db5a12dfb5be55dd39cca3c68d9`.

**Next CI gate:** verify `0d2ccf56dded8db5a12dfb5be55dd39cca3c68d9` through the full IndoChain test/tidy/vet workflow before proceeding to the next finalized-handoff negative boundary.

**Verified missing-voting-power CI:** IndoChain CI #967 (workflow run `35982129267`) completed successfully for `0d2ccf56dded8db5a12dfb5be55dd39cca3c68d9`; the full test/tidy/vet gate passed. The finalized handoff now explicitly rejects finality evidence whose vote sender has no supplied voting power with `consensus.ErrVoteSenderNotInVotingPower`.

**Next finalized-handoff validator-membership boundary:** Added `TestConsensusRuntimeNegativeMissingValidatorMembership`. The test keeps the certificate and voting-power evidence intact but supplies an empty validator membership set, requiring `consensus.ErrValidatorNotFound` and preserving the canonical genesis head/hash. Implementation commit: `8ce3e1f4c754505f0e1acfe2625ac65d312b447e`.

**Next CI gate:** verify `8ce3e1f4c754505f0e1acfe2625ac65d312b447e` through the full IndoChain test/tidy/vet workflow before proceeding to the next finalized-handoff negative boundary.


**CI failure and root cause:** IndoChain CI #971 (workflow run `35982909334`) failed in `TestConsensusRuntimeNegativeMissingValidatorMembership`. The test expected `consensus.ErrValidatorNotFound`, but the existing consensus-message validation pipeline rejects the vote sender earlier with the active-validator membership error `sender not found`, wrapped by the finality validation path. The protocol boundary is therefore already rejecting the missing membership correctly; the test assertion was too specific to the lower-level validator-set sentinel.

**Correction:** Updated `TestConsensusRuntimeNegativeMissingValidatorMembership` to assert the actual sender-membership rejection message while retaining the canonical-head immutability check. Implementation commit: `6c8f1ec6ab61deecd6e94c288376ab56f83301f8`.

**Next CI gate:** verify `6c8f1ec6ab61deecd6e94c288376ab56f83301f8` through the full IndoChain test/tidy/vet workflow before proceeding to the next finalized-handoff negative boundary.


**Verified membership-fix CI:** IndoChain CI #975 (workflow run `35983278368`) completed successfully for `6c8f1ec6ab61deecd6e94c288376ab56f83301f8`; the full test/tidy/vet gate passed.

**Next finalized-handoff voting-power integrity boundary:** Added `TestConsensusRuntimeNegativeInvalidVotingPowerSet`. The test supplies a directly constructed voting-power set containing zero power so the finalized handoff exercises `VotingPowerSet.Validate` rather than failing during fixture construction. The handoff must reject the invalid set with `consensus.ErrInvalidVotingPowerSet` and preserve the canonical genesis head/hash. Initial implementation commit: `4b3260775b4277757a0c6e602f2966e0bebdc4a9`.

**Correction:** The test initially used the wrong struct field name for `VotingPowerSet`; corrected to `Validators` in `ff0ea4d59e971f60a3e31e8e51e5eb58d4896bf3` before the CI gate.

**Next CI gate:** verify `ff0ea4d59e971f60a3e31e8e51e5eb58d4896bf3` through the full IndoChain test/tidy/vet workflow before proceeding to the next finalized-handoff negative boundary.

**Verified invalid-voting-power-set CI:** IndoChain CI #981 (workflow run `35984088304`) completed successfully for `ff0ea4d59e971f60a3e31e8e51e5eb58d4896bf3`; the full test/tidy/vet gate passed. The finalized handoff now explicitly rejects a directly constructed zero-power entry with `consensus.ErrInvalidVotingPowerSet` while preserving the canonical genesis head/hash.

**Next finalized-handoff voting-power integrity boundary:** Added `TestConsensusRuntimeNegativeDuplicateVotingPowerValidator`. The test supplies a directly constructed voting-power set containing the same validator identifier twice, requiring `consensus.ErrInvalidVotingPowerSet` and preserving the canonical genesis head/hash. Implementation commit: `b7856625597828fe5518a9839e7861ee04853b21`.

**Next CI gate:** verify `b7856625597828fe5518a9839e7861ee04853b21` through the full IndoChain test/tidy/vet workflow before proceeding to the next finalized-handoff negative boundary.

**Verified duplicate-voting-power CI:** IndoChain CI #985 (workflow run `35984429456`) completed successfully for `b7856625597828fe5518a9839e7861ee04853b21`; the full test/tidy/vet gate passed. The finalized handoff now explicitly rejects duplicate validator identifiers in the supplied voting-power set with `consensus.ErrInvalidVotingPowerSet` while preserving the canonical genesis head/hash.

**Next finalized-handoff voting-power integrity boundary:** Added `TestConsensusRuntimeNegativeUnsortedVotingPowerSet`. The test supplies a directly constructed voting-power set whose validator identifiers are not in canonical byte-sorted order, requiring `consensus.ErrInvalidVotingPowerSet` and preserving the canonical genesis head/hash. Implementation commit: `2143ac2a599a932c0124c0be34d2e92b57b6b7fc`.

**Next CI gate:** verify `2143ac2a599a932c0124c0be34d2e92b57b6b7fc` through the full IndoChain test/tidy/vet workflow before proceeding to the next finalized-handoff negative boundary.


**Verified unsorted-voting-power CI:** IndoChain CI #989 (workflow run `35985380325`) completed successfully for `2143ac2a599a932c0124c0be34d2e92b57b6b7fc`; the full test/tidy/vet gate passed. The finalized handoff now explicitly rejects a non-canonical validator-ID ordering in the supplied voting-power set with `consensus.ErrInvalidVotingPowerSet` while preserving the canonical genesis head/hash.

**Next finalized-handoff voting-power integrity boundary:** Added `TestConsensusRuntimeNegativeEmptyValidatorIDVotingPower`. The test supplies a directly constructed voting-power set containing an empty validator identifier with positive power, requiring `consensus.ErrInvalidVotingPowerSet` and preserving the canonical genesis head/hash. Implementation commit: `977053ff849cde6ecce0337108c61ab3e2720465`.

**Next CI gate:** verify `977053ff849cde6ecce0337108c61ab3e2720465` through the full IndoChain test/tidy/vet workflow before proceeding to the next finalized-handoff negative boundary.

**Verified empty-validator-ID CI:** IndoChain CI #993 (workflow run `35985556028`) completed successfully for `977053ff849cde6ecce0337108c61ab3e2720465`; the full test/tidy/vet gate passed. The finalized handoff now explicitly rejects a voting-power entry with an empty validator identifier using `consensus.ErrInvalidVotingPowerSet` while preserving the canonical genesis head/hash.

**Next finalized-handoff voting-power integrity boundary:** Added `TestConsensusRuntimeNegativeVotingPowerTotalOverflow`. The test supplies two distinct valid-looking voting-power entries whose uint64 powers overflow during total-power calculation, requiring `consensus.ErrInvalidVotingPowerSet` and preserving the canonical genesis head/hash. Implementation commit: `d5153827889828eafa3779c9e7cbc00cf638ac0d`.

**Next CI gate:** verify `d5153827889828eafa3779c9e7cbc00cf638ac0d` through the full IndoChain test/tidy/vet workflow before proceeding to the next finalized-handoff negative boundary.


**Next finalized-handoff proposer-authority boundary:** Added `TestConsensusRuntimeNegativeMissingValidatorAuthority`. The test keeps the finalized certificate, validator membership, and voting power valid, but supplies a validator authority resolver that returns no public key. `Node.CommitFinalizedBlock` must reject the explicit consensus-to-execution authority handoff with `consensus.ErrExecutionAuthorityMissing` and preserve the canonical genesis head/hash. Implementation commit: `b784de94227ebd98af8b0b9898f8f0482c049a25`.

**Next CI gate:** verify `b784de94227ebd98af8b0b9898f8f0482c049a25` through the full IndoChain test/tidy/vet workflow before proceeding to the next finalized-handoff negative boundary.


**Verified proposer-authority continuation:** CI #1000 (workflow run `35986087538`) completed successfully for `b784de94227ebd98af8b0b9898f8f0482c049a25`; the missing-validator-authority boundary passed the full test/tidy/vet gate. CI #1002 (workflow run `35986266327`) also completed successfully for status-doc commit `ebd479c11626497cf8e51501fc589c12d8f1d654`.

**Next proposer-authority boundary:** Added `TestConsensusRuntimeNegativeValidatorAuthorityResolutionError`. The test keeps finalized evidence valid but makes the injected validator-authority resolver return an explicit lookup error. `Node.CommitFinalizedBlock` must propagate that resolver error and preserve the canonical genesis head/hash. Implementation commit: `6b5575856ea23bf3ae8a29471c228886ca6ec450`.

**Next CI gate:** verify `6b5575856ea23bf3ae8a29471c228886ca6ec450` through the full IndoChain test/tidy/vet workflow before proceeding to the next finalized-handoff authority boundary.


**Verified resolver-error CI:** IndoChain CI #1008 (workflow run `35986643826`) completed successfully for `6b5575856ea23bf3ae8a29471c228886ca6ec450`; the full test/tidy/vet gate passed. The finalized handoff now explicitly propagates validator-authority resolver failures without changing canonical state.

**Next execution-authority validation boundary:** Added `TestResolveProposerAuthorityRejectsInvalidAuthorization` in `IndoChain/internal/consensus/execution_authority_test.go`. The test covers missing block hash, missing proposer, and missing certificate payload, requiring `consensus.ErrInvalidExecutionAuthority` before resolver lookup. Implementation commit: `d8c2f4396d22606acd36666f2e5d7eb272faa0f0`.

**Next CI gate:** verify `d8c2f4396d22606acd36666f2e5d7eb272faa0f0` through the full IndoChain test/tidy/vet workflow before proceeding to the next execution-authority boundary.


**Verified execution-authority validation CI:** IndoChain CI #1012 (workflow run `35986828442`) completed successfully for `d8c2f4396d22606acd36666f2e5d7eb272faa0f0`; the full test/tidy/vet gate passed.

**Next execution-authority precedence boundary:** Added `TestResolveProposerAuthorityValidatesBeforeResolverLookup`. The test supplies invalid finalized authorization together with a resolver that would return an explicit error, requiring `consensus.ErrInvalidExecutionAuthority` first. This locks the validation-before-external-resolution ordering without invoking the resolver for malformed authorization. Implementation commit: `bd691c3b96d2ac06df4c3f5667b67a3657aa94a5`.

**Next CI gate:** verify `bd691c3b96d2ac06df4c3f5667b67a3657aa94a5` through the full IndoChain test/tidy/vet workflow before proceeding to the next execution-authority boundary.

**Verified execution-authority precedence CI:** IndoChain CI #1016 (workflow run `35986968222`) completed successfully for `bd691c3b96d2ac06df4c3f5667b67a3657aa94a5`; the full test/tidy/vet gate passed. Validation-before-external-resolution ordering is now verified.

**Next execution-authority resolution boundary:** Added `TestResolveProposerAuthorityRejectsEmptyResolvedPublicKey`. The test supplies valid finalized authorization but a resolver that returns an empty public key, requiring `consensus.ErrExecutionAuthorityMissing`. Implementation commit: `49f045f2c6727c9fd603cc24358f8d29237dd51d`.

**Next CI gate:** verify `49f045f2c6727c9fd603cc24358f8d29237dd51d` through the full IndoChain test/tidy/vet workflow before proceeding to the next execution-authority boundary.

**Verified empty-resolved-authority CI:** IndoChain CI #1020 (workflow run `35988090467`) completed successfully for `49f045f2c6727c9fd603cc24358f8d29237dd51d`; the full test/tidy/vet gate passed. Empty resolved proposer authority is now explicitly covered.

**Next execution-authority isolation boundary:** Added `TestResolveProposerAuthorityClonesResolverPublicKey`. The test uses a resolver that returns its backing public-key slice directly, then mutates the resolved result and requires the resolver-owned key to remain unchanged. Implementation commit: `f796757e8f6b70247c09aef859a7f28c097b8002`.

**Next CI gate:** verify `f796757e8f6b70247c09aef859a7f28c097b8002` through the full IndoChain test/tidy/vet workflow before proceeding to the next execution-authority boundary.


**Next execution-authority isolation boundary:** Hardened `ResolveProposerAuthority` so the resolver receives a defensive copy of `FinalizedBlockAuthorization.Proposer`, preventing a resolver from mutating caller-owned proposer identity through the shared byte slice. Added `TestResolveProposerAuthorityClonesProposerForResolver`, using a resolver that deliberately mutates its input and verifying the caller's proposer remains unchanged. Implementation commit: `6238744593430fa4fd5789c604ace590782197c6`; regression-test commit: `fb10d92d6c6c835eac59a5c44b98b3d9c971f3c3`.

**Next CI gate:** verify `fb10d92d6c6c835eac59a5c44b98b3d9c971f3c3` through the full IndoChain test/tidy/vet workflow before proceeding to the next execution-authority boundary.


**Verified proposer-authority input-isolation CI:** IndoChain CI #1029 (workflow run `35991014226`) completed successfully for `fb10d92d6c6c835eac59a5c44b98b3d9c971f3c3`; the full test/tidy/vet gate passed. The execution-authority resolver now has verified isolation on both directions of the byte-slice handoff: resolver-owned public-key output cannot mutate through the returned slice, and resolver input cannot mutate caller-owned proposer identity.

**Next execution-to-transaction authority boundary:** review and extend the finalized handoff around the separate `TransactionAuthorityResolver` path. The current architecture intentionally keeps validator/proposer authority distinct from transaction sender authority; the next work should add negative coverage for sender-authority resolution/propagation while preserving canonical-state immutability.


**Next execution-to-transaction authority boundary:** Hardened `state.ApplyTransaction` so `PublicKeyResolver.PublicKeyForSender` receives a defensive copy of `tx.Sender`. Added `TestApplyTransactionClonesSenderForAuthorityResolver`, using a resolver that deliberately mutates its input; transaction execution must still verify the original signature, preserve the transaction sender, and commit the expected transfer state. Implementation commit: `83da6a6b07f21d5d5f336e3e40a81a6c69cd4085`; regression-test commit: `5f48ff1ccb2a59203c37c5821caca3fed40d32db`.

**Next CI gate:** verify `5f48ff1ccb2a59203c37c5821caca3fed40d32db` through the full IndoChain test/tidy/vet workflow before proceeding to the next transaction-authority boundary.


**Verified sender-authority input-isolation CI:** IndoChain CI #1036 (workflow run `35991830391`) completed successfully for `5f48ff1ccb2a59203c37c5821caca3fed40d32db`; the full test/tidy/vet gate passed. The transaction execution boundary now has verified isolation for resolver input: a sender-authority resolver cannot mutate the caller-owned transaction sender while signature verification and state transition continue against the original identity.

**Next transaction-authority resolution boundary:** Added `TestApplyTransactionPropagatesSenderAuthorityResolverError` in `IndoChain/internal/core/state/transition_test.go`. The test injects a sender-authority resolver that returns an explicit lookup error and requires `ApplyTransaction` to propagate that error without mutating canonical sender state or creating the recipient account. Implementation/test commit: `0a7eff7d64196576bafc42eba67f1c8eac81c283`.

**Next CI gate:** verify `0a7eff7d64196576bafc42eba67f1c8eac81c283` through the full IndoChain test/tidy/vet workflow before proceeding to the next transaction-authority boundary.


**Next transaction-authority resolution boundary:** Added `TestApplyTransactionRejectsEmptyResolvedSenderAuthority`. The injected sender-authority resolver returns an empty public key; transaction validation must reject it with `transaction.ErrInvalidPublicKey` and preserve the canonical sender state without creating the recipient. Implementation/test commit: `d5ad56d18e6779b7fffda6831c503cefe35a3f81`.

**Verified empty-resolved-authority CI:** IndoChain CI #1044 (workflow run `35992475424`) completed successfully for `d5ad56d18e6779b7fffda6831c503cefe35a3f81`; the full test/tidy/vet gate passed. The transaction execution boundary now explicitly rejects an empty resolved sender public key with `transaction.ErrInvalidPublicKey` while preserving canonical state.

**Next transaction-authority precedence boundary:** Hardened `state.ApplyTransaction` so structural transaction validation runs before injected sender-authority resolution. Added `TestApplyTransactionValidatesBeforeSenderAuthorityResolver`, which supplies an invalid transaction version together with a resolver error and requires `transaction.ErrInvalidVersion`, proving malformed transactions are rejected before external authority lookup and canonical state remains unchanged. Implementation commit: `8cf77d9c7cd8a46de63df563b34c6353e225b627`; regression-test commit: `16c2144b43fdb7e11663483bd2764cd71f148597`.

**Next CI gate:** verify `16c2144b43fdb7e11663483bd2764cd71f148597` through the full IndoChain test/tidy/vet workflow before proceeding to the next transaction-authority boundary.


**Verified transaction-authority validation precedence:** IndoChain CI #1050 (workflow run `35994245603`) completed successfully for `16c2144b43fdb7e11663483bd2764cd71f148597`; the full test/tidy/vet gate passed. Structural transaction validation is now verified to precede sender-authority resolver lookup.

**Next consensus runtime locking boundary:** Hardened `ValidatorRuntime` with an explicit locked proposal once the configured quorum is reached. After locking, a vote carrying a different payload is rejected with `consensus.ErrConflictingLockedProposal` before it can enter the vote aggregator; finalization also requires the locked payload to remain identical to the active proposal. Added `TestValidatorRuntimeRejectsVoteConflictingWithLockedProposal`, including the invariant that the conflicting vote is not recorded and the runtime remains in `PhasePrecommit`. Implementation commit: `59867d76e1b04aa43637d9b039bb01c6db91803f`; regression-test commit: `9bf20d37ca6db46b99f44325dedd7eecf96dead0`.

**Limitation:** This is a development locking invariant, not production BFT locking. The existing v0.1 runtime still models votes through the current `MessageTypeVote` / `VoteAggregator` boundary and does not yet implement distinct prevote/precommit message types, timeout/round-change, or lock carry-over across rounds.

**Next CI gate:** verify `9bf20d37ca6db46b99f44325dedd7eecf96dead0` through the full IndoChain test/tidy/vet workflow before proceeding to timeout/round-change or the next consensus lifecycle boundary.

### 4.23 Consensus Timeout / Round-Change Runtime Boundary

A deterministic development round-change boundary is now implemented in `IndoChain/internal/consensus/runtime.go`.

`ValidatorRuntime.AdvanceRound(next)` models a timeout/round-change transition by requiring a strictly newer round, resetting the phase to `PhaseProposal`, clearing the current proposal, resetting round-local vote aggregation, and preserving the previously locked proposal. The state transition and replacement vote aggregator are prepared before runtime mutation so a failed transition leaves the runtime unchanged.

A validator that already holds a lock may accept the same locked proposal in the next round, while a conflicting proposal is rejected with `ErrConflictingLockedProposal`. Round changes after finalization are rejected, and rejected conflicting proposals do not mutate round-local state.

Vote validation is performed before the lock-conflict check so malformed or unauthorized messages cannot use the lock boundary to bypass the existing consensus-message/validator validation order.

Regression tests cover:
- successful round advancement with lock preservation;
- reset of proposal and round-local votes;
- acceptance of the locked proposal in the next round;
- rejection of a conflicting proposal after round change;
- rejection of round change after finalization;
- non-mutating behavior on rejected transitions.

Commits:
- round-change runtime boundary: `d8d13ce5000990399e47de5767f1c9ada1f9888e`
- round-change regression tests: `ebadefc03688b55658ad08930274f24d27dd3cb4`
- vote validation precedence hardening: `20c51dc4b48743fcbcc7f692dd5f0cdf78092eac`

CI gate verified green: IndoChain CI #1063 (run `36062531531`) completed successfully for implementation commit `20c51dc4b48743fcbcc7f692dd5f0cdf78092eac`.

This remains a development-only timeout/round-change invariant. It does not yet define timeout certificates, proposer timeout messages, prevote/precommit wire separation, lock carry-over evidence, multi-node round synchronization, or the production BFT algorithm.


Formatting follow-up: runtime field alignment was normalized to gofmt-style formatting in commit `21d4be9f07e2b41ca0987f90688e0e828edee171`. The latest completed IndoChain CI remains #1063 (run `36062531531`) with success on implementation commit `20c51dc4b48743fcbcc7f692dd5f0cdf78092eac`.

### 4.24 Consensus Timeout Evidence Boundary

A deterministic development timeout-evidence boundary is now implemented in `IndoChain/internal/consensus/timeout.go`.

`TimeoutCertificate` binds protocol version, chain, epoch, height, current round, target next round, caller-supplied quorum threshold, and a canonical set of validator identifiers. Construction requires a strictly newer target round, unique validators with voting power, and quorum. Validator identifiers are cloned and canonically byte-sorted.

`ValidateTimeoutCertificate` independently verifies context, target round, threshold, validator uniqueness/membership, checked voting-power aggregation, quorum, and canonical validator ordering without mutating the supplied state or certificate.

This is intentionally a development evidence boundary. It does not yet define signed timeout messages, timeout signature aggregation, lock-carrying evidence, proposer synchronization, or a production BFT timeout algorithm.

Regression tests cover quorum success, canonical ordering, insufficient quorum, stale target round, duplicate validators, and non-canonical certificate rejection.

Commits:
- timeout certificate implementation: `7664ba77a1e1f941cb699e55e58432aaec5165ef`
- canonical ordering fix: `1652ecff8c3d721bc2c1c46ca4af30ff537cc853`
- timeout regression tests: `1c969c2576f9f8e00a2c8146d74d94f8cb69c000`

CI gate verified green: IndoChain CI #1078 (run `36065754434`) completed successfully for timeout regression-test commit `1c969c2576f9f8e00a2c8146d74d94f8cb69c000`.

### 4.25 Consensus Signed Timeout Message Boundary

A signed timeout-message boundary is now implemented in `IndoChain/internal/consensus/timeout_message.go`.

`MessageTypeTimeout` is added to the consensus message model. A timeout message binds the current protocol/chain/epoch/height/round context to a canonical 8-byte big-endian target round and is signed through the existing INDOCHAIN-CONSENSUS signing domain.

`ValidateTimeoutMessage` composes structural message validation, exact round-state context validation, validator membership, strictly newer target-round validation, external validator public-key resolution, and Ed25519 signature verification. Resolver input and returned public-key material are defensively copied at the authority boundary.

`NewTimeoutCertificateFromMessages` authenticates each timeout message, requires every message to target the same newer round, clones sender identifiers, and delegates quorum/canonical-order enforcement to the existing `TimeoutCertificate` boundary. Failed validation does not advance `RoundState` or mutate the supplied message set.

Regression tests cover successful signed timeout evidence, tampered payload/signature rejection, mismatched target rounds, and stale target-round rejection.

Commits:
- timeout message type: `c03674a8531db480ca3177f85000a102986fe47a`
- signed timeout boundary: `bc0003f57f3919ac3c5a8a17a92da703bef9b6a3`
- signed timeout regression tests: `eb3e4b9a3df23aff8ec68878a01712394c9b356e`

CI gate verified green: IndoChain CI #1099 (run `36067928984`) completed successfully for documentation commit `93f195bc4b52760440f92a60a0ab5f4a94917f78`.

### 4.26 Signed Timeout Evidence → Runtime Round Advance

The timeout boundary is now integrated with ValidatorRuntime through AdvanceRoundWithTimeoutEvidence.

The runtime first authenticates the supplied signed timeout messages, requires a common strictly newer target round, validates validator membership/voting power/quorum, independently validates the resulting timeout certificate, and only then invokes the existing atomic AdvanceRound(next) transition.

Rejected or insufficient timeout evidence leaves runtime state unchanged. Successful timeout advancement resets the phase to Proposal and clears round-local proposal/certificate state while retaining the existing lock semantics handled by AdvanceRound.

The timeout certificate validator boundary was also tightened so certificate validators must belong to the supplied ValidatorSet, not merely appear in the voting-power set.

Regression tests cover successful signed-evidence round advancement, insufficient quorum without mutation, tampered signature evidence without mutation, and validator-membership rejection.

Commits:
- runtime timeout integration: `0a3a1b55f43f3c5ed53e0ccb1a6b26bc7887f867`
- runtime integration tests: `2f935874d3a237fa7acd81a758485737ce728c95`
- timeout validator-membership hardening: `46ae55d052a1381a01d13d100ceb7697d4ff2fd1`
- membership regression test: `bc9555c1efa4bc4f7f3bb49dff3ea3fb5afb5c82`

CI #1109 (run `36263385593`) first failed at compile time due to an incorrect validation return type. CI #1113 (run `36263420613`) then exposed missing membership enforcement during certificate construction. Both root causes are fixed; latest fix is `84995bf3ed1e86e33b883d861f8f5a98d4dcea40`. A fresh CI run must be green before this milestone is considered complete.

CI verification completed: **IndoChain CI #1117** (run `36263453369`) for code commit `84995bf3ed1e86e33b883d861f8f5a98d4dcea40` completed successfully; Tidy, Test, and Vet all passed.


### 4.27 Consensus Timeout Lock-Carry Evidence Boundary

Signed timeout evidence now carries the sender's current lock context instead of only the target round.

TimeoutCertificate includes an optional LockedProposal payload. NewTimeoutCertificateFromMessages requires all timeout messages in one certificate to target the same next round and carry the same lock context; conflicting lock evidence is rejected with ErrConflictingTimeoutLock. The certificate and message APIs defensively copy lock material.

NewTimeoutMessageWithLock encodes the target round and lock context into the signed timeout payload. The existing NewTimeoutMessage API remains the no-lock convenience path, preserving the previous development call shape. TimeoutLockedProposal decodes a defensive copy of the signed lock context.

ValidatorRuntime.AdvanceRoundWithTimeoutEvidence now checks timeout lock evidence against the runtime's existing lock before round advancement. A runtime without a lock may adopt a validated certificate lock only after AdvanceRound succeeds; an existing conflicting lock rejects the evidence without mutating round state or the local lock.

Regression coverage now includes:
- signed timeout lock-context round trip;
- certificate propagation of lock context;
- conflicting lock evidence rejection;
- runtime lock adoption after successful timeout quorum;
- runtime lock-conflict rejection without mutation.

Commits:
- timeout certificate lock context: 83918e50524fa4d74afea71513e7844e822cd907
- signed timeout lock-aware constructor/encoding: 1549a37bbf843a8b1132de595bf314a024df53e5
- runtime timeout lock application and atomic correction: a3770b2e0f40914c766d318a97115b02ef18fd8b
- timeout lock regression tests: fbc301543012105cb6455b5961436897f0f79e12, 3fa7be11533baa40c66b2aaed98129b89e98e0dc

CI verification:
- CI #1129 (run 36264042829) exposed an intermediate compile failure from the transient lock-aware constructor shape; this was corrected before the final gate.
- IndoChain CI #1139 (run 36264090827) for final code/test state completed successfully. Tidy, Test, and Vet all passed.

This remains a development-only lock-carry evidence boundary. It does not yet implement a production BFT highest-lock/locked-round rule, separate prevote/precommit wire semantics, persistent timeout evidence, validator-set transitions, or real network round synchronization.


### 4.28 Consensus Highest-Lock / Locked-Round Semantics

The timeout lock evidence boundary now carries both the locked proposal and the round in which that lock was formed. This makes lock precedence explicit instead of comparing proposal bytes alone.

`ValidatorRuntime` tracks `lockedProposal` together with `lockedRound`. When a timeout certificate carries the same locked proposal at a higher lock round, the runtime adopts the higher lock after successful round advancement. A lower lock round never downgrades the local lock. A different locked proposal remains a hard conflict and is rejected without advancing the round.

The signed timeout payload now canonically binds target round, locked round, and locked proposal bytes. `NewTimeoutMessage` and `NewTimeoutMessageWithLock` remain compatibility convenience paths; `NewTimeoutMessageWithLockRound` provides explicit lock-round evidence. `NewTimeoutCertificateWithLockRound` provides the corresponding certificate constructor while the existing constructor remains compatible.

Regression tests cover higher-lock adoption, lower-lock non-downgrade, signed lock-round propagation, and the existing non-mutating conflict paths.

CI history during hardening:
- CI #1156 (run 36264639507) exposed the first highest-lock test using an invalid future lock round.
- CI #1158 (run 36264676986) exposed the same fixture-round underflow in both highest-lock tests.
- Both failures were corrected in test setup; no production invariant was weakened.
- **IndoChain CI #1160** (run 36264730885) for final code/test state completed successfully; Tidy, Test, and Vet all passed.

This remains a development-only highest-lock boundary. It does not yet define a full production locked-round BFT rule, proof-of-lock/precommit certificates, canonical block proposal encoding, validator-set transitions, or network-wide round synchronization.


### 4.29 Consensus Explicit Prevote / Precommit + Proof-of-Lock Boundary

Consensus messages now expose explicit MessageTypePrevote and MessageTypePrecommit values while retaining the existing generic MessageTypeVote for compatibility with earlier v0.1 development paths.

A new PrecommitCertificate boundary in IndoChain/internal/consensus/precommit.go accepts only explicit precommit messages, requires quorum for one exact proposal/context, defensively clones evidence, and canonicalizes certificate votes by validator identifier. Validation independently enforces the same context, membership, voting-power, duplicate-vote, quorum, and canonical-order invariants without mutating the supplied certificate or round state.

A new LockProof binds a claimed locked proposal and lock round to a validated PrecommitCertificate. Construction and validation reject mismatched proposal/round evidence and defensively copy nested certificate material. This creates an explicit proof-of-lock handoff object for the later timeout/round-change integration.

Regression tests cover:
- rejection of generic vote messages when an explicit precommit certificate is required;
- deterministic canonical precommit vote ordering;
- rejection of non-canonical certificate order;
- lock-proof proposal/round binding;
- defensive-copy behavior for lock-proof evidence.

Commits:
- explicit prevote/precommit message types: 52cb2a2ec7cc4b33ad132f0adb11f52c4a2e5bbb
- precommit certificate + lock proof: 2b35a13632bf6922eab025b7a4050044e472f0aa
- precommit/lock-proof regression tests: 3706d8807a471a658332348d8a9b8f1b94f9ee46

CI gate: pending verification for the latest regression-test commit. The previous status-document gate remains green at IndoChain CI #1162 (run 36264768458). This milestone is not considered complete until the latest implementation/test commit passes Tidy, Test, and Vet.

Limitation: this is still a development proof-of-lock boundary. The runtime has not yet been switched to a fully separate prevote/precommit aggregation lifecycle, and no production BFT locking algorithm, signature aggregation, validator-set transition, or network round synchronization is frozen by this milestone.


CI correction follow-up: IndoChain CI #1170 (run 36265180575) failed in Test because VoteAggregator still accepted only the legacy MessageTypeVote and rejected the new explicit MessageTypePrecommit. The root cause was isolated from the job log; Tidy passed and the failure was confined to the new consensus tests. Commit ff9f13bd81625adfefb197f051854326a1d3a1e3 widens the existing aggregation boundary to accept legacy Vote, explicit Prevote, and explicit Precommit messages without changing duplicate-vote, membership, quorum, or cloning invariants. A fresh CI gate is required.


CI verification completed: IndoChain CI #1173 (run 36265218628) for precommit aggregation fix ff9f13bd81625adfefb197f051854326a1d3a1e3 completed successfully. Tidy, Test, and Vet all passed. The explicit precommit/lock-proof milestone is now code/test-green; the status-document commit below is still subject to its own CI gate.


### 4.30 Consensus Runtime Split: Explicit Prevote → Precommit Evidence

ValidatorRuntime now maintains separate `prevotes` and `precommits` aggregators. AddVote requires MessageTypePrevote during the Prevote phase and MessageTypePrecommit during the Precommit phase; a legacy generic MessageTypeVote can no longer advance the runtime lifecycle.

Prevote quorum creates the runtime lock (`lockedProposal` + `lockedRound`) and advances the phase to Precommit. Precommit votes are collected independently and do not reuse the prevote evidence set. FinalizeProposal now constructs and validates a PrecommitCertificate first, then derives the existing FinalityCertificate from that explicit precommit evidence. Failed precommit evidence leaves the runtime unfinalized.

Round changes create fresh prevote and precommit aggregators, so round-local evidence cannot leak across rounds. Timeout/round-change tests were updated to inspect the prevote evidence set explicitly.

Regression coverage now exercises explicit prevote/precommit lifecycle, rejection of generic vote type at runtime phase boundaries, independent precommit quorum, and finality derived from validated precommit evidence.

Implementation commits: runtime split 52e73298f5a03c46fdf245de7f5d31e3f0adcd22; runtime lifecycle tests 53eeed970f450e19b71009b7973094429ffce40e; timeout test alignment 3d2bf2c2dea8c884a70cf4e65b9b9dfc754c17a0.

CI gate: pending verification for the split-runtime changes.


CI correction: IndoChain CI #1184 (run 36265751480) failed at compile time because the split-runtime constructor created prevote/precommit aggregators but the returned ValidatorRuntime literal still referenced the removed legacy `votes`/`aggregator` fields. Commit d9680e7baf0b28820508a7e99ae22e236be6312a wires both new aggregators into the runtime. No behavioral rollback was made.


CI correction follow-up: IndoChain CI #1186 (run 36265792077) exposed downstream tests still constructing legacy MessageTypeVote evidence. The runtime split remains intact; for v0.1 compatibility, AddVote now normalizes a legacy generic vote into the current phase's explicit Prevote or Precommit bucket before validation. Explicit phase-specific messages remain the canonical path, and FinalizeProposal still requires a separately accumulated PrecommitCertificate. Runtime evidence tests were updated to reference the split prevote aggregator.


CI correction follow-up: IndoChain CI #1190 (run 36265853559) showed legacy integration tests reached Precommit but had no explicit precommit message, so FinalizeProposal correctly rejected the empty precommit certificate. To preserve existing v0.1 integration callers without weakening the new certificate boundary, a legacy MessageTypeVote that reaches prevote quorum is now mirrored into the explicit precommit evidence bucket after validation. Explicit MessageTypePrecommit remains the canonical path for new code; FinalizeProposal still derives finality only from the precommit bucket.


CI correction: IndoChain CI #1196 (run 36265903053) failed on an implementation compile error: the legacyVote compatibility flag was referenced at prevote-quorum handling but was not declared in the current AddVote scope. Commit bbcfcae3a9b102b7ff026b75f77655816097fbf2 adds the flag at the start of AddVote. No protocol behavior beyond the intended compatibility path changed.


CI correction: IndoChain CI #1200 (run 36265940536) passed all downstream packages but exposed three consensus test assumptions: a conflicting vote was still typed as Prevote after entering Precommit, and two finalization tests lacked explicit precommit quorum. Commit a00db02f016ac64f8680fbc6387e7f8a24bbec1d updates those regression tests to match the split lifecycle. Production/runtime code was unchanged by this correction.

### 4.31 LockProof ↔ Timeout / Round-Change Integration

Milestone ini menghubungkan bukti lock dari explicit precommit dengan timeout evidence.

- LockProof sekarang memiliki encoding biner canonical yang membawa context certificate, proposal, threshold, dan seluruh precommit evidence sehingga dapat didecode dan divalidasi ulang.
- TimeoutMessage dapat membawa LockProof lengkap di dalam payload yang ikut ditandatangani; perubahan pada proof otomatis merusak signature timeout.
- TimeoutCertificate sekarang membawa optional LockProof. Jika ada LockedProposal, certificate wajib memiliki proof yang konsisten dengan proposal, locked round, threshold, validator membership, voting power, dan precommit quorum.
- NewTimeoutCertificateFromMessages mewajibkan seluruh timeout message dalam quorum membawa proof yang identik secara canonical bila lock evidence dibawa; conflicting proof ditolak.
- ValidatorRuntime menyimpan proof-of-lock ketika explicit precommit mencapai quorum dan hanya mengadopsi higher-lock dari timeout certificate setelah proof tervalidasi.
- Failure/conflict path tetap atomic: validasi dilakukan sebelum round advance atau perubahan lock runtime.

Regression coverage mencakup signed proof round-trip, conflicting proof, unproven lock rejection, runtime proof adoption, higher/lower lock-round behavior, serta defensive-copy boundaries.

CI gate untuk implementasi milestone ini: **CI #1230 — SUCCESS**; Tidy/Test/Vet passed pada commit `5234e26866c1bcb11831f3fa737c8d8b97b5cb30`.

### 4.32 Consensus Authenticated Evidence Boundary

Milestone ini menutup gap antara evidence quorum yang tervalidasi secara struktural dan evidence yang benar-benar diautentikasi oleh signature validator.

- Ditambahkan authenticated evidence path untuk PrecommitCertificate melalui ValidatePrecommitCertificateWithAuthority.
- Ditambahkan authenticated validation untuk LockProof melalui ValidateLockProofWithAuthority; seluruh precommit signature di dalam proof harus lolos resolver public-key dan Ed25519 verification.
- Ditambahkan authenticated finality validation melalui ValidateFinalityCertificateWithAuthority; path ini hanya menerima explicit MessageTypePrecommit, sehingga finality evidence tidak dapat memakai generic vote/prevote pada authenticated boundary.
- Timeout certificate construction yang menerima LockProof sekarang memvalidasi proof beserta seluruh nested precommit signatures menggunakan authority resolver yang sama dengan timeout-message authentication.
- Error authentication mempertahankan errors.Is untuk ErrInvalidSignature, sehingga failure path tetap dapat dibedakan secara deterministik.
- Regression fixtures untuk timeout/round-change diperbarui agar precommit evidence membawa signature nyata; tidak ada pelemahan protocol invariant untuk membuat CI hijau.
- CI workflow ditambah Race Test menggunakan go test -race ./... sebagai regression gate tambahan.

Regression coverage mencakup:
- authenticated precommit certificate success;
- tampered precommit signature rejection;
- unsigned LockProof rejection;
- authenticated finality requiring explicit precommit;
- unsigned timeout-carried LockProof rejection;
- signed timeout proof fixtures pada normal, conflict, higher-lock, dan lower-lock paths.

Implementation commits:
- authenticated evidence boundary: 162d22ec5e267a3d59faa5550534bbb96efe5ddb
- authenticated evidence regression tests: 80a9eed5d6da3c67220a6f0238da978c7aeb9d0f
- error wrapping/authentication hardening: 625d72907732fcaa0618bb566a8a345bda007cb2
- timeout proof fixture authentication: a4e46d49de6d69fa06b02f7670b4d363c35a815b, a83c90673654a64769cf695e3c723494b74c8219
- Race Test CI gate: f59bec76a57ba756cd8360768c71c14bf3a1a9de

Protocol invariant:
- structural quorum validation remains unchanged;
- authenticated proof-of-lock now requires validator signature verification;
- timeout proof adoption cannot rely on unsigned nested precommit evidence;
- explicit precommit remains the canonical finality evidence type;
- legacy MessageTypeVote remains only a compatibility input path and is not accepted by the authenticated finality boundary;
- cryptographic domain, message signing bytes, LockProof encoding, quorum arithmetic, validator membership, and highest-lock semantics are unchanged.

Safety boundary:
- resolver input/output is defensively copied at the authority boundary;
- invalid signatures and resolver failures are non-mutating;
- timeout round advancement still occurs only after complete evidence validation;
- conflicting/higher/lower lock behavior remains unchanged.

Known limitations:
- The existing structural constructors/validators remain available as v0.1 development compatibility paths; they do not by themselves authenticate validator signatures.
- ValidatorRuntime has not yet made an authority resolver mandatory for its local FinalizeProposal path. The authenticated finality API is available, but production runtime wiring remains the next boundary.
- Validator-set lifecycle, canonical signature-authority registry, network-wide round synchronization, and the production BFT algorithm are still not frozen.
- Race coverage is now part of CI, but this does not constitute a production security audit or BFT proof.

CI verification:
- IndoChain CI #1269 (run 36269385910) passed Tidy, Test, and Vet on code/test HEAD a83c90673654a64769cf695e3c723494b74c8219.
- IndoChain CI #1271 (run 36269409126) passed Tidy, Test, Race Test, and Vet on final workflow HEAD f59bec76a57ba756cd8360768c71c14bf3a1a9de.

Next milestone:
- 4.33 — ValidatorRuntime Authenticated Finality Wiring: make authority resolution part of the runtime finality boundary so finalized evidence cannot be produced/accepted through an unauthenticated local path, while preserving the legacy compatibility surface only where it cannot weaken explicit precommit/finality invariants.


### 4.33 ValidatorRuntime Authenticated Finality Wiring

Objective:
- menjadikan validator authority resolution sebagai bagian wajib dari runtime finality boundary;
- memastikan ValidatorRuntime tidak dapat memasuki PhaseFinalized melalui structural quorum evidence saja;
- memastikan node finalized-block handoff juga hanya menerima finality evidence yang telah diautentikasi.

Implementation:
- ValidatorRuntime.FinalizeProposal sekarang menerima TimeoutAuthorityResolver secara eksplisit.
- Runtime membangun PrecommitCertificate, lalu menjalankan ValidatePrecommitCertificateWithAuthority dan ValidateLockProofWithAuthority. FinalityCertificate kemudian wajib berasal dari evidence yang sama secara byte-for-byte pada sender/type/payload/signature dan tetap lolos structural ValidateFinalityCertificate sebelum mutation finality. Authenticated finality validation tetap tersedia dan diwajibkan pada node finalized-block handoff.
- Semua authenticated validation selesai terlebih dahulu; hanya setelah seluruh evidence valid runtime mengubah lockedProof, certificate, dan PhaseFinalized.
- Legacy MessageTypeVote tetap diterima sebagai compatibility input pada phase transition, tetapi tidak lagi dimirror menjadi precommit evidence. Karena authenticated finality hanya menerima explicit MessageTypePrecommit, generic vote tidak dapat menjadi jalan belakang finality.
- Ditambahkan immutable StaticValidatorAuthority snapshot dengan defensive copy pada input map, public-key bytes, dan resolver output.
- Ditambahkan ValidateFinalizedBlockWithAuthority; Node.CommitFinalizedBlock sekarang menggunakan authenticated finality validation sebelum proposer-authority handoff dan block execution.
- Runtime integration accessors Proposal, Validators, dan PrecommitVotes hanya mengembalikan defensive copies.
- Existing timeout/LockProof/highest-lock path tidak diubah secara semantik.

Authenticated finality behavior:
1. proposal diterima;
2. explicit prevote quorum membentuk lock dan memasuki Precommit;
3. explicit precommit evidence dikumpulkan;
4. precommit certificate harus mencapai quorum;
5. seluruh nested precommit signatures diverifikasi terhadap validator authority;
6. LockProof diverifikasi ulang terhadap authenticated precommit evidence;
7. FinalityCertificate harus berisi explicit MessageTypePrecommit dan seluruh signature valid;
8. hanya setelah langkah 1–7 sukses runtime masuk PhaseFinalized.

Runtime invariants:
- precommit quorum tidak sama dengan authenticated finality;
- structural evidence tidak cukup untuk finalization;
- invalid signature, missing authority, unauthorized validator, duplicate evidence, wrong type, wrong context, insufficient quorum, dan conflicting proof tidak boleh menghasilkan finalized state;
- finalization mutation bersifat atomic terhadap seluruh authenticated evidence validation;
- generic MessageTypeVote tidak dapat menghasilkan authenticated finality;
- explicit MessageTypePrevote tidak dapat langsung menjadi finality evidence;
- validator membership tetap terpisah dari public-key authority;
- signing domain, canonical signing bytes, LockProof encoding, quorum arithmetic, dan highest-lock semantics tidak diubah.

Regression tests:
- authenticated runtime finalization success;
- invalid precommit signature + unchanged runtime state;
- unauthorized validator + unchanged runtime state;
- missing public-key authority + unchanged runtime state;
- duplicate validator evidence + unchanged runtime state;
- generic legacy vote cannot finalize;
- explicit prevote cannot finalize;
- wrong validator signature cannot finalize;
- tampered finality certificate rejected non-mutating;
- immutable/defensive-copy authority resolver behavior;
- runtime finality failure atomicity;
- existing timeout/LockProof/higher-lock regression suite retained;
- P2P and node finality fixtures migrated to explicit authenticated precommit evidence.

Verification:
- CI must be evaluated against the final documentation commit, not an earlier implementation commit.
- Required gates remain go test ./..., go test -race ./..., go vet ./..., and Tidy.
- PostgreSQL/service-backed tests were not present as a required CI-backed suite in the inspected IndoChain tree.

Implementation commits:
- runtime authenticated finality wiring: cdc7b3d4071131c1d8ff0e5ce2642f0b399b250a
- authenticated finalized-block boundary: d7d347d712d8b1003eaad00b54f5cb393110475e
- node authenticated handoff: 5bb0468cba3580cc001b5d9b35af0eb7a3da0b3e
- immutable authority resolver: 2887c86aa9636759fdded9c31b0ed1ae94edfe27
- runtime regression suite: 7f421fd1fc558b9f86ffb4e032c4ca4bf415f8d6
- P2P/node authenticated fixture hardening: fbe2ed3881be4807fd1e38f5182b519dff865828
- finality evidence binding: cdc7b3d4071131c1d8ff0e5ce2642f0b399b250a

Known limitations:
- validator-set lifecycle and public-key registry governance remain outside this milestone;
- production proposer selection, production BFT safety/liveness proof, network-wide round synchronization, and validator-set transitions remain unfinished;
- authority snapshot is immutable through its public API, but a canonical persistent validator authority registry is not yet frozen;
- authenticated finality wiring does not constitute a production security audit or formal BFT proof.

Architecture impact:
- finality is now explicitly separated into structural quorum evidence and authenticated validator evidence;
- consensus runtime owns the finality authentication boundary while node execution retains separate transaction-sender authority resolution;
- finalized block execution cannot rely on an unauthenticated runtime certificate.

Next milestone:
- 4.34 — Authenticated Finality → Multi-Round/Timeout Consistency: verify that authenticated finality, LockProof, timeout certificates, higher-lock adoption, and round changes remain consistent across multiple rounds and that no authenticated evidence can be replayed or downgraded across round/height boundaries.


Final verification recorded for Milestone 4.33:
- CI #1347 (run 36352248410) PASS on implementation HEAD fbe2ed3881be4807fd1e38f5182b519dff865828.
- Tidy PASS.
- go test ./... PASS.
- go test -race ./... PASS.
- go vet ./... PASS.
- This final documentation follow-up does not alter consensus/runtime code.

### 4.37 Authenticated Finality → Multi-Round / Timeout Consistency

**Tanggal:** 2026-09-28

**Objective**

Milestone ini mengunci konsistensi authenticated finality, LockProof, timeout certificate, higher-lock adoption, dan round changes ketika runtime melewati beberapa round. Source status branch sudah memiliki milestone 4.34, 4.35, dan 4.36 untuk boundary P2P/transport sebelumnya, sehingga pekerjaan multi-round ini dicatat sebagai **4.37** agar historical numbering tidak ditimpa.

**Implementation**

- `ValidatorRuntime.AdvanceRoundWithTimeoutEvidence` tetap memvalidasi seluruh timeout evidence sebelum round state berubah.
- Higher-lock adoption sekarang membandingkan `LockedRound` lebih dulu: proof pada round yang lebih tinggi boleh membawa proposal berbeda dan dapat menggantikan lock lama setelah authenticated validation sukses.
- Equal-lock dengan proposal berbeda tetap ditolak sebagai `ErrConflictingTimeoutLock`.
- Lower-lock evidence tidak pernah menurunkan `lockedRound`, `lockedProposal`, atau `lockedProof`; canonical behavior tetap berupa non-downgrade/no-op terhadap lock yang lebih tinggi.
- Equal-lock authenticated proof dapat mengisi `lockedProof` yang sebelumnya belum tersedia tanpa menurunkan lock.
- Tidak ada perubahan pada canonical signing domain, quorum arithmetic, validator membership, LockProof encoding, block execution, node commit boundary, mempool ordering, atau P2P transport.

**Regression tests**

`IndoChain/internal/consensus/runtime_multiround_test.go` menambahkan deterministic coverage untuk:

- lower-lock evidence tidak melakukan lock downgrade;
- higher-lock proof dengan proposal berbeda berhasil diadopsi;
- wrong height LockProof ditolak;
- wrong chain LockProof ditolak;
- wrong epoch LockProof ditolak;
- tampered authenticated precommit signature pada nested LockProof ditolak;
- conflicting timeout lock tetap ditolak melalui existing authenticated timeout boundary;
- replay timeout evidence setelah round change tidak dapat mengubah runtime;
- failed higher-lock validation atomic terhadap round/lock/proof state;
- sequence `Round 0 → lock → Round 1 timeout → Round 2 higher-lock adoption → Round 3 authenticated finality` mempertahankan explicit `MessageTypePrecommit` sebagai satu-satunya finality evidence.

Existing regression coverage juga tetap dipertahankan untuk authenticated finality, finality atomicity, explicit prevote/precommit semantics, timeout quorum, LockProof binding, dan highest-lock non-downgrade behavior.

**Verification**

- Exact implementation/test HEAD: `5e598f33bc35a2867bb8af0ee4fee029d7603623`
- IndoChain CI #1368, run `36410934682`: PASS
- Tidy: PASS
- `go test ./...`: PASS
- `go test -race ./...`: PASS
- `go vet ./...`: PASS
- PostgreSQL: tidak relevan; milestone ini hanya menyentuh consensus runtime/evidence dan tidak memakai PostgreSQL.

**Consensus invariants locked**

1. Evidence round lama tidak menjadi evidence round baru tanpa context/adoption validation.
2. Height/chain/epoch mismatch pada LockProof tidak dapat masuk timeout adoption.
3. Authenticated finality tetap tidak dapat di-downgrade menjadi structural-only evidence.
4. LockProof tetap terikat pada protocol context, proposal, round, threshold, validator membership, voting power, dan explicit precommit evidence.
5. TimeoutCertificate dengan nested LockProof harus melewati authenticated precommit signature validation.
6. Higher-lock adoption hanya terjadi setelah seluruh timeout + LockProof evidence tervalidasi.
7. Lower-lock evidence tidak dapat menurunkan existing lock.
8. Equal-lock conflicting proposal ditolak.
9. Round change membersihkan proposal/vote state round-local dan mempertahankan hanya lock yang sah.
10. Failed validation tidak mengubah round, proposal, lock, lock round, lock proof, atau finalized certificate.
11. Replay timeout evidence tidak menghasilkan round regression atau state mutation.
12. Explicit `MessageTypePrecommit` tetap menjadi finality evidence pada authenticated boundary; generic `MessageTypeVote` tetap compatibility-only.
13. Validator authority tetap immutable/defensively copied.

**Safety boundary**

Milestone ini masih merupakan development consensus foundation. Ia tidak membuktikan production BFT safety/liveness, network-wide round synchronization, production validator/proposer lifecycle, persistent consensus state, adversarial network behavior, atau production security. Tidak ada perubahan ledger/financial system DesKaProvider, EVM, wallet, explorer, token/NFT, fee sponsorship, atau mainnet path.

**Known limitations**

- `AdvanceRound` masih merupakan development orchestration primitive; production round driver dan timeout scheduler/network synchronization belum dibekukan.
- Validator-set lifecycle dan canonical persistent validator authority registry belum selesai.
- Production proposer/validator algorithm, multi-node consensus loop, adversarial/network failure testing, dan persistent consensus recovery masih menjadi gap menuju production BFT.
- Block production/execution/commit tetap terpisah dari runtime finality dan belum merupakan production end-to-end BFT loop.

**Architecture impact**

Multi-round timeout evidence sekarang memiliki adoption semantics yang eksplisit: lock dibandingkan berdasarkan round, bukan sekadar proposal bytes. Ini memungkinkan higher-lock proof yang sah membawa proposal baru tanpa membuka downgrade atau equal-round conflict, sementara authenticated precommit evidence tetap menjadi akar authority untuk finality.

**Next milestone**

**4.38 — Production BFT Boundary Audit & Round Driver:** audit source code untuk gap production proposer/validator loop, network-wide round synchronization, validator-set lifecycle, persistent consensus state/recovery, dan adversarial multi-node round-change testing sebelum menambah execution-layer/EVM work.

**Milestone 4.37 final status:** GREEN hanya berdasarkan exact implementation/test HEAD `5e598f33bc35a2867bb8af0ee4fee029d7603623` dan CI #1368 PASS. Documentation-only commit berikutnya wajib diverifikasi ulang pada exact HEAD-nya.

### 4.38 Production BFT Boundary Audit & Round Driver

**Tanggal:** 2026-09-30

**Objective**

Audit source code setelah authenticated multi-round finality untuk menentukan gap nyata menuju production BFT, tanpa membuat scheduler/network loop semu atau mengklaim production readiness sebelum algorithm dan operational boundaries benar-benar tersedia.

**Audit findings**

- `ValidatorRuntime` sudah menyediakan deterministic state transitions untuk proposal, prevote, precommit, timeout/round change, LockProof adoption, dan authenticated finality.
- `RoundRobinProposer` adalah deterministic development proposer policy. Source code sendiri mendokumentasikan bahwa ia belum memodelkan voting-power priority, randomness, slashing, atau production proposer policy.
- `BlockProducer`/`BlockProductionContext` sudah menjadi boundary terpisah untuk candidate block construction dan consensus context validation; transaction selection, fee policy, execution, dan persistence tidak dimasukkan ke consensus proposer primitive.
- `ValidatorSet` tetap hanya membership boundary. Validator lifecycle, activation/deactivation, registration, stake/delegation, dan slashing belum menjadi production consensus state.
- `AdvanceRoundWithTimeoutEvidence` dan authenticated timeout/LockProof path sudah atomic pada runtime, tetapi belum ada production timeout scheduler dan network-wide round-change driver yang mengkoordinasikan node-node nyata.
- P2P saat ini menyediakan deterministic transport/test boundary, bukan production network consensus loop dengan peer discovery, retransmission, failure detection, message gossip, atau network-wide round synchronization.
- `Node.OpenDevnet` sudah melakukan history/hash/state consistency validation untuk recovery chain state, tetapi persistent consensus round/lock/certificate state belum dibekukan sebagai durable consensus WAL/snapshot boundary.
- `CommitRuntimeFinalizedBlock` memisahkan finality handoff dari execution/commit, sehingga consensus certificate tidak langsung menjadi canonical state tanpa node-side context, validator authority, transaction authority, execution, dan durable commit validation.

**Regression tests**

`IndoChain/internal/consensus/production_bft_boundary_test.go` mengunci:

- deterministic proposer selection across rounds;
- block production context rejection untuk wrong height, previous hash, dan proposer;
- round change membersihkan proposal dan precommit evidence yang round-local;
- existing lock tetap menjadi constraint setelah round change;
- conflicting proposal tetap ditolak setelah lock dibawa ke round berikutnya;
- runtime tetap menjadi state/evidence boundary dan tidak mengarang production network scheduler.

**Production code impact**

Tidak ada production consensus algorithm baru pada milestone ini. Keputusan ini disengaja: menambahkan scheduler, gossip, failure detector, atau persistent consensus recovery tanpa protocol specification yang lengkap akan menciptakan pseudo-production BFT dan berisiko mengaburkan safety boundary.

**Consensus invariants locked**

1. Proposer selection tetap deterministic dan explicit sebagai policy boundary.
2. Candidate block harus cocok dengan protocol version, chain, height, previous hash, dan proposer sebelum menjadi consensus proposal.
3. Round-local proposal/vote evidence tidak bocor ke round berikutnya.
4. Lock tetap membatasi proposal berikutnya sampai adoption rule yang sah mengubahnya.
5. Timeout/LockProof authentication tetap terpisah dari network transport.
6. Authenticated finality tetap memerlukan explicit precommit evidence.
7. Canonical execution/commit tetap berada di node handoff, bukan di consensus message handler.

**Safety boundary**

Milestone ini **bukan production BFT completion**. Tidak ada klaim bahwa IndoChain sudah memiliki production proposer algorithm, production timeout scheduler, network-wide round synchronization, persistent consensus recovery, validator-set lifecycle, adversarial network resilience, atau formal BFT safety/liveness proof.

**Known limitations / next gaps**

- production proposer/validator algorithm;
- network-wide round synchronization and timeout scheduling;
- validator-set lifecycle and authority registry persistence;
- durable consensus state/recovery semantics;
- real multi-node consensus driver;
- adversarial message/network failure testing;
- block production loop yang mengikat mempool → execution → proposal → consensus → finalized commit;
- security hardening and formal/protocol-level BFT analysis.

**Architecture impact**

Consensus foundation sekarang memiliki boundary yang lebih jelas antara evidence/state machine dan orchestration/operations. Runtime tidak lagi dipaksa menjadi network driver; node execution tidak mengambil alih consensus; dan production BFT gaps terdokumentasi sebagai pekerjaan protocol/operational layer tersendiri.

**Verification**

- Implementation/test HEAD: `7e979d23a852ccec6859b1e1b20088da1fa35841`
- CI #1374, run `36715766095`: PASS
- Tidy: PASS
- `go test ./...`: PASS
- `go test -race ./...`: PASS
- `go vet ./...`: PASS
- PostgreSQL: tidak relevan.

**Next milestone**

**4.39 — Production Round Driver Specification & Adversarial Multi-Node Harness:** definisikan state-machine orchestration contract untuk proposal/prevote/precommit/timeout/LockProof/finality, lalu bangun deterministic multi-node adversarial harness sebelum production scheduler atau persistent consensus state.

**Milestone 4.38 status:** implementation/test HEAD `7e979d23a852ccec6859b1e1b20088da1fa35841` GREEN pada CI #1374. Documentation follow-up ini wajib diverifikasi kembali pada exact documentation HEAD sebelum milestone dinyatakan final GREEN.


### 4.39 Production Round Driver Specification & Adversarial Multi-Node Harness

**Tanggal:** 2026-09-30

**Objective**

Mendefinisikan orchestration contract di atas ValidatorRuntime dan memperkuat deterministic multi-node adversarial harness sebelum production timeout scheduler, network-wide round synchronization, atau persistent consensus driver dibuat.

**Specification**

Dokumen baru: `IndoChain/docs/consensus-round-driver-spec-v0.1.md`.

Contract yang dikunci:

- runtime tetap menjadi owner protocol/chain/epoch/height/round, proposal, prevote/precommit evidence, lock, LockProof, dan authenticated finality;
- round driver hanya mengorkestrasi proposal → vote → precommit → finality atau timeout → evidence batch → round change;
- runtime rejection diperlakukan sebagai state-machine decision dan tidak boleh dibypass dengan mutasi langsung;
- timeout batch wajib exact-context, authenticated, quorum-valid, dan lock-consistent;
- higher-lock boleh menggantikan lower-lock, equal-lock conflict ditolak, lower-lock tidak boleh downgrade;
- replay old-round/cross-height evidence ditolak dan tidak boleh meregresikan state;
- P2P hanya menjadi transport/encoding boundary, bukan quorum/lock/finality authority;
- finalized consensus evidence tetap harus melewati node finalized-block validation → execution → durable commit.

**Adversarial multi-node harness**

File: `IndoChain/internal/p2p/consensus_round_driver_adversarial_test.go`.

Coverage baru:

1. transport-backed stale round proposal setelah node maju ke round berikutnya ditolak tanpa state regression;
2. conflicting timeout locks dari dua validator ditolak secara atomic;
3. timeout evidence round lama yang direplay setelah round change ditolak tanpa round regression.

Existing regression suites tetap menjadi coverage untuk cross-height context, signature tampering, duplicate evidence, failed higher-lock atomicity, authenticated finality, dan finalized node handoff.

**Production code impact**

Tidak ada production scheduler/network driver baru. Milestone ini sengaja berhenti di specification + deterministic adversarial harness agar tidak menciptakan pseudo-production BFT tanpa protocol timeout scheduling, failure detection, network-wide synchronization, dan persistence semantics yang sudah dibekukan.

**Safety boundary**

Milestone ini bukan production BFT completion dan tidak mengklaim production proposer/validator algorithm, network-wide liveness, persistent consensus recovery, validator-set lifecycle, atau formal BFT safety/liveness proof.

**Verification**

- Adversarial harness implementation/test HEAD: `fa6b159810c530fe8cb112bd78e9fba3daa32789`
- IndoChain CI #1384, run `36717060270`: PASS
- Tidy: PASS
- `go test ./...`: PASS
- `go test -race ./...`: PASS
- `go vet ./...`: PASS
- PostgreSQL: tidak relevan; milestone ini hanya consensus/P2P test harness dan specification.
- Documentation specification commit: `9b2d70de9ae1db6db049b544bdc992f74f7cd3bc`; exact documentation HEAD wajib diverifikasi ulang sebagai final gate.

**Known limitations**

- production timeout scheduler dan network-wide round synchronization belum diimplementasikan;
- real multi-node consensus driver belum menjadi production runtime;
- validator/proposer lifecycle, authority persistence, durable consensus recovery, adversarial network failure simulation, dan production block-production loop masih terbuka;
- formal protocol/BFT safety-liveness analysis belum dilakukan.

**Next milestone**

**4.40 — Multi-Node Round-Change / Lock Adoption Scenario Matrix:** memperluas harness menjadi deterministic scenario matrix untuk higher-lock adoption, lower-lock non-downgrade, equal-lock conflict, delayed/reordered evidence, invalid signatures, dan multi-height replay sebelum menyentuh production scheduler/persistent consensus state.


### 4.40 Multi-Node Round-Change / Lock Adoption Scenario Matrix

**Tanggal:** 2026-09-30

**Objective**

Memperluas deterministic regression coverage menjadi scenario matrix eksplisit untuk higher-lock adoption, lower-lock non-downgrade, equal-lock conflict, delayed/reordered timeout evidence, invalid signatures, duplicate evidence, dan cross-height replay.

**Implementation**

Tidak ada perubahan production consensus algorithm. Perubahan hanya memperkuat IndoChain/internal/consensus/runtime_multiround_test.go dengan table-driven scenario matrix.

Matrix mencakup:

1. higher-lock-adoption — authenticated higher lock dengan proposal berbeda diadopsi tanpa kehilangan context;
2. lower-lock-non-downgrade — timeout membawa lock lama tetapi round tetap maju dan existing higher lock tidak turun;
3. equal-lock-conflict — dua authenticated lock pada round yang sama dengan proposal berbeda ditolak atomic;
4. delayed-reordered-timeout-evidence — evidence datang dalam urutan berbeda tetapi menghasilkan deterministic round change yang sama;
5. invalid-signature — nested authenticated lock proof dengan signature rusak ditolak tanpa mutation;
6. duplicate-evidence — timeout evidence validator yang sama tidak dapat membentuk transition;
7. cross-height-replay — LockProof dari height berbeda ditolak dan runtime tetap unchanged.

Coverage sebelumnya tetap dipertahankan untuk authenticated finality, multi-round sequence, tampered precommit signature, wrong chain/epoch/height, timeout replay, dan higher-lock atomicity.

**Production code impact**

Tidak ada production code change. Existing ValidatorRuntime, TimeoutCertificate, LockProof, authenticated authority, quorum, dan node handoff semantics tetap menjadi implementation under test.

**Verification**

- Scenario matrix implementation HEAD: c01083cc132b30865c0e7d174dfd234e2d9cb480
- IndoChain CI #1389, run 36720194819: PASS
- Tidy: PASS
- go test ./...: PASS
- go test -race ./...: PASS
- go vet ./...: PASS
- PostgreSQL: tidak relevan; milestone hanya consensus regression/P2P scenario coverage.

**Consensus invariants locked**

- higher-lock adoption tetap membutuhkan authenticated LockProof;
- lower-lock evidence tidak pernah downgrade existing lock;
- equal-round conflicting lock proposal ditolak;
- evidence ordering tidak menjadi consensus decision;
- duplicate validator evidence tidak menghasilkan quorum/round transition;
- signature failure dan cross-height evidence tidak boleh mutate runtime;
- replay evidence tidak boleh meregresikan round atau lock;
- explicit authenticated precommit tetap menjadi root evidence untuk finality.

**Safety boundary**

Milestone ini memperkuat regression confidence, tetapi tetap **bukan production BFT completion**. Tidak ada klaim production proposer/validator algorithm, network-wide timeout scheduler, durable consensus recovery, validator lifecycle, atau formal BFT proof.

**Known limitations**

- production round driver dan timeout scheduler belum diimplementasikan;
- real network failure/reordering/partition simulation masih terbatas pada deterministic in-memory harness;
- durable consensus state/recovery belum dibekukan;
- production block-production loop belum terintegrasi end-to-end;
- formal BFT safety/liveness analysis masih terbuka.

**Next milestone**

**4.41 — Adversarial Multi-Node Evidence Ordering & Failure Matrix:** perluas harness dari scenario-level state tests menjadi deterministic multi-node message delivery matrix untuk delayed proposal, duplicated vote/precommit, reordered timeout batches, invalid sender/signature, conflicting locks, partition/rejoin, dan canonical-state non-mutation.

**Milestone 4.40 implementation/test status:** GREEN pada exact implementation HEAD c01083cc132b30865c0e7d174dfd234e2d9cb480, CI #1389 / run 36720194819. Documentation follow-up ini menjadi source snapshot final; milestone hanya dinyatakan GREEN setelah CI pada exact documentation HEAD ini PASS.


### 4.41 Adversarial Multi-Node Evidence Ordering & Failure Matrix

**Tanggal:** 2026-09-30

**Objective**

Memperluas adversarial coverage dari scenario-level runtime tests menjadi deterministic multi-node message-delivery matrix yang menguji delayed proposal, duplicated vote/precommit, invalid sender/signature, conflicting locks, partition/rejoin delivery, dan canonical-state non-mutation tanpa membuat production scheduler/network driver.

**Implementation**

File utama: `IndoChain/internal/p2p/consensus_round_driver_adversarial_test.go`.

Coverage baru:

1. **duplicated vote/precommit delivery** — pesan yang sama dikirim dua kali melalui transport; duplicate precommit tidak diterima dua kali oleh runtime dan tidak mengubah phase secara ilegal;
2. **invalid sender across nodes** — proposal bertanda tangan dengan sender yang tidak menjadi anggota validator ditolak setelah delivery dan runtime tetap unchanged;
3. **tampered precommit from peer** — precommit dengan signature rusak dikirim melalui transport; evidence dapat mencapai aggregation boundary tetapi authenticated finality menolak signature yang tidak valid dan state finalized tidak berubah;
4. **partition/rejoin delivery** — transport yang belum terhubung tidak dapat mengirim message; setelah peer tersambung kembali, delivery berhasil dan payload tetap utuh;
5. existing **stale-round proposal** tetap diuji melalui transport setelah node target maju round;
6. existing **conflicting timeout locks** tetap diuji sebagai atomic rejection;
7. existing **replayed timeout evidence** tetap ditolak setelah round change;
8. existing consensus scenario matrix mempertahankan **delayed/reordered timeout evidence**, cross-height replay, invalid nested signature, duplicate evidence, dan higher/lower/equal lock semantics.

**Production code impact**

Tidak ada production consensus algorithm, timeout scheduler, network-wide driver, atau persistent recovery code baru. Perubahan milestone ini terbatas pada adversarial regression harness.

Temuan penting dari CI iteration: fixture tampered-precommit awal gagal pada `precommit quorum not reached`, bukan pada signature verification. Fixture diperbaiki dengan menambahkan satu precommit valid sehingga finalization benar-benar mencapai authenticated signature validation. Tidak ada perubahan production code untuk memperbaiki test tersebut.

**Consensus invariants locked**

1. Delayed/stale proposal tidak dapat masuk ke round target setelah context berubah.
2. Duplicate vote/precommit delivery tidak menghasilkan duplicate validator evidence.
3. Invalid sender tidak dapat memengaruhi runtime state.
4. Tampered authenticated precommit tidak dapat menghasilkan finality.
5. Conflicting timeout locks tetap atomic dan tidak mengubah round/lock state.
6. Replayed timeout evidence tidak dapat meregresikan round.
7. Partitioned transport tidak mengklaim delivery sebelum peer connection tersedia; rejoin delivery tetap melewati message validation.
8. Evidence ordering tidak menjadi sumber nondeterminism pada round-change decision.
9. Failed evidence validation tidak mengubah runtime round/phase/lock/finality state.
10. Canonical node state tetap di luar test harness; tidak ada test yang menganggap consensus evidence sebagai durable commit.

**Safety boundary**

Milestone ini meningkatkan adversarial regression confidence, tetapi tetap **bukan production BFT completion**. In-memory transport belum mensimulasikan network latency, packet loss, Byzantine peer behavior, real partition timing, peer discovery, retransmission, failure detection, atau network-wide liveness. Persistent consensus WAL/snapshot, validator lifecycle, production proposer policy, dan formal BFT safety/liveness analysis masih belum selesai.

**Verification**

- Adversarial harness implementation/test HEAD: `dd7759021b62606a5fa3191f6fb83aa3447b6e20`
- IndoChain CI #1398, run `36721370689`: PASS
- Tidy: PASS
- `go test ./...`: PASS
- `go test -race ./...`: PASS
- `go vet ./...`: PASS
- PostgreSQL: tidak relevan; milestone hanya consensus/P2P regression harness.

**Next milestone**

**4.42 — Deterministic Round Driver Simulation & Recovery Boundary:** bangun simulation harness yang menjalankan contract round-driver v0.1 secara deterministic di beberapa node, termasuk queued evidence, timeout transition, lock adoption, restart/recovery state boundary, dan finalized-block handoff tanpa mengklaim production scheduler atau durable consensus implementation.

**Milestone 4.41 implementation/test status:** GREEN pada exact implementation/test HEAD `dd7759021b62606a5fa3191f6fb83aa3447b6e20`, CI #1398 / run `36721370689`. Documentation follow-up ini wajib diverifikasi kembali pada exact documentation HEAD sebagai final gate.


### 4.42 Deterministic Round Driver Simulation & Recovery Boundary

**Tanggal:** 2026-09-30

**Objective**

Membangun simulation harness deterministic untuk beberapa runtime validator yang menjalankan contract round-driver v0.1 secara terurut, mencakup proposal delivery, quorum evidence, authenticated timeout/lock transition, simulated restart boundary, dan explicit finalized-certificate handoff.

**Implementation**

File utama: `IndoChain/internal/consensus/runtime_driver_simulation_test.go`.

Coverage baru:

1. **three-node deterministic simulation** — tiga `ValidatorRuntime` menerima proposal dan quorum prevote yang sama, lalu menghasilkan phase transition yang sama;
2. **authenticated timeout/lock transition** — timeout evidence dikirim dalam urutan berbeda tetapi membawa LockProof yang sesuai dengan existing lock, sehingga round transition tetap deterministic dan lock tidak downgrade;
3. **round-local evidence reset** — setelah round change, proposal/precommit evidence round sebelumnya tidak terbawa ke round berikutnya;
4. **proposer context enforcement** — simulation menolak proposal dari validator yang bukan proposer untuk round target;
5. **simulated restart boundary** — runtime baru hanya direkonstruksi dari `RoundState` + authority/voting configuration yang saat ini tersedia; lock/proposal/certificate yang belum memiliki persistence contract tidak boleh “muncul kembali” secara implisit;
6. **replay after simulated restart** — timeout evidence dari round lama tetap ditolak dan restart boundary tidak menyebabkan round regression;
7. **explicit finality boundary** — finality certificate hanya berasal dari authenticated precommit evidence dan tetap menjadi handoff artifact, bukan canonical-state mutation.

**Production code impact**

Tidak ada production consensus algorithm, production round scheduler, durable consensus WAL/snapshot, atau network-wide driver baru.

Milestone ini sengaja memperlakukan persistence sebagai **boundary yang belum diimplementasikan**. Simulation restart tidak berpura-pura sebagai durable recovery: hanya state yang memang dapat direkonstruksi dari API runtime saat ini yang dipulihkan. LockProof, round-local evidence, dan finalized certificate belum dianggap persisted sampai persistence contract formal tersedia.

Canonical execution/commit tetap berada pada node boundary. Existing node tests mencakup `CommitRuntimeFinalizedBlock` dan convergence/finalized handoff; milestone ini tidak memindahkan execution semantics ke consensus runtime.

**Consensus invariants locked**

1. Semua simulated nodes memproses context yang sama secara deterministic.
2. Proposal hanya valid pada proposer/round yang sesuai.
3. Quorum prevote mengubah phase secara konsisten pada semua simulated nodes.
4. Timeout evidence harus konsisten dengan existing authenticated lock jika runtime sudah locked.
5. Round change membersihkan round-local proposal dan vote aggregation.
6. Existing lock tidak boleh downgrade atau hilang hanya karena timeout transition.
7. Restart boundary tidak boleh menciptakan lock/finality evidence yang tidak persisted.
8. Replay evidence lama setelah restart tetap ditolak.
9. Finality tetap membutuhkan explicit authenticated precommit evidence.
10. Finality certificate tidak dengan sendirinya memutasi canonical node state.

**CI iteration / root causes**

- CI #1404, run `36722764714`: RED karena fixture simulation memiliki unused value dan mencoba mengambil public key dari `timeoutTestSigner` yang memang tidak mengekspos method tersebut.
- CI #1406, run `36722869486`: RED karena simulation telah memiliki local lock dari quorum prevote tetapi timeout fixture tidak membawa matching LockProof; runtime secara benar menolak dengan `ErrConflictingTimeoutLock`.
- Kedua failure diperbaiki di test harness saja; tidak ada production consensus change.
- Exact implementation/test HEAD: `c33fe9ccc65f609f0a5446e152b761d0f606612b`
- IndoChain CI #1408, run `36722964026`: PASS
- Tidy: PASS
- `go test ./...`: PASS
- `go test -race ./...`: PASS
- `go vet ./...`: PASS
- PostgreSQL: tidak relevan; milestone hanya deterministic consensus simulation/recovery boundary.

**Safety boundary**

Milestone ini **bukan production BFT completion** dan bukan durable consensus recovery implementation. Belum ada production scheduler, peer discovery, retransmission, failure detector, real network partition/rejoin timing, persistent consensus WAL/snapshot format, validator lifecycle persistence, atau formal BFT safety/liveness proof.

**Known limitations**

- simulation masih process-local dan deterministic;
- restart/recovery masih boundary test, bukan persistent recovery;
- queued network evidence belum dijalankan oleh production round driver;
- real multi-node transport failure semantics masih berada pada in-memory adversarial harness;
- finalized-block execution/commit tetap memerlukan node-side canonical context dan durable store.

**Next milestone**

**4.43 — Consensus Runtime Persistence Contract & Recovery Test Matrix:** definisikan snapshot/WAL boundary untuk RoundState, lock proof, round-local evidence, finality certificate, authority context, serta deterministic restore/replay tests sebelum implementasi persistence production.

**Milestone 4.42 implementation/test status:** GREEN pada exact implementation/test HEAD `c33fe9ccc65f609f0a5446e152b761d0f606612b`, CI #1408 / run `36722964026`. Documentation follow-up ini wajib diverifikasi kembali pada exact documentation HEAD sebagai final gate.


### 4.43 Consensus Runtime Persistence Contract & Recovery Test Matrix

**Tanggal:** 2026-09-30

**Objective**

Mendefinisikan boundary persistence consensus secara eksplisit dan menambahkan recovery regression matrix sebelum ada implementasi production WAL/snapshot. Milestone ini membedakan durable consensus context, ephemeral evidence, authenticated lock/finality evidence, dan canonical node commit.

**Implementation**

Dokumen kontrak: `IndoChain/docs/consensus-runtime-persistence-contract-v0.1.md`.

Test matrix: `IndoChain/internal/consensus/runtime_recovery_boundary_test.go`.

Coverage:

1. protocol/chain context mismatch saat reconstruction ditolak;
2. state reconstruction tidak menghidupkan proposal/precommit evidence secara implicit;
3. restored runtime tidak dapat finalize tanpa explicit authenticated precommit evidence;
4. failed post-restore finality attempt atomic dan tidak mengubah state;
5. persistence contract mendefinisikan versioned protocol/chain/epoch/height/round/phase, validator authority, voting power, quorum threshold, dan proposer policy sebagai recovery context;
6. ephemeral proposal/vote/transport evidence tidak menjadi authority hanya karena tersimpan atau replay;
7. LockProof/FinalityCertificate wajib divalidasi penuh bila kelak dipersist;
8. restore harus transactional dan canonical commit tetap berada di node boundary.

**Production code impact**

Tidak ada production WAL, snapshot storage, crash-recovery engine, atau canonical persistence implementation baru. Milestone ini sengaja menyelesaikan **contract + test boundary** terlebih dahulu.

**Recovery invariants locked**

1. Restore tidak boleh mencampur chain/protocol context.
2. Restore tidak boleh menurunkan round.
3. Restore tidak boleh menginventarisasi evidence ephemeral sebagai finality.
4. LockProof hanya boleh dipulihkan setelah authenticated validation.
5. FinalityCertificate hanya boleh dipulihkan setelah authenticated validation dan exact context binding.
6. Recovery failure harus atomic.
7. Replayed stale evidence tidak boleh mengubah restored runtime.
8. Consensus recovery tidak otomatis berarti canonical block/state commit.

**Verification**

- Recovery matrix: `IndoChain/internal/consensus/runtime_recovery_boundary_test.go`
- Persistence contract: `IndoChain/docs/consensus-runtime-persistence-contract-v0.1.md`
- Implementation/test/documentation current HEAD: `39d8e7b50bf3b2d7f701dff15d08935e0bf378df`
- CI #1414, run `36723489808`: verification in progress at documentation update time; final milestone gate requires this exact documentation HEAD to be GREEN.
- PostgreSQL: tidak relevan; milestone hanya consensus persistence contract/recovery boundary.

**Safety boundary**

Milestone ini **bukan production BFT completion** dan bukan durable consensus persistence. Storage engine, WAL record format, snapshot format, fsync/crash semantics, distributed recovery, peer-state persistence, validator lifecycle persistence, serta formal BFT safety/liveness analysis tetap belum didefinisikan/diimplementasikan.

**Next milestone**

**4.44 — Production Consensus Persistence Design Review:** review contract v0.1 terhadap existing storage interfaces dan canonical commit semantics, lalu buat deterministic serialization/versioning test vectors tanpa mengaktifkan production restore sebelum format dan atomicity contract disetujui oleh test matrix.

**Milestone 4.43 implementation status:** pending final exact documentation HEAD CI gate.


### 4.44 Production Consensus Persistence Design Review

**Tanggal:** 2026-09-30

**Objective**

Review persistence contract v0.1 against the actual storage interfaces, canonical node commit semantics, `RoundState`, validator authority, voting power, quorum threshold, proposer policy, LockProof, FinalityCertificate, round-local evidence, node recovery, and finalized-block handoff.

**Implementation**

1. Reviewed `IndoChain/internal/storage/storage.go`: `ChainStore.CommitBlockState` remains the canonical block/state persistence boundary.
2. Reviewed `IndoChain/internal/storage/file_store.go`: development `FileStore` uses gob as an implementation format with temp-file + sync + rename; its source explicitly states gob is not canonical protocol encoding.
3. Reviewed `IndoChain/internal/consensus/state.go`: protocol version, chain ID, epoch, height, round, and phase are explicit `RoundState` context.
4. Reviewed `ValidatorRuntime`: runtime additionally depends on validator membership, voting power, quorum threshold, proposer selector, lock state, round-local vote aggregation, and authenticated finality evidence.
5. Reviewed `StaticValidatorAuthority`: authority is an immutable public-key snapshot and must remain an authenticated recovery context.
6. Added deterministic contract-level serialization/versioning vectors in `IndoChain/internal/consensus/persistence_vectors_test.go`.
7. Expanded `IndoChain/docs/consensus-runtime-persistence-contract-v0.1.md` with the architecture review findings and explicit production-persistence boundary.

**Durable vs ephemeral boundary**

- Durable consensus context: protocol/chain/epoch/height/round/phase, validator authority/membership, voting power, quorum threshold, proposer policy/version.
- Authenticated safety evidence: LockProof and FinalityCertificate only after complete revalidation.
- Ephemeral evidence: proposal bytes, vote aggregation, transient timeout/transport evidence, peer/network state.
- Canonical blockchain state: block execution and commit owned by node/storage.

No consensus persistence object is allowed to imply canonical state commit.

**Deterministic serialization**

`persistence_vectors_test.go` verifies that the same semantic versioned consensus context produces identical serialized bytes and that serialization round-trips to the same semantic representation.

The vector includes explicit validator identity/public-key/voting-power entries, threshold, proposer policy/version, and consensus context. The SHA-256 digest is fixed as a regression vector. Initial CI #1424 exposed a test-vector digest mismatch; the failure was isolated to the expected test constant and corrected without production-code changes.

This is a **contract-level test vector**, not a production WAL/snapshot format.

**Recovery ordering**

Future restore must be transactional:

1. decode/version-check;
2. validate protocol/chain/epoch/height/round/phase;
3. validate validator authority/voting power;
4. validate threshold/proposer policy;
5. validate LockProof/FinalityCertificate when present;
6. rebuild fresh aggregators;
7. publish recovered runtime only after all validation succeeds.

Canonical block/state commit remains outside consensus recovery.

**Tests / verification**

- `IndoChain/internal/consensus/runtime_recovery_boundary_test.go`
- `IndoChain/internal/consensus/persistence_vectors_test.go`
- `IndoChain/docs/consensus-runtime-persistence-contract-v0.1.md`
- Exact implementation HEAD after vector fix: `ec65f91b33dacca89dde6989565668a6cde46bd9`.
- CI #1425, run `36724366897`: PASS on exact implementation HEAD `ec65f91b33dacca89dde6989565668a6cde46bd9`.
- PostgreSQL: tidak relevan.

**Production code impact**

Tidak ada production WAL/snapshot reader/writer, no durable consensus store, no production restore activation, dan no change to canonical node commit semantics.

**Known limitations**

- WAL record format belum didefinisikan.
- Snapshot lifecycle/version compatibility belum diaktifkan.
- Crash/fsync semantics dan durable atomicity antara consensus state dan canonical state belum didefinisikan.
- Distributed recovery coordination belum didefinisikan.
- Current proposer policy tetap development-only.
- Milestone ini tidak mengubah status production BFT.

**Safety boundary**

4.44 belum boleh dianggap production consensus durability atau production BFT completion sampai WAL/snapshot format, crash boundary, recovery ordering, and durable atomicity contract selesai dan diuji.

**Next milestone**

**4.45 — Consensus WAL/Snapshot Record Contract & Crash Boundary Matrix:** definisikan record envelope, versioning, ordering, checksum/integrity, snapshot/WAL interaction, crash cut points, stale-record rejection, context mismatch rejection, and deterministic recovery vectors sebelum production persistence implementation.

**Milestone 4.44 status:** implementation/design review completed; implementation HEAD is GREEN. Final documentation HEAD remains pending its own CI gate.


### 4.45 Consensus WAL/Snapshot Record Contract & Crash Boundary Matrix

**Tanggal:** 2026-09-30

**Objective**

Mendefinisikan record envelope, versioning, ordering, checksum/integrity, snapshot/WAL interaction, crash cut points, stale-record rejection, context mismatch rejection, dan deterministic recovery vectors sebelum production persistence implementation.

**Implementation**

- IndoChain/internal/consensus/persistence_record_contract.go — contract object untuk versioned snapshot/WAL record envelope, contiguous sequencing, consensus-context digest, dan checksum validation.
- IndoChain/internal/consensus/persistence_record_contract_test.go — deterministic checksum vector dan crash-boundary matrix.
- IndoChain/docs/consensus-runtime-persistence-contract-v0.1.md — contract diperluas dengan 4.45 record/crash boundary.

**Locked invariants**

1. First WAL record sequence harus 1.
2. Record berikutnya harus contiguous dari durable sequence sebelumnya.
3. Snapshot pada sequence N menjadi recovery base; WAL replay dimulai dari N+1.
4. Duplicate, stale, dan gapped sequence ditolak.
5. Context digest mengikat record ke protocol/chain/epoch/height/round/phase dan authority/configuration digest.
6. Payload/context corruption ditolak melalui checksum/context validation.
7. Partial append tidak dianggap record valid.
8. Canonical block/state commit tetap di node/storage boundary dan tidak tersirat dari persistence recovery.

**Production code impact**

Perubahan production hanya berupa contract types + validation primitives yang tidak melakukan file I/O dan tidak mengaktifkan WAL/snapshot persistence. Tidak ada writer/reader production, fsync policy, crash recovery engine, atau canonical commit mutation baru.

**Verification scope**

Test matrix mencakup valid first WAL, sequence gap, duplicate sequence, context mismatch, payload corruption, unsupported format version, snapshot + contiguous WAL, dan context digest change.

**Safety boundary**

Milestone ini bukan production durable consensus recovery dan bukan production BFT completion. Crash semantics terhadap filesystem/process failure masih memerlukan harness failure-injection pada milestone berikutnya.

**Next milestone**

**4.46 — Persistence Failure Injection & Recovery Harness:** uji partial write, checksum failure, context mismatch, sequence gap, snapshot/WAL replay, dan atomic recovery publication melalui persistence adapter/harness terkontrol tanpa mengaktifkan production persistence.

**Milestone 4.45 status:** implementation/design contract completed; final exact documentation HEAD wajib diverifikasi GREEN oleh CI sebelum milestone dinyatakan selesai.


**CI iteration / root cause**

- CI #1437, run 36725502329, exact HEAD c0da0d0b83552b1a65bc3bf0638ecdb2b9b2d58f: RED.
- Root cause was isolated to persistence_record_contract_test.go: the checksum-corruption fixture shallow-copied record1 and mutated the shared payload backing array, so the earlier valid-append case was unintentionally corrupted.
- No production consensus logic caused the failure.
- Fix commit 56bb1406b487324f66c20607259331a1b2d2719d isolates the corrupted payload before mutation.
- Final milestone gate remains the exact documentation HEAD after this fix and must be GREEN.


**Final verification**

- Final implementation/documentation HEAD before this verification note: 001d52f8fca9e0640a93cd822e1bfcd3d6f77c5c.
- CI #1441 / run 36725761479: GREEN.
- go mod tidy: PASS.
- go test ./...: PASS.
- go test -race ./...: PASS.
- go vet ./...: PASS.
- The final status-document commit must itself pass the exact-head CI gate before 4.45 is declared complete.


### 4.46 Persistence Failure Injection & Recovery Harness

**Tanggal:** 2026-09-30

**Objective**

Menguji contract 4.45 melalui controlled in-memory persistence adapter/harness tanpa mengaktifkan production WAL/snapshot persistence.

**Implementation**

- IndoChain/internal/consensus/persistence_failure_injection_test.go
- Harness mensimulasikan failure sebelum append, partial write, checksum corruption, context mismatch, sequence gap, snapshot + WAL replay, stale WAL terhadap snapshot, dan atomic recovery publication.
- Recovery dibangun ke candidate state terlebih dahulu dan baru dipublish setelah seluruh validation berhasil.

**Verification**

- CI #1448 / run 36726341648
- Exact implementation HEAD 2eefbca3de224ea83a718086795965287be432fd
- CI GREEN / success.
- Final documentation HEAD tetap wajib melewati CI exact-head sebelum milestone dinyatakan selesai.

**Production boundary**

Tidak ada production WAL writer/reader, filesystem persistence activation, fsync policy, process-crash recovery, distributed recovery, atau canonical state mutation baru.

**Known limitations**

Harness masih in-memory dan tidak menguji actual filesystem/process crash. Atomicity terhadap canonical ChainStore dan durable storage tetap belum teraktivasi.

**Next milestone**

**4.47 — Persistence Adapter Boundary & Canonical Commit Coordination Contract:** definisikan interface boundary antara consensus recovery/persistence dan canonical ChainStore commit, termasuk ordering, failure ownership, and no-partial-publication invariants, tanpa mengaktifkan production WAL.

**Milestone 4.46 status:** implementation/test completed and final documentation HEAD verified GREEN. Final docs HEAD `adab8d74094c55e14940da54b0434acac7ba0fd8`; CI #1452 / run `36726558485`: GREEN.


### 4.47 Persistence Adapter Boundary & Canonical Commit Coordination Contract

**Tanggal:** 2026-09-30

**Objective**

Mendefinisikan interface boundary antara consensus/recovery persistence dan canonical ChainStore commit, termasuk ordering, failure ownership, dan no-partial-publication invariants tanpa mengaktifkan production WAL/snapshot persistence.

**Implementation**

- `IndoChain/internal/consensus/canonical_commit_boundary.go`
  - `CanonicalCommitCandidate` membawa block, block hash, dan resulting state snapshot.
  - `CanonicalCommitter` menjadi narrow handoff interface yang kompatibel secara struktural dengan `storage.ChainStore.CommitBlockState`.
  - `ValidateCanonicalCommitCandidate` memverifikasi non-nil state, non-zero hash, deterministic block-hash binding, dan optional state-root binding.
  - `CommitCanonicalCandidate` memastikan validation terjadi sebelum store call dan canonical commit dipanggil tepat satu kali; commit error tidak diperlakukan sebagai successful publication.
- `IndoChain/internal/consensus/canonical_commit_boundary_test.go`
  - regression coverage untuk valid candidate, hash mismatch, state-root mismatch, validation-before-store, exactly-one commit, commit failure propagation, dan nil committer.
- `IndoChain/docs/consensus-persistence-canonical-commit-boundary-v0.1.md`
  - mendokumentasikan ordering, failure ownership, no-partial-publication, scope, dan known limitations.

**Locked invariants**

1. Recovery/execution harus menghasilkan candidate sebelum canonical commit.
2. Candidate hash harus cocok dengan deterministic block hash.
3. Optional non-zero block state root harus cocok dengan candidate state root.
4. Invalid candidate tidak boleh memanggil canonical store.
5. Successful handoff memanggil canonical commit tepat satu kali.
6. Commit failure tidak boleh dianggap sebagai successful consensus/runtime publication.
7. Runtime/node state hanya boleh dipublish setelah canonical commit sukses.
8. Coordination layer tidak melakukan retry/resubmission otomatis.

**Production code impact**

Milestone ini menambahkan contract/validation boundary pada package consensus. Tidak ada WAL writer/reader, snapshot persistence, filesystem crash recovery, fsync policy, automatic retry, atau canonical state mutation baru.

**Verification scope**

Test boundary menggunakan fake in-memory committer. Test tidak mengaktifkan production persistence dan tidak mengklaim durable filesystem atomicity.

**Known limitations**

- `ChainStore.CommitBlockState` tetap menjadi canonical storage atomicity boundary.
- Contract ini tidak membuktikan filesystem/process crash atomicity.
- MemoryStore/FileStore belum menjadi transactional durable persistence engine.
- Production BFT tetap belum selesai.

**Next milestone**

**4.48 — Canonical Commit Failure-Atomicity Regression Matrix:** perluas regression matrix di node/storage boundary untuk memverifikasi bahwa canonical commit failure tidak memajukan head/state dan tidak menghasilkan partial publication pada implementasi store yang diuji, tetap tanpa mengaktifkan WAL production.

**Milestone 4.47 status:** implementation/design completed and implementation/test HEAD verified GREEN. Implementation/test HEAD `839a624ffcf00ded714d25b61711752c78e1e865`; CI #1465 / run `36728091994`: GREEN. Final documentation HEAD still requires its own exact-head CI gate.


### 4.48 Canonical Commit Failure-Atomicity Regression Matrix

**Tanggal:** 2026-09-30

**Objective**

Memverifikasi bahwa kegagalan pada canonical `ChainStore.CommitBlockState` tidak memajukan canonical head/state dan tidak mempublikasikan partial snapshot pada `FileStore` yang sedang digunakan.

**Implementation**

- `IndoChain/internal/storage/commit_failure_atomicity_test.go`
  - `FileStore`: successful initial commit, injected rename failure, in-memory head/state unchanged, persisted snapshot unchanged after reopen.
  - `MemoryStore`: regression untuk successful block/state publication.
- `IndoChain/docs/consensus-canonical-commit-failure-atomicity-v0.1.md`
  - regression matrix, atomicity ordering, scope, relationship dengan 4.47, dan limitations.

**Locked invariants**

1. Candidate snapshot dibangun sebelum publication.
2. `FileStore.data` hanya dipublish setelah temporary file berhasil di-sync, close, dan atomic rename.
3. Rename failure tidak boleh mengubah in-memory canonical head/state.
4. Rename failure tidak boleh mengubah persisted canonical snapshot.
5. Canonical commit failure tetap merupakan error dan tidak boleh diperlakukan sebagai successful publication.

**Production code impact**

Tidak ada perubahan production storage behavior. Milestone ini menambah regression coverage terhadap implementasi `FileStore` yang sudah ada dan tidak mengaktifkan WAL/snapshot recovery production.

**Verification**

Implementation/test HEAD: `c066032081824fceb6bf9997c21c02891bd557a5`.

CI #1477 / run `36732898863`: **GREEN**.

`go mod tidy` ✅  
`go test ./...` ✅  
`go test -race ./...` dan `go vet ./...` juga wajib pada final documentation gate; final documentation HEAD telah diverifikasi melalui CI #1481 / run `36733106037`.

**Known limitations**

- Failure injection menggunakan test-only redirect terhadap file target; tidak menambahkan fault injection API ke production.
- `MemoryStore` belum memiliki injectable failure point setelah block publication dan sebelum state publication.
- Process-crash/disk-power-loss semantics belum diuji.
- WAL/snapshot recovery production belum diaktifkan.
- Production BFT tetap belum selesai.

**Next milestone**

**4.49 — Canonical Commit ↔ Consensus Publication Ordering Regression:** uji ordering end-to-end pada boundary node agar consensus/runtime state tidak maju sebelum canonical storage commit berhasil, tetap tanpa mengaktifkan WAL production.

**Milestone 4.48 status:** completed. Final documentation HEAD `794fc2a737fa060b8a5510e5f591cd21a00f73db`; CI #1481 / run `36733106037`: GREEN (Tidy/Test/Race/Vet).


### 4.49 Canonical Commit ↔ Consensus Publication Ordering Regression

**Tanggal:** 2026-09-30

**Objective**

Memastikan authenticated consensus finality evidence tidak otomatis memajukan canonical node state, dan canonical Head/HeadHash/State hanya dipublish setelah CommitBlockState berhasil.

**Implementation**

- IndoChain/internal/node/node_test.go
  - TestCommitRuntimeFinalizedBlockStoreFailureDoesNotPublishCanonicalNodeState
  - TestCommitRuntimeFinalizedBlockPublishesCanonicalStateOnlyAfterSuccessfulCommit
- IndoChain/docs/consensus-canonical-consensus-publication-ordering-v0.1.md
  - ordering contract, regression matrix, production boundary, dan known limitations.

**Locked invariants**

1. Finality evidence dan canonical publication adalah dua boundary berbeda.
2. Node canonical head/state tidak boleh maju sebelum storage commit sukses.
3. Storage head/state tidak boleh maju ketika commit gagal.
4. Successful commit membuat node publication konvergen dengan canonical storage.
5. Tidak ada automatic retry/resubmission dari coordination boundary.

**Production code impact**

Tidak ada perubahan production consensus/storage behavior pada milestone ini; coverage ditambahkan untuk mengunci ordering yang sudah diimplementasikan pada CommitFinalizedBlock/ImportBlockWithAuthority.

**Verification**

Implementation commit: 75daf849b52466b30e1e78f15a8485f1b66fa288.

CI #1489 / run 36734412117: GREEN / success. Tidy, Test, Race Test, dan Vet semuanya PASS pada exact documentation HEAD.

**Next milestone**

4.49 completed after exact-head CI gate. Next milestone ditentukan setelah review status branch berikutnya.

### 4.50 Finalized Block Replay & Stale Context Regression

**Tanggal:** 2026-09-30

**Objective**

Memastikan finalized block replay dan stale consensus context tidak dapat melewati canonical node/storage validation boundary.

**Implementation**

- `IndoChain/internal/node/node_test.go`
  - `TestCommitFinalizedBlockRejectsAlreadyCommittedBlockWithoutMutation`
  - `TestCommitFinalizedBlockRejectsDifferentBlockAtCommittedHeightWithoutMutation`
  - `TestCommitFinalizedBlockRejectsStaleContextWithoutStorageMutation`
- `IndoChain/docs/consensus-finalized-replay-stale-context-v0.1.md`

**Locked invariants**

1. Exact finalized replay ditolak tanpa mutation.
2. Different block pada committed height tidak bypass context validation.
3. Stale context ditolak sebelum storage mutation.
4. Rejection tidak mengubah canonical node/storage state.
5. Tidak ada automatic retry/resubmission.

**Production code impact**

Tidak ada perubahan production behavior; milestone ini menambah regression coverage.

**Verification**

Implementation commit: `5d4a9518eb9aac7a55bb394ed9734a1fdcd415fe`.

Exact-head CI gate: pending.

**Next milestone**

4.50 implementation/test gate GREEN; final documentation HEAD masih memerlukan exact-head CI gate tersendiri.

### 4.51 Authenticated Consensus Round Driver Integration

**Tanggal:** 2026-10-01

**Objective**

Mengintegrasikan primitive BFT yang sudah ada menjadi executable orchestration boundary yang melakukan authenticated message ingress dan deterministic timeout/round-change coordination, tanpa menambahkan scheduler semu atau memindahkan authority dari ValidatorRuntime.

**Implementation**

- `IndoChain/internal/consensus/authenticated_runtime.go`
  - `AcceptAuthenticatedProposal` memvalidasi exact consensus context, validator membership, dan signature authority sebelum proposal masuk ke runtime.
  - `AddAuthenticatedVote` memvalidasi exact context, validator membership, dan signature authority sebelum prevote/precommit masuk ke round-local aggregation.
  - API development `AcceptProposal` / `AddVote` dipertahankan untuk compatibility; authenticated path menjadi boundary yang dipakai driver.
- `IndoChain/internal/consensus/round_driver.go`
  - `RoundDriver` menjadi event-driven orchestration boundary di atas `ValidatorRuntime`.
  - Proposal, prevote, dan precommit diroute melalui exact-context + signature validation.
  - Timeout message dikumpulkan tanpa langsung memajukan round.
  - Round transition hanya terjadi melalui explicit `AdvanceRoundFromTimeoutEvidence`.
  - Timeout queue hanya dibersihkan setelah transition sukses; failed batch tetap tersedia untuk recovery/discard.
  - Driver tidak memiliki clock, peer discovery, retransmission, failure detector, atau canonical storage mutation.
- `IndoChain/internal/consensus/round_driver_test.go`
  - unsigned vote rejection sebelum aggregation;
  - tampered vote rejection sebelum aggregation;
  - explicit timeout batch boundary;
  - atomic conflicting-lock timeout rejection.

**Locked invariants**

1. Authenticated proposal/vote/timeout harus exact-context dan berasal dari validator yang terdaftar.
2. Signature failure tidak boleh mencapai vote aggregation.
3. Timeout collection tidak boleh memajukan round secara implicit.
4. Failed timeout batch tidak boleh memutasi runtime dan tidak boleh diam-diam hilang.
5. Successful timeout transition membersihkan evidence round-local melalui runtime.
6. Runtime tetap menjadi owner phase, quorum, lock, finality, dan state transition.
7. Driver tidak melakukan canonical block/state commit.

**Production boundary**

Milestone ini merupakan integrasi BFT orchestration yang nyata, tetapi **bukan production BFT completion**. Clock/scheduler, network-wide peer/gossip/retransmission, validator-set lifecycle, production proposer policy, durable consensus recovery, end-to-end block-production loop, dan formal BFT safety/liveness analysis masih terbuka.

**Verification**

- Implementation HEAD: `ee5864dc3bf996083404430759f9a8285bfb7a83`
- Exact-head CI #1510 / run `36789565991`: GREEN.
- Tidy: PASS.
- `go test ./...`: PASS.
- `go test -race ./...`: PASS.
- `go vet ./...`: PASS.
- Final documentation HEAD `0a400b18ff436e478ca9e06fb959e71890a3e125`: GREEN pada CI run `36789673826`.
- PostgreSQL: tidak relevan.

**Next integration target**

Bind `RoundDriver` ke existing P2P consensus transport dan block-candidate production sehingga pipeline proposal/prevote/precommit/timeout/finality berjalan melalui transport nyata, dengan canonical execution/commit tetap berada di node boundary.

**Milestone 4.51 status:** implementation completed; exact implementation HEAD `e6d0b7c9473715bf8af28bce83490be2a22c3a7e` initially RED due to a test asserting an unsigned message could cross a transport encoder that correctly rejects missing signatures. The test was corrected in `e256011e985214e097891c93b857adf616de5559`. Exact implementation CI run `36790356111`: GREEN (Tidy/Test/Race/Vet).

### 4.52 P2P Consensus Round Driver & Proposal Publication Boundary

**Tanggal:** 2026-10-01

**Objective**

Mengikat authenticated `RoundDriver` ke transport P2P yang sudah tersedia dan menyediakan publication boundary untuk deterministic block proposal evidence tanpa membekukan canonical block wire serialization terlalu dini.

**Implementation**

- `IndoChain/internal/consensus/proposal_signing.go`
  - `BuildSignedProposalMessage` memvalidasi `BlockProposal`, membentuk exact protocol/chain/epoch/height/round proposal message, dan menandatanganinya melalui consensus domain.
  - Candidate tetap terpisah dari consensus evidence karena canonical block serialization v0.1 belum frozen.
- `IndoChain/internal/p2p/consensus_transport.go`
  - generalisasi `SendConsensus` / `ReceiveConsensus` agar bekerja pada `Transport` interface, bukan hanya `InMemoryTransport`.
  - framing/decoding consensus message tetap menggunakan existing consensus codec dan transport payload limit.
- `IndoChain/internal/p2p/consensus_round_driver.go`
  - `ConsensusRoundDriver` mengikat P2P transport ke authenticated `consensus.RoundDriver`.
  - `Publish` mengirim authenticated consensus evidence.
  - `PublishBlockProposal` membangun + menandatangani deterministic proposal evidence sebelum publish.
  - `ReceiveAndHandle` menerima tepat satu consensus message dan menyerahkannya ke authenticated runtime boundary.
  - Driver tidak memiliki clock, retransmission, peer discovery, atau canonical storage ownership.
- `IndoChain/internal/p2p/consensus_round_driver_test.go`
  - remote proposal authenticated sebelum runtime mutation;
  - tampered signature ditolak tanpa mutation;
  - unsigned message ditolak sebelum transport enqueue;
  - proposal signing boundary memiliki explicit signer requirement.

**Locked invariants**

1. Consensus message tetap exact-context sebelum runtime handling.
2. Signature wajib tersedia sebelum consensus message dapat melewati transport encoder.
3. Receiver tetap melakukan authority/signature verification; transport encoding bukan pengganti authentication.
4. Tampered proposal tidak boleh memajukan round state.
5. Proposal evidence dibangun dari deterministic block candidate hash.
6. Canonical block serialization/wire encoding tidak dibekukan oleh milestone ini.
7. P2P driver tidak melakukan canonical execution atau storage commit.

**Production boundary**

Milestone ini sudah menjadi transport/runtime integration boundary yang executable, tetapi belum merupakan full multi-node production consensus loop.

Masih terbuka:

- candidate block dissemination/fetch melalui canonical block/sync protocol;
- local proposer loop dari mempool ke candidate;
- automatic prevote/precommit emission policy;
- peer-wide broadcast/retransmission;
- timeout clock/failure detector;
- multi-node multi-height commit loop;
- validator-set lifecycle;
- durable consensus recovery activation.

**Verification**

- Initial implementation HEAD: `e6d0b7c9473715bf8af28bce83490be2a22c3a7e` — CI RED due to test-boundary assertion.
- Corrected implementation HEAD: `e256011e985214e097891c93b857adf616de5559`.
- CI run `36790356111`: GREEN.
- Tidy: PASS.
- `go test ./...`: PASS.
- `go test -race ./...`: PASS.
- `go vet ./...`: PASS.

**Next integration target**

Candidate dissemination/fetch harus diikat ke existing block/sync transport sehingga node penerima dapat memperoleh candidate block berdasarkan proposal hash, memvalidasi candidate terhadap consensus context, lalu menjalankan authenticated prevote/precommit melalui `ConsensusRoundDriver`.

**Milestone 4.52 status:** implementation/test completed; final documentation HEAD requires exact-head CI verification.


### 4.53 Consensus Proposal ↔ Candidate Block Dissemination/Fetch Boundary

**Tanggal:** 2026-10-01

**Objective**

Mengikat proposal consensus yang sudah authenticated ke candidate block nyata melalui existing P2P block/sync message types. Receiver harus dapat meminta block pada height proposal, menerima development-encoded candidate, lalu memverifikasi deterministic block hash terhadap proposal payload sebelum candidate dipakai lebih lanjut.

**Implementation**

- `IndoChain/internal/p2p/block_codec.go`
  - menambahkan development-only block/candidate codec untuk `block.Block` dan `BlockResponse`;
  - mempertahankan header, proposer, consensus evidence, dan signed `transaction.Transaction`;
  - round-trip verification mempertahankan deterministic block hash;
  - codec secara eksplisit bukan canonical block serialization.
- `IndoChain/internal/p2p/candidate_exchange.go`
  - `CandidateExchange.PublishCandidate` mengirim candidate melalui existing `MessageTypeBlock`;
  - `ServeOneRequest` memakai existing `BlockRequest` + `SyncReader` dan mengembalikan `BlockResponse`;
  - `FetchCandidate` meminta block pada proposal height dan menolak candidate bila height atau deterministic block hash tidak cocok dengan proposal payload.
- `IndoChain/internal/p2p/consensus_round_driver.go`
  - `NewConsensusRoundDriverWithCandidateExchange` menggabungkan authenticated consensus driver dengan candidate exchange;
  - `PublishBlockProposalAndCandidate` mengirim signed proposal evidence lalu complete development candidate;
  - `FetchProposalCandidate` mengikat candidate hasil fetch ke proposal hash;
  - `ServeCandidateRequest` menjadi boundary untuk existing sync request handling.
- Tests:
  - development block codec round-trip;
  - candidate fetch/hash binding melalui in-memory P2P boundary;
  - existing consensus runtime authentication tetap menjadi authority boundary.

**Locked invariants**

1. Proposal payload tetap menjadi deterministic block-hash binding.
2. Candidate fetch dibatasi ke proposal height.
3. Candidate dengan hash berbeda dari authenticated proposal ditolak.
4. Candidate codec tidak mengubah canonical block hash algorithm atau mengklaim canonical serialization freeze.
5. Candidate exchange tidak melakukan canonical execution, state publication, atau storage commit.
6. Existing sync reader tetap menjadi source untuk block retrieval; exchange tidak membuat second canonical block store.
7. Request/response handling tetap bounded oleh existing payload/request limits.

**Production boundary**

Milestone ini menutup gap antara authenticated proposal evidence dan block candidate retrieval pada executable P2P boundary. Ini **belum full production multi-node BFT loop**.

Masih terbuka:

- multiplexing request/response dengan concurrent consensus traffic;
- automatic proposer loop dari mempool;
- automatic prevote/precommit policy setelah candidate validation;
- peer-wide broadcast/retransmission;
- timeout clock/failure detector;
- multi-node multi-height canonical commit loop;
- validator-set lifecycle;
- durable consensus recovery;
- final canonical serialization freeze.

**Verification**

- Initial implementation HEAD `b399f3086b3f16da628ba85b2a927ec49c2bf499`: CI RED karena duplicate package error `ErrUnexpectedSyncMessage` pada candidate exchange.
- Root cause diperbaiki dengan memakai existing sync error declaration.
- Corrected implementation HEAD `ca16643bc211ebada60ff57764fcde2e647b59f`.
- Exact-head CI run `36790985190`: **GREEN**.
- Tidy: PASS.
- `go test ./...`: PASS.
- `go test -race ./...`: PASS.
- `go vet ./...`: PASS.
- Additional exact-head CI run `36790979322`: **GREEN**.
- PostgreSQL: tidak relevan.

**Next integration target**

Bind fetched candidate ke consensus proposal acceptance sehingga receiver tidak hanya memiliki block/hash binding, tetapi dapat menjalankan candidate validation/execution policy sebelum authenticated prevote emission.

**Milestone 4.53 status:** implementation/test completed; exact-head CI gate GREEN.


### 4.54 Candidate Execution Validation → Authenticated Prevote Boundary

**Tanggal:** 2026-10-01

**Objective**

Memastikan candidate block yang diperoleh dari proposal P2P tidak cukup hanya hash-match. Sebelum authenticated proposal diterima dan runtime masuk ke Prevote, candidate harus lolos consensus-context validation dan deterministic block execution terhadap snapshot canonical state.

**Implementation**

- `IndoChain/internal/consensus/candidate_validation.go`
  - `ValidateBlockCandidateForConsensus` memvalidasi block-production context, menjalankan candidate melalui existing `block.ExecuteBlock` pada state snapshot, dan memverifikasi execution result/state root tanpa memutasi canonical state.
  - `ValidateAuthenticatedBlockProposal` menggabungkan candidate validation dengan existing signature/validator-authority boundary sebelum memanggil `AcceptAuthenticatedProposal`.
  - `ValidateBlockCandidateContext` mengikat proposal message, height/round/context, previous hash, chain/protocol, dan proposer identity.
- `IndoChain/internal/p2p/consensus_round_driver.go`
  - `AcceptFetchedBlockProposal` menjadi P2P-to-consensus boundary: fetched candidate harus lolos execution validation terlebih dahulu sebelum runtime menerima authenticated proposal.
- `IndoChain/internal/consensus/round_driver.go`
  - menambahkan read-only `Authority()` accessor untuk meneruskan authority resolver yang sama ke authenticated candidate path.
- Tests:
  - valid candidate dieksekusi pada snapshot dan canonical state tetap identik;
  - state-root mismatch ditolak tanpa canonical mutation;
  - proposal/candidate context mismatch ditolak.

**Locked invariants**

1. Proposal signature/validator authority tetap wajib valid.
2. Candidate harus cocok dengan exact proposal consensus context.
3. Candidate harus cocok dengan expected proposer.
4. Candidate harus memenuhi `TransactionsRoot`.
5. Candidate harus dapat dieksekusi secara deterministic menggunakan existing block execution rules.
6. Non-zero `StateRoot` harus cocok dengan hasil execution.
7. Candidate hash harus cocok dengan authenticated proposal payload.
8. Canonical node state tidak boleh dimutasi selama prevote admission.
9. Hanya setelah seluruh validation berhasil runtime boleh maju dari Proposal ke Prevote.

**Production boundary**

Milestone ini memperkecil gap antara consensus evidence dan executable block validity. Namun ini **belum full production BFT** dan belum melakukan canonical commit pada prevote admission.

Masih terbuka:

- automatic local proposer loop dari mempool;
- automatic local prevote/precommit emission policy;
- concurrent request/response multiplexing;
- peer-wide broadcast/retransmission;
- timeout clock/failure detector;
- multi-node multi-height canonical commit loop;
- validator-set lifecycle;
- durable consensus recovery;
- final canonical serialization freeze.

**Verification**

- Implementation exact HEAD: `49bae7485ebc3e817ecafa759ba88b2db56cd581`.
- CI run `36792081645`: **GREEN** — Tidy/Test/Race/Vet PASS.
- Additional exact-head CI run `36792084200`: **GREEN** — Tidy/Test/Race/Vet PASS.
- PostgreSQL: tidak relevan.

**Next integration target**

Masuk ke **local proposer/prevote emission boundary**: gunakan deterministic mempool snapshot + block candidate builder untuk menghasilkan proposal lokal, lalu emit authenticated prevote hanya setelah candidate validation berhasil. Tetap tanpa scheduler/clock produksi dan tanpa canonical commit pada fase prevote.

**Milestone 4.54 status:** implementation/test completed; exact implementation HEAD CI GREEN.
