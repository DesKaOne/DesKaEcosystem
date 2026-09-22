# Consensus Round State — v0.1 Development

## Scope

This document defines the first executable round-state boundary for IndoChain consensus development.

It is intentionally an implementation boundary, not a final PoS+BFT specification.

## State

RoundState carries:

- protocol version
- chain ID
- epoch
- block height
- consensus round
- consensus phase

The initial phase is Proposal and the initial round is 0.

## Phase model

The current development model exposes:

1. Proposal
2. Prevote
3. Precommit
4. Finalized

The phase ordering is monotonic within a round.

This ordering is a development abstraction. Final BFT semantics, quorum rules, voting rules and finality conditions remain outside this milestone.

## Transition rules

### Round

A round may remain unchanged or increase.

Increasing the round resets the phase to Proposal.

Round regression is rejected.

### Height

A height may remain unchanged or increase.

Increasing the height resets both round and phase:

new height -> round 0 -> Proposal

Height regression is rejected.

### Phase

A phase may remain unchanged or advance.

Phase regression is rejected.

## Context

A state context consists of:

- protocol version
- chain ID
- epoch
- height

Two round states are considered to have the same context only when all four values match.

## Guardrails

This implementation does not yet define:

- proposer selection
- validator sets
- voting power
- quorum
- vote aggregation
- timeout behavior
- evidence handling
- finality certificates
- consensus/network transport
- persistence of consensus state

Those belong to later consensus milestones.

## Determinism

The state transition functions are pure value transitions: they return a new RoundState and do not mutate shared state.

Invalid transitions return errors without changing the source state.

## Status

Development-only consensus foundation for v0.1. The final consensus specification may replace or refine these phase semantics before protocol freeze.