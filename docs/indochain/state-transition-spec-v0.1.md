# IndoChain State Transition Specification v0.1

> Status: Draft Specification
> Scope: Devnet protocol baseline

## 1. State Model

The primary state model is account-based.

Conceptually:

~~~text
PreState + BlockTransactions + ProtocolRules
                    ↓
                 PostState
                    ↓
                StateRoot
~~~

## 2. Determinism

All nodes processing the same canonical state and valid block must produce the same post-state and state commitment.

## 3. Account State

The exact account schema is TBD, but must define at minimum the information required for authorization, nonce tracking, balances, and contract/application state where applicable.

## 4. Transaction Execution

For each transaction, the state transition engine must validate authorization and protocol constraints before applying state changes.

Execution must account for:

- nonce;
- balance/value;
- gas/fees;
- recipient or execution target;
- payload;
- VM result where applicable;
- logs/events where applicable.

## 5. Failure Semantics

The protocol must distinguish invalid transactions from valid transactions whose execution produces a protocol-defined failure/revert.

Exact rollback and gas-consumption semantics depend on the final VM/fee specification.

## 6. State Commitments

After block execution, the resulting state is committed using StateRoot.

The exact commitment/tree/database representation is TBD.

## 7. Native Asset

The roadmap identifies dIDR as the proposed native asset. Its exact monetary and accounting rules remain defined by the economics/protocol specifications.

## 8. Contracts

If VM support is enabled, contract storage and execution state become part of the deterministic state transition.

The VM must not depend on wall-clock time, external network access, local filesystem state, or nondeterministic host behavior unless explicitly virtualized by protocol.

## 9. Re-execution

A node must be able to deterministically re-execute canonical blocks to verify state transitions, subject to configured pruning/snapshot capabilities.

## 10. Test Vectors

Provide vectors for native transfer, nonce progression, fee deduction, invalid authorization, insufficient balance, contract execution, execution failure, and resulting state commitments where each feature is enabled.

## 11. Open Items

- exact account schema;
- state tree/commitment;
- fee charging order;
- failed execution semantics;
- contract storage model;
- VM host interface.
