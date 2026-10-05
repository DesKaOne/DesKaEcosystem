# Consensus Message Implementation v0.1

## Purpose

This milestone introduces the first executable boundary for the draft consensus-message specification.

It does not implement PoS+BFT consensus, proposer selection, voting rounds, quorum, or finality.

## Message model

The development message contains:

- protocol version;
- chain ID;
- epoch;
- height;
- round;
- validator/sender identifier;
- message type;
- payload;
- signature.

Supported logical message types are:

- proposal;
- vote;
- finality evidence;
- validator-set update.

## Validation

The boundary validates:

- protocol version;
- chain ID;
- supported message type;
- required sender;
- required signature;
- payload size.

## Signing

Consensus signing uses the dedicated domain INDOCHAIN-CONSENSUS, separate from transaction signing.

Signing bytes are deterministic for the development implementation and include chain/version/epoch/height/round/sender/type/payload context.

This is development serialization only. It is not the final canonical wire encoding.

## Scope guardrail

The message boundary is intentionally independent from:

- block production;
- validator selection;
- consensus rounds;
- finality;
- P2P transport.

Those remain separate implementation milestones.
