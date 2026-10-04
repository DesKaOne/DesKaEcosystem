# Consensus Message Validation Pipeline v0.1

> Status: Development implementation boundary
> Scope: deterministic pre-consensus integration

## Purpose

This boundary composes the consensus validation pieces already implemented without prematurely defining the BFT algorithm.

The pipeline validates, in order:

1. message structure and protocol context;
2. exact round-state context;
3. validator membership of the sender.

## API

The implementation is exposed through:

- `MessageValidationContext`
- `ValidateConsensusMessage`

The context carries the existing `ValidationRules`, `RoundState`, and `ValidatorSet`.

## Validation order

~~~text
Consensus Message
      │
      ▼
ValidateMessage
      │
      ▼
ValidateMessageAgainstState
      │
      ▼
ValidateMessageSender
      │
      ▼
Accepted by current validation boundary
~~~

The order is intentional: malformed or wrong-context messages are rejected before membership checks.

## Signature verification

Signature verification remains a distinct cryptographic boundary through `VerifyMessageSignature`.

The composed pipeline does not silently infer a public key from validator membership and does not redefine the consensus message signature rules.

A caller that has the appropriate public key can perform cryptographic verification after structural/context/membership validation.

## Scope guardrails

This milestone does **not** define or implement:

- proposer selection;
- validator voting power;
- quorum thresholds;
- vote aggregation;
- locking rules;
- timeout behavior;
- finality certificates;
- validator-set transitions;
- staking or delegation;
- rewards or slashing;
- P2P transport.

The consensus design and v0.1 specification explicitly leave those items open. This boundary therefore provides integration of existing invariants without freezing those protocol choices.

## Determinism

Validation is pure with respect to its inputs. It does not mutate the message, round state, or validator set.

The boundary is intended as a reusable guard before future consensus-engine message handling.
