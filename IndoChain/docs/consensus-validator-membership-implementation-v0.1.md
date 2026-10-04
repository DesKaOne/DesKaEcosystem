# Consensus Validator Membership — v0.1 Development

## Scope

This milestone adds a deterministic validator-membership boundary for consensus development.

The set stores validator identifiers in canonical byte-sorted order and rejects empty or duplicate identifiers.

## Membership

The boundary exposes:

- construction from validator identifiers;
- deterministic ordering;
- validation;
- membership lookup;
- required-membership validation.

This provides a base for later sender authorization.

## Guardrails

This implementation does not define:

- minimum stake;
- voting power;
- delegation;
- validator registration transactions;
- activation/deactivation rules;
- epoch transitions;
- proposer selection;
- rewards;
- slashing;
- validator key rotation.

Those decisions remain open in the validator design and belong to later consensus/validator milestones.

## Determinism

Validator identifiers are cloned on construction and sorted by byte ordering.

Validation does not mutate the validator set.

## Integration boundary

The validator set is intentionally independent from account state, staking economics, P2P transport, and block execution.

A later consensus-engine boundary can use this set to verify that a consensus message sender belongs to the active validator membership without yet deciding voting power or quorum.

## Status

Development-only consensus foundation for v0.1. The final validator-set representation and activation rules remain subject to the protocol specification before freeze.
