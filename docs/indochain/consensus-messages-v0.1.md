# IndoChain Consensus Messages v0.1

> Status: Draft Specification

## 1. Purpose

This document defines the logical message classes used by the modular consensus layer before exact wire encoding is frozen.

## 2. Message Envelope

Every consensus message should conceptually contain:

~~~text
ConsensusMessage
├── ProtocolVersion
├── ChainID
├── Epoch
├── Height
├── Round / View
├── Sender / Validator
├── MessageType
├── Payload
└── Signature / Authentication
~~~

## 3. Proposal

A proposal identifies the block candidate and proposer context required for validators to verify the candidate.

## 4. Vote

A vote authenticates a validator's protocol-defined position for a height/round and block identifier or nil/empty value where supported.

The exact vote phases are TBD.

## 5. Finality Evidence

Finality evidence represents the protocol-defined proof that sufficient validator agreement has been reached.

The exact quorum certificate/commit structure is TBD.

## 6. Validator-Set Updates

Validator-set changes must be applied at deterministic protocol boundaries and must not create ambiguity about which set is authoritative for a given height/epoch.

## 7. Replay Protection

Consensus messages must be bound to chain ID, protocol version, height, epoch/round, and message type as required to prevent cross-context replay.

## 8. Signature Domain

Consensus signatures must use a dedicated domain separate from transaction signing.

## 9. Validation

A node must reject malformed, stale, wrong-chain, unauthorized, duplicate/conflicting, or incorrectly signed consensus messages according to the final consensus rules.

## 10. Open Items

- exact BFT algorithm;
- proposal/vote phases;
- round/view rules;
- quorum formula;
- evidence encoding;
- aggregate-signature strategy;
- validator-set transition rules.
