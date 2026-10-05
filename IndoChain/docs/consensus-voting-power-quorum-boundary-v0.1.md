# IndoChain v0.1 — Consensus Voting Power & Quorum Boundary

## Purpose

This milestone introduces deterministic handling for externally supplied validator voting power and a generic quorum comparison. It does not freeze production PoS economics or a specific BFT quorum policy.

## Implemented boundary

`IndoChain/internal/consensus/voting_power.go` provides:
- `ValidatorVotingPower`: validator identifier plus positive `uint64` voting power.
- `VotingPowerSet`: deterministic byte-sorted validator/power entries.
- cloning of validator identifiers at construction.
- duplicate/empty/zero-power rejection.
- validator voting-power lookup.
- checked total voting-power calculation.
- `QuorumThreshold`: caller-supplied numerator/denominator.
- `QuorumReached`: deterministic weighted threshold comparison.

The quorum comparison uses arbitrary-precision integer arithmetic so the full `uint64` input range can be compared without multiplication overflow.

## Deliberate non-decisions

This milestone does not define:
- how stake maps to voting power;
- minimum stake;
- delegation;
- validator registration or activation;
- validator-set transitions;
- epoch changes;
- proposer selection;
- a production quorum fraction;
- vote aggregation;
- locking;
- timeouts;
- finality certificates;
- rewards;
- slashing.

A caller must provide the voting-power set and quorum threshold explicitly.

## Relationship to validator membership

`ValidatorSet` remains the membership/authorization boundary.
`VotingPowerSet` is intentionally separate so future validator runtime logic can combine membership, voting power, message context, signature verification, and algorithm-specific quorum/finality rules.

No implicit conversion from validator membership to voting power exists.

## Testing scope

Tests cover deterministic sorting and input cloning, invalid and duplicate entries, power lookup and total power, threshold validation, below/exact/above quorum cases, and voted power greater than total power.

The implementation remains a development consensus foundation, not a complete BFT engine.