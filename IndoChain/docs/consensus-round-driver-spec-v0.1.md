# IndoChain v0.1 — Production Round Driver Specification

## Scope

This document defines the orchestration contract required around the existing deterministic consensus runtime before a production timeout scheduler or network-wide round driver is introduced.

The specification is intentionally an orchestration boundary. It does not freeze a production proposer algorithm, validator lifecycle, network failure detector, persistence format, or BFT safety/liveness proof.

## State ownership

The consensus runtime remains the authoritative owner of:

- protocol/chain/epoch/height/round context;
- current proposal payload;
- prevote and precommit evidence;
- locked proposal and lock round;
- authenticated LockProof;
- authenticated finality certificate;
- deterministic round transition and timeout-evidence validation.

The round driver must not mutate these fields directly.

## Driver input/output contract

For each local consensus round, the driver coordinates these events:

1. determine the expected proposer through the configured proposer policy;
2. receive or construct a proposal for the exact current context;
3. submit the proposal to ValidatorRuntime.AcceptProposal or AcceptBlockProposal;
4. collect prevote evidence and submit it through AddVote;
5. once the runtime enters PhasePrecommit, collect explicit precommit evidence;
6. finalize only through FinalizeProposal with authenticated validator authority;
7. when a timeout occurs, collect signed MessageTypeTimeout messages for the exact current round;
8. call AdvanceRoundWithTimeoutEvidence only after collecting the evidence batch;
9. after a successful round change, discard round-local proposal/vote queues and continue from the runtime's new PhaseProposal state;
10. carry only the runtime-approved lock/LockProof into the next round.

The driver must treat runtime rejection as a state-machine decision, not as permission to mutate or bypass the runtime.

## Context invariants

Every event must preserve:

- protocol version;
- chain ID;
- epoch;
- height;
- exact round for proposal/vote/timeout messages;
- validator membership;
- voting-power membership;
- authenticated validator authority where signatures are required.

A message from an older round or height must never be reinterpreted as evidence for a newer context.

## Timeout and lock adoption

A timeout batch is valid only when:

- every timeout message authenticates against the validator authority;
- every message belongs to the runtime's current protocol/chain/epoch/height/round;
- all messages target the same strictly newer round;
- timeout quorum is reached;
- carried lock evidence is mutually consistent;
- nested LockProof is authenticated and structurally valid when present.

The driver must not merge timeout batches with different lock proposals, silently discard conflicting lock evidence, or convert structural timeout evidence into authenticated finality.

After successful adoption:

- a higher lock may replace a lower lock;
- an equal lock may only add missing authenticated proof;
- a conflicting equal-round lock is rejected;
- a lower lock must never downgrade existing lock state.

## Replay and partition behavior

The driver must be idempotent at the orchestration boundary:

- replayed old-round messages are rejected by context validation;
- replayed timeout certificates cannot regress round state;
- cross-height evidence is rejected;
- delayed messages may be dropped after the runtime rejects them;
- transport delivery order must not be used as a substitute for consensus validation.

A network partition must therefore produce rejected/stale evidence or a timeout path, not direct canonical-state mutation.

## Transport boundary

P2P transport is responsible for:

- peer-to-peer delivery;
- deterministic consensus message encoding/decoding;
- preserving the encoded consensus message payload.

The transport must not decide proposer eligibility, quorum, lock adoption, or finality.

The round driver is responsible for:

- mapping transport events to runtime calls;
- maintaining local event queues;
- scheduling timeouts;
- deciding when a timeout evidence batch is complete enough to submit.

The production timeout scheduler and network-wide synchronization mechanism are intentionally not implemented by this milestone.

## Canonical commit boundary

Consensus finality is not itself a canonical state commit.

The expected handoff remains:

proposal -> consensus evidence -> authenticated finality -> node finalized-block validation -> execution -> durable commit

The round driver must not bypass node-side finalized-block validation or directly mutate chain/state storage.

## Adversarial harness requirements

Before production scheduling is introduced, the deterministic multi-node harness must cover at least:

- stale proposal from an older round;
- stale timeout evidence replay after round change;
- conflicting timeout lock evidence;
- cross-height message rejection;
- invalid/tampered signatures;
- delayed/out-of-order messages;
- duplicate evidence;
- failed higher-lock adoption with unchanged runtime state;
- successful higher-lock adoption without lock downgrade;
- authenticated finality after multi-round progression.

Current milestone coverage adds transport-backed tests for stale round proposals, conflicting timeout locks, and replayed timeout evidence. Existing consensus/node regression suites already cover cross-height, signature, duplicate, atomicity, and finality boundaries.

## Non-goals

This specification does not claim:

- production BFT completion;
- production proposer/validator algorithm;
- network-wide failure detection;
- persistent consensus WAL/snapshot semantics;
- validator-set lifecycle;
- slashing/reward economics;
- production network liveness;
- formal BFT safety/liveness proof.

Those remain separate protocol and operational work.
