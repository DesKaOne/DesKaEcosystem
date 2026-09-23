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
