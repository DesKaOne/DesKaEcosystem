# IndoChain Governance

> Status: Draft / Governance Design Baseline

## 1. Purpose

Governance provides a controlled mechanism for changing protocol parameters and, where explicitly supported, coordinating protocol upgrades.

Governance must never introduce ambiguous state-transition rules.

## 2. Governance Scope

Potential governance-controlled parameters include:

- protocol parameters;
- validator-set policy;
- fee parameters;
- gas parameters;
- staking parameters;
- reward parameters;
- feature activation;
- approved protocol upgrades.

Parameters that affect deterministic execution must have explicit activation rules.

## 3. Governance Lifecycle

~~~text
Proposal
   ↓
Validation
   ↓
Voting
   ↓
Quorum / Threshold
   ↓
Accepted
   ↓
Scheduled Activation
   ↓
Protocol Change
~~~

## 4. Proposal Types

Candidate types:

- parameter change;
- software/protocol upgrade;
- validator policy change;
- treasury/community allocation;
- emergency security action.

The final proposal taxonomy remains open.

## 5. Voting

Voting may use validator stake, delegated stake, or another protocol-defined voting power model.

The protocol must specify:

- eligibility;
- voting period;
- quorum;
- approval threshold;
- abstention;
- delegation;
- proposal deposit;
- duplicate/conflicting proposals.

## 6. Timelock

Production changes should support a delay between acceptance and activation where appropriate.

This gives operators, wallets, explorers, and users time to upgrade before a deterministic rule changes.

## 7. Emergency Actions

Emergency governance must be narrowly scoped and auditable.

An emergency mechanism must not become an unrestricted bypass of consensus.

## 8. Upgrade Compatibility

Every protocol upgrade should define:

- target version;
- activation height/epoch;
- required node version;
- compatibility period;
- migration requirements;
- rollback/recovery considerations.

## 9. Security

Governance must protect against:

- replayed votes;
- duplicate voting;
- unauthorized proposals;
- insufficient quorum;
- malicious parameter values;
- accidental incompatible activation.

## 10. Open Decisions

- voting power;
- quorum;
- approval threshold;
- proposal deposit;
- timelock duration;
- emergency authority;
- treasury governance;
- upgrade mechanism.
