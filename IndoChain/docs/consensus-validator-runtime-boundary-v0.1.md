# IndoChain v0.1 — Validator Runtime Boundary

## Purpose

This milestone introduces a deterministic development orchestration boundary that connects the consensus primitives already implemented.

The runtime coordinates:

1. proposer selection;
2. proposal acceptance;
3. vote aggregation;
4. caller-supplied quorum;
5. finality certificate creation.

It does not claim to be the production BFT consensus engine.

## Runtime Lifecycle

The development lifecycle is:

Proposal → Prevote → Precommit → Finalized

### Proposal

The runtime checks:

- current phase is Proposal;
- message type is Proposal;
- message context is valid;
- sender belongs to the validator set;
- sender is the selected proposer for the current round;
- proposal payload is non-empty.

The payload remains opaque.

### Vote

Votes are passed through the existing vote aggregation boundary.

The runtime advances from Prevote to Precommit only when the supplied quorum threshold is reached for the accepted payload.

Duplicate validators and validators without voting power are rejected by the existing aggregation layer.

### Finalization

Once the runtime reaches Precommit, it constructs a FinalityCertificate using the existing finality boundary.

The runtime then advances to Finalized.

Finalization here means development consensus evidence has reached the supplied quorum. It does not commit a block or mutate canonical chain state.

## Deliberate Non-Goals

This milestone does not define:

- production BFT algorithm;
- prevote/precommit wire semantics;
- lock/unlock rules;
- timeout handling;
- round-change behavior;
- validator-set lifecycle;
- stake/delegation mapping;
- production proposer policy;
- canonical proposal encoding;
- canonical finality encoding;
- block execution;
- block commit;
- persistence;
- P2P transport;
- signature authority registry;
- validator rewards or slashing.

## Safety Boundary

The runtime is intentionally deterministic and non-persistent.

Rejected proposals and votes do not advance the runtime phase.

A finalized development certificate is evidence over an opaque payload. Block validity and canonical state commitment remain outside this boundary.

## Test Coverage

Tests cover:

- expected proposer acceptance;
- unexpected proposer rejection without phase mutation;
- quorum-driven phase progression;
- finality certificate generation;
- refusal to finalize before quorum.

## Next Integration

The next step is to define the production consensus state machine and block-production interface explicitly before connecting this runtime to canonical block execution or P2P message transport.


## Timeout / Round-Change Boundary

The development runtime now exposes `ValidatorRuntime.AdvanceRound(next)` for a deterministic timeout/round-change transition.

The transition:
- requires a strictly newer round;
- resets the runtime phase to Proposal;
- clears the current round proposal;
- replaces the vote aggregator with one bound to the new round;
- preserves the previously locked proposal;
- rejects round changes after Finalized.

A preserved lock constrains the next-round proposal: the same locked payload remains acceptable, while a conflicting proposal is rejected. This is intentionally a development invariant rather than a claim of full production BFT locking.

Vote messages are validated against the current round and validator set before the lock-conflict check, preserving the existing validation boundary and preventing malformed messages from bypassing it.

### Tests

Coverage includes successful round advancement, round-local state reset, lock preservation, conflicting proposal rejection, and finalized-state round-change rejection.

### Non-goals

This boundary does not yet define timeout certificates, dedicated timeout messages, prevote/precommit wire types, lock-carrying evidence, multi-node round synchronization, or a production BFT timeout algorithm.


## Timeout Evidence Boundary

The consensus package now includes a development-only `TimeoutCertificate` boundary. It binds the current consensus context to a strictly newer target round, a caller-supplied quorum threshold, and a unique canonical validator set.

This evidence requires a strictly newer target round, valid threshold, non-empty validator evidence, validator membership and voting power, unique identifiers, checked voting-power aggregation reaching quorum, and canonical byte ordering.

The certificate describes why a runtime may advance rounds, but does not itself perform the transition. `ValidatorRuntime.AdvanceRound(next)` remains the state-transition boundary.

It intentionally does not define signed timeout messages, aggregated signatures, lock-carrying evidence, network synchronization, or a production BFT timeout algorithm.

## Signed Timeout Message Boundary

The consensus package now includes a development-only signed timeout-message boundary.

A timeout message uses `MessageTypeTimeout` and carries the target next round as a canonical 8-byte big-endian payload. The message is bound to the current protocol/chain/epoch/height/round context and uses the existing consensus signing domain.

`ValidateTimeoutMessage` validates message structure, exact round-state context, validator membership, strictly newer target round, public-key resolution, and Ed25519 signature verification.

`NewTimeoutCertificateFromMessages` authenticates all supplied timeout messages, requires a common target round, and delegates unique-validator, voting-power, quorum, and canonical-order checks to `NewTimeoutCertificate`. It does not advance the runtime or mutate canonical state.

### Tests

Coverage includes:
- successful signed timeout evidence and certificate construction;
- tampered payload/signature rejection;
- mismatched target-round rejection;
- stale target-round rejection.

### Non-goals

This boundary does not yet define aggregated timeout signatures, lock-carrying timeout evidence, proposer synchronization, automatic runtime advancement, P2P timeout transport, persistent timeout evidence, or the production BFT timeout algorithm.
