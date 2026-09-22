# IndoChain Consensus Specification v0.1

> Status: Draft Specification
> Scope: Devnet protocol baseline

## 1. Purpose

Consensus determines how nodes agree on the canonical block sequence and, for the selected production direction, how finalized state is established.

## 2. Architecture

Consensus is modular:

~~~text
consensus/
├── pos/
├── bft/
└── pow/
~~~

PoS+BFT is the production direction. PoW remains an optional mining/profile module for development, experimentation, and benchmarking.

## 3. Validator Set

The active validator set is consensus-critical. The protocol must define how validators enter, become active, participate, exit, and become inactive.

The exact stake threshold and validator-set size are TBD.

## 4. Proposer Selection

A proposer is selected according to the active consensus profile. Selection must be deterministic from canonical protocol inputs and must not depend on local randomness unless that randomness is itself protocol-defined.

## 5. BFT Voting

The BFT layer may use a multi-phase voting model to establish agreement and finality.

The final phases, quorum threshold, vote encoding, timeout behavior, and evidence format are TBD.

## 6. Finality

The protocol must distinguish:

- proposed/observed block;
- accepted canonical block;
- finalized block.

Once finalized under the selected BFT rules, a block must not be reverted by an honest node except through an explicitly defined protocol upgrade/emergency mechanism.

## 7. Validator Misbehavior

The protocol should be able to represent evidence for consensus-critical faults such as conflicting proposals or votes where applicable.

Slashing conditions, evidence retention, and penalties remain TBD.

## 8. Epochs

Epochs may be used to manage validator sets, proposer schedules, rewards, and governance-related changes.

Epoch length and transition rules are TBD.

## 9. Rewards

Validator rewards may derive from protocol-defined issuance, fees, or both.

Exact reward calculation is defined by economics and remains TBD for v0.1.

## 10. PoW Profile

The PoW module must remain isolated from PoS+BFT consensus interfaces.

A PoW profile may define difficulty, nonce, mining rewards, and block acceptance independently for suitable networks.

## 11. Consensus Messages

Consensus messages require canonical serialization and domain separation. Candidate message classes include proposal, vote/prevote, precommit/finalization evidence, and validator-set updates.

Exact message schemas are TBD.

## 12. Test Requirements

Tests must cover deterministic proposer selection, valid/invalid votes, conflicting messages, timeout behavior, validator-set changes, finality, restart/recovery, and Byzantine scenarios.

## 13. Open Items

- final BFT algorithm;
- quorum formula;
- proposer selection;
- epoch length;
- validator-set limits;
- stake thresholds;
- slashing rules;
- reward rules;
- timeout schedule;
- consensus message encoding.
