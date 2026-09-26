# IndoChain v0.1 — Consensus Finality Boundary

## Purpose

This document defines the development boundary for a finality certificate without freezing the production BFT algorithm.

The milestone connects the existing:

1. consensus message validation;
2. exact round-state context;
3. validator membership;
4. voting power;
5. quorum;
6. vote aggregation

into a deterministic certificate that can be independently validated.

## Implemented Boundary

IndoChain/internal/consensus/finality.go provides:

- FinalityCertificate
- NewFinalityCertificate
- ValidateFinalityCertificate

A certificate contains:

- protocol version
- chain ID
- epoch
- height
- round
- opaque payload
- caller-supplied quorum threshold
- the unique validator votes supporting that payload

Certificate construction re-validates every vote through the existing vote aggregation boundary and requires the supplied quorum threshold to be reached.

Certificate validation reconstructs the same development aggregation context and checks:

- state context equality;
- threshold validity;
- non-empty payload/evidence;
- validator authorization;
- voting-power membership;
- duplicate-vote rejection;
- payload-specific voting power;
- quorum.

Validation is non-mutating and does not advance RoundState.

## Deliberate Non-Goals

This milestone does not define or freeze:

- a specific BFT algorithm;
- Tendermint/HotStuff/PBFT semantics;
- prevote vs precommit meaning;
- lock/unlock rules;
- timeout or round-change rules;
- proposer priority/randomness/VRF;
- production quorum fraction;
- validator activation/deactivation;
- stake/delegation mapping;
- validator-set transitions;
- slashing/evidence economics;
- rewards;
- canonical certificate serialization;
- persistent certificate storage;
- P2P finality transport;
- block-production integration;
- finality gadget across multiple heights.

## Security Boundary

The certificate is an attestation over an opaque payload. It must not be interpreted as proof that the payload is a valid block until block/proposal validation is explicitly connected.

Signature verification remains a separate concern because v0.1 does not yet define a canonical validator-ID-to-public-key authority registry.

## Test Coverage

The test suite covers:

- quorum requirement;
- cloning of payload/evidence inputs;
- context mismatch;
- mixed payload rejection;
- duplicate sender rejection;
- non-mutating validation.

## Next Integration

The next consensus work should connect finality evidence to validator/block runtime only after the production meaning of proposal, prevote, precommit, timeout, and finality is specified.
