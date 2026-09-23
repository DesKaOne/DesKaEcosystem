# IndoChain v0.1 — Consensus Proposer Selection Boundary

## Purpose

This milestone adds a deterministic proposer-selection boundary after validator membership and voting-power/quorum foundations.

## Implemented boundary

`IndoChain/internal/consensus/proposer.go` defines `ProposerSelector` and a development-only `RoundRobinProposer`.

`RoundRobinProposer`:
- validates the current `RoundState`;
- validates the supplied `ValidatorSet`;
- rejects an empty validator set;
- uses the deterministic byte-sorted validator order already established by `ValidatorSet`;
- selects `round mod validator_count`;
- returns a cloned validator identifier.

## Deliberate non-decisions

This is a development selector, not the production PoS proposer algorithm. It does not model stake, voting power, proposer priority, randomness, VRF, weighted scheduling, validator performance, slashing, or rewards.

The production proposer policy remains open and must be specified before mainnet consensus is considered frozen.

## Relationship to voting power

`VotingPowerSet` remains separate. The current round-robin selector does not implicitly consume voting power, so adding voting power did not silently change proposer semantics.

## Testing scope

Tests cover deterministic selection across rounds, canonical validator ordering, result cloning, empty validator sets, and invalid round state.

The implementation remains a consensus foundation and does not constitute a complete BFT engine or finality mechanism.