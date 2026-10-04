# Consensus Message ↔ Round State Context — v0.1 Development

## Scope

This milestone connects the executable consensus message boundary with the executable round-state boundary.

It verifies that a message belongs to the exact protocol context represented by the current round state before later consensus semantics are applied.

## Context fields

The validation boundary requires exact equality for:

- protocol version
- chain ID
- epoch
- block height
- consensus round

A mismatch in any field is rejected with `ErrConsensusMessageContextMismatch`.

## State validation

The round state is validated first. Invalid protocol/chain context or invalid phase is rejected before message matching is attempted.

## What this does not define

This boundary intentionally does not decide:

- whether the sender is a validator;
- proposer eligibility;
- message-type-specific payload rules;
- voting power;
- quorum;
- timeout behavior;
- vote aggregation;
- evidence validity;
- finality;
- validator-set transitions;
- network transport.

Those remain later consensus-engine responsibilities.

## Determinism

Validation is a pure boundary check. It does not mutate the supplied round state or message.

## Development status

This is an implementation boundary for v0.1 development. The final consensus specification may refine the context model before protocol freeze.
