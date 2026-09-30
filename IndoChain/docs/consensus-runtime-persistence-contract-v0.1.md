# IndoChain Consensus Runtime Persistence Contract v0.1

## Status

Design boundary only. This document defines what a future consensus persistence layer must preserve and what it must never infer. It does **not** implement WAL, snapshots, storage, or recovery in production.

## Scope

The contract covers the in-memory state owned by `ValidatorRuntime`:

- protocol/chain/epoch/height/round/phase;
- validator authority and voting-power context;
- current proposal;
- prevote/precommit aggregation;
- locked proposal and locked round;
- authenticated `LockProof`;
- `FinalityCertificate`.

Canonical block/state execution remains a node/storage concern and is not moved into consensus persistence.

## Recovery classes

### Durable consensus context

A future snapshot/WAL format must explicitly identify:

1. protocol version;
2. chain ID;
3. epoch;
4. height;
5. round;
6. phase;
7. validator membership/authority identity;
8. voting-power configuration;
9. quorum threshold;
10. proposer-selection policy/version.

Each item must be versioned and validated against the active chain context before restore.

### Ephemeral evidence

The following must not be restored merely because a `RoundState` was restored:

- current proposal bytes;
- prevote aggregation;
- precommit aggregation;
- uncommitted timeout messages;
- transient transport ordering;
- transient peer/network state.

If a future recovery design persists these items for replay optimization, they must be represented as authenticated evidence with explicit height/round/context binding and independently revalidated before affecting runtime state.

### Locked/finality evidence

`lockedProposal`, `lockedRound`, `LockProof`, and `FinalityCertificate` are safety-critical authenticated state. A future persistence implementation must either:

- persist the complete authenticated artifact and revalidate it against authority, membership, voting power, chain, epoch, height, round, proposal and threshold; or
- restore the runtime without the artifact and require explicit authenticated evidence replay.

It must never reconstruct a lock or finality certificate from an unauthenticated summary.

## Atomic restore

Restore must be transactional:

1. decode/version-check the candidate state;
2. validate chain/epoch/height/round context;
3. validate validator authority and voting power;
4. validate threshold;
5. validate any persisted LockProof/FinalityCertificate;
6. construct fresh runtime aggregators;
7. publish the recovered runtime only after every validation succeeds.

Any failure must leave the pre-recovery runtime/canonical node state unchanged.

## Replay and anti-replay

Recovery must reject:

- lower-height evidence;
- cross-chain or cross-epoch evidence;
- stale round evidence when the restored round is newer;
- duplicate validator evidence that changes quorum accounting;
- conflicting equal-round locks;
- finality evidence without authenticated precommit signatures.

A recovery implementation must not use transport arrival order as an authority signal.

## Canonical commit boundary

Consensus recovery does not imply canonical block/state commit. A restored finality certificate must still pass the existing finalized-block validation and node commit boundary before canonical state changes.

## Deterministic recovery matrix

The required test matrix is:

| Case | Expected result |
|---|---|
| exact state restore | same protocol context and round |
| protocol mismatch | reject atomically |
| chain mismatch | reject atomically |
| height replay | reject stale/cross-height evidence |
| round regression | reject |
| proposal-only restore | no implicit finality |
| precommit evidence absent | finality impossible |
| authenticated LockProof restore | validate before adoption |
| conflicting LockProof | reject without mutation |
| stale TimeoutCertificate | reject without round regression |
| tampered signature | reject without mutation |
| duplicate evidence | reject/deduplicate without extra voting power |
| finalized certificate | validate before node handoff |
| canonical commit failure | no partial canonical mutation |

## Non-goals for v0.1

This contract does not define:

- storage engine;
- WAL record binary format;
- snapshot file format;
- crash-consistency/fsync policy;
- distributed recovery protocol;
- peer state persistence;
- validator registration/lifecycle persistence;
- production BFT liveness mechanism.

Those require separate protocol decisions and tests.

## Current implementation boundary

As of milestone 4.43, production runtime reconstruction uses existing constructor inputs and intentionally restores only state/configuration that the current API can validate. Ephemeral proposal/vote evidence is not implicitly restored. Production durable consensus persistence remains unimplemented.


## 4.44 Design Review Findings

### Storage compatibility

The existing `storage.ChainStore` owns canonical block/state persistence through `CommitBlockState`. This boundary is separate from `ValidatorRuntime` state and must remain so.

The development `FileStore` uses a gob-encoded `fileSnapshot` and atomically replaces the file through temporary-file write, sync, close, and rename. Its own source documentation states that gob is an implementation format, not canonical protocol encoding. Therefore consensus persistence must not reuse gob as a protocol serialization format.

### Consensus context compatibility

`RoundState` already carries protocol version, chain ID, epoch, height, round, and phase. `ValidatorRuntime` additionally depends on immutable validator membership, voting power, quorum threshold, and a `ProposerSelector`.

The current proposer implementation is `RoundRobinProposer`, explicitly a deterministic development policy. A persisted context therefore records a policy/version identifier rather than pretending that the current selector is a production proposer policy.

Validator authority is currently represented through `StaticValidatorAuthority` as an immutable public-key snapshot. A future durable representation must serialize validator identity plus the authoritative public key and revalidate it before accepting authenticated evidence.

### Durable vs ephemeral boundary

The review confirms:

- **durable consensus context:** protocol/chain/epoch/height/round/phase, validator membership/authority, voting power, quorum threshold, proposer policy/version;
- **authenticated safety evidence:** LockProof and FinalityCertificate, only after complete context/signature/quorum validation;
- **ephemeral evidence:** proposal bytes, vote aggregation, transient timeout messages, transport ordering, peer/network state;
- **canonical state:** block/state execution and commit owned by node/storage.

Restoring a `RoundState` alone must therefore never imply restoration of proposal, votes, lock, or finality.

### Serialization/versioning boundary

Milestone 4.44 adds contract-level deterministic serialization vectors in `IndoChain/internal/consensus/persistence_vectors_test.go`.

The vector uses a versioned, explicit field schema with ordered validator entries and a deterministic SHA-256 test vector. It verifies:

1. identical semantic context produces identical serialized bytes;
2. serialized context round-trips to the same semantic representation.

This is a **test vector for the persistence contract**, not yet a production wire/storage format. No WAL or snapshot reader/writer is introduced by this milestone.

### Atomic recovery ordering

The required future recovery ordering is:

1. decode/version-check;
2. validate protocol/chain/epoch/height/round/phase;
3. validate validator authority and voting power;
4. validate quorum threshold and proposer policy/version;
5. validate persisted LockProof/FinalityCertificate, if present;
6. rebuild fresh runtime aggregators;
7. publish recovered runtime only after all checks succeed.

Canonical block/state commit remains outside this sequence.

### Review decision

The contract is sufficiently explicit for deterministic serialization test vectors, but **not yet sufficient to activate production WAL/snapshot recovery**. Crash/fsync semantics, record format, snapshot lifecycle, WAL ordering, durable atomicity across consensus and canonical state, and distributed recovery coordination still require a separate design step.



## 4.45 WAL/Snapshot Record Contract & Crash Boundary Matrix

Milestone 4.45 defines the persistence record boundary without activating a production WAL/snapshot writer or reader.

### Record envelope

IndoChain/internal/consensus/persistence_record_contract.go defines a versioned record envelope containing:

- format version;
- record type (snapshot or WAL);
- strictly ordered sequence number;
- exact consensus-context digest;
- opaque payload bytes;
- SHA-256 integrity checksum over the canonical envelope fields.

The envelope is a contract object only. It does not perform file I/O, fsync, rename, log rotation, or recovery activation.

### Ordering contract

- The first WAL record must use sequence 1.
- Subsequent records must advance exactly by one from the last durable sequence.
- A snapshot establishes a durable sequence boundary and is followed by contiguous WAL records.
- Duplicate, stale, or gapped sequences are rejected.
- Record validation is bound to the expected consensus-context digest.

### Integrity and context binding

The checksum covers format version, record type, sequence, context digest, payload length, and payload bytes. Mutating payload or context without recomputing the checksum is rejected.

The context digest is derived from a deterministic binary representation of:

- protocol version;
- chain ID;
- epoch;
- height;
- round;
- phase;
- validator authority digest;
- voting-power digest;
- quorum threshold;
- proposer policy and policy version.

A record from another protocol/chain/epoch/height/round context therefore cannot be accepted merely because its sequence number is newer.

### Snapshot/WAL interaction

The contract treats a snapshot at sequence N as the recovery base. WAL replay starts at N+1 and must remain contiguous. No WAL record at or below the snapshot base may be replayed as new state.

This milestone does not define snapshot file layout, compaction, retention, or garbage collection.

### Crash boundary matrix

| Cut point | Required recovery result |
|---|---|
| before append | previous durable sequence remains authoritative |
| complete record append | record may become eligible after integrity/context validation |
| partial record append | partial bytes are not a valid record |
| checksum corruption | reject record; do not advance durable sequence |
| context mismatch | reject record; do not mutate recovered runtime |
| sequence gap | reject replay; do not infer missing records |
| duplicate/stale sequence | reject replay |
| snapshot + contiguous WAL | restore snapshot, then replay WAL strictly in order |
| canonical commit failure | consensus recovery must not imply partial canonical state mutation |

### Deterministic vectors

IndoChain/internal/consensus/persistence_record_contract_test.go covers:

- deterministic checksum regression vector;
- valid first WAL record;
- sequence gap;
- duplicate sequence;
- context mismatch;
- payload corruption;
- unsupported format version;
- snapshot followed by contiguous WAL;
- context-digest change detection.

These are contract-level vectors. They do not establish durable crash recovery until a storage implementation exists and is tested against actual filesystem/process crash behavior.

### Safety boundary

4.45 does not activate production persistence. It does not implement:

- WAL file I/O;
- snapshot file I/O;
- fsync or directory durability policy;
- atomic cross-file consensus/canonical-state commit;
- process crash recovery;
- distributed recovery;
- validator lifecycle persistence;
- production BFT scheduler/liveness.

The existing node/storage boundary remains the owner of canonical block/state commit.

### Next milestone

**4.46 — Persistence Failure Injection & Recovery Harness:** exercise the 4.45 contract against a controlled persistence adapter/harness, including partial writes, checksum failures, context mismatch, sequence gaps, snapshot/WAL replay, and atomic recovery publication, while keeping production activation disabled until the crash semantics are explicitly tested.


## 4.46 Persistence Failure Injection & Recovery Harness

4.46 exercises the 4.45 persistence contract through a controlled in-memory failure-injection adapter. The harness is test-only and does not introduce production WAL/snapshot I/O.

### Failure points

The harness covers:

- failure before append: durable sequence remains unchanged;
- partial write: no partial record becomes durable;
- checksum corruption: recovery rejects the corrupted record;
- context mismatch: recovery rejects the record against the expected consensus context;
- sequence gap: replay rejects missing sequence continuity;
- snapshot followed by contiguous WAL: recovery replays the snapshot base and subsequent WAL records in order;
- WAL at or before the snapshot base: replay rejects stale sequence;
- failed recovery publication: target state remains unchanged until full validation succeeds.

### Atomic publication rule

Recovery is constructed into a candidate state first. The candidate is assigned to the target only after all records have passed format, sequence, context, and checksum validation. This keeps recovery failure from partially publishing runtime state.

### Scope

The adapter is intentionally in-memory and controlled. It does not claim filesystem durability, process-crash semantics, fsync behavior, directory durability, cross-file atomicity, or production recovery activation.

The next milestone can therefore focus on defining the persistence adapter boundary and failure semantics without conflating test harness behavior with durable storage guarantees.
