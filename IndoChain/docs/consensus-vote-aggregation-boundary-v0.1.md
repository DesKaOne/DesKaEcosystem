# IndoChain v0.1 — Consensus Vote Aggregation Boundary

## Purpose

This document defines the development-only vote aggregation boundary introduced for IndoChain v0.1.

The boundary connects the existing message validation, validator membership, voting-power, and quorum primitives without freezing a production BFT algorithm.

## Scope

The implementation provides:

- VoteAggregator for one exact RoundState context;
- validation that accepted messages are MessageTypeVote;
- existing consensus message/context/validator validation;
- sender uniqueness within the aggregation context;
- external VotingPowerSet lookup;
- aggregation of voting power for an exact opaque vote payload;
- caller-supplied quorum evaluation through QuorumThreshold.

The vote payload remains opaque. The aggregator therefore does not decide what a vote commits to.

## Signature Boundary

Signature verification remains separate from aggregation. Callers must verify signatures using the existing VerifyMessageSignature boundary and an appropriate validator authority mapping.

The current v0.1 validator model does not define a canonical mapping from validator identifier to public key, so the aggregator must not invent one.

## Explicit Non-Goals

This milestone does not define:

- a production BFT algorithm;
- prevote/precommit semantics;
- proposal validity rules;
- locking or unlock rules;
- timeout behavior;
- round advancement;
- finality certificates;
- validator-set transitions;
- stake/delegation rules;
- proposer priority/randomness;
- rewards or slashing;
- a production quorum fraction;
- canonical vote payload serialization;
- persistence or P2P transport for votes.

## Invariants

For a successfully constructed aggregator:

1. state and validation rules share protocol version and chain ID;
2. validator membership remains deterministic;
3. each sender contributes at most once;
4. only validators with voting power can be aggregated;
5. payload power is the sum of unique sender power;
6. quorum uses the already-defined overflow-safe QuorumReached;
7. adding a vote does not mutate the caller's message buffers.

## Relationship to Existing Boundaries

Message
  ↓
ValidateConsensusMessage
  ├── structure/protocol
  ├── exact RoundState context
  └── validator membership
          ↓
     VoteAggregator
          ├── unique sender
          ├── VotingPowerSet
          └── QuorumThreshold

The next consensus milestone can introduce explicit finality semantics and then connect the consensus engine to validator/block-production runtime.
