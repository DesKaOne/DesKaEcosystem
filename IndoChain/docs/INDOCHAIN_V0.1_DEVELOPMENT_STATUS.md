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

CI gate for this milestone must be verified against the latest timeout-message test commit before this status is considered complete.
