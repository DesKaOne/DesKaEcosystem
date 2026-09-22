# IndoChain Consensus

> Status: Draft / Consensus Design Baseline

## 1. Direction

The architecture currently targets a modular PoS + BFT direction.

This is a design direction, not yet a frozen consensus specification.

## 2. Responsibilities

Consensus is responsible for:

- proposer selection;
- validator participation;
- proposal validation;
- voting;
- quorum/finality;
- validator-set changes;
- evidence handling;
- slashing hooks;
- epoch transitions.

## 3. Validator Set

The validator set is derived from canonical protocol state.

Potential validator lifecycle:

~~~text
Register
  ↓
Stake
  ↓
Activate
  ↓
Participate
  ↓
Reward / Slash
  ↓
Exit / Unbond
~~~

Exact thresholds and timing remain open.

## 4. Proposal and Voting

Conceptual flow:

~~~text
Select proposer
      ↓
Build block
      ↓
Broadcast proposal
      ↓
Validators verify
      ↓
Validators vote
      ↓
Quorum reached
      ↓
Finality / Commit
~~~

The exact message types, rounds, quorum formula, timeout rules, and locking rules must be specified before implementation freeze.

## 5. Finality

The protocol should distinguish:

- proposed;
- accepted/committed locally;
- finalized.

The finality definition must be deterministic and externally verifiable.

## 6. Epochs

Epochs may be used for:

- validator-set updates;
- reward calculation;
- parameter activation;
- scheduled governance changes.

Epoch length is not yet frozen.

## 7. Slashing

Candidate slashable behavior includes:

- double-signing;
- conflicting votes;
- invalid consensus behavior where provable.

Evidence must be cryptographically verifiable.

Exact penalties and unbonding behavior belong in the validator/economics specification.

## 8. Rewards

Consensus may expose hooks for block/proposer and validator rewards.

Reward amount, issuance, distribution, and treasury behavior remain tokenomics decisions.

## 9. Safety and Liveness

The implementation must test:

- conflicting proposals;
- delayed messages;
- offline validators;
- network partitions;
- validator restarts;
- duplicate messages;
- malicious peers;
- recovery after missed rounds.

## 10. Open Decisions

- exact BFT protocol;
- proposer selection;
- quorum threshold;
- block interval;
- timeout schedule;
- validator count limits;
- epoch length;
- slashing parameters;
- reward parameters;
- evidence format.
