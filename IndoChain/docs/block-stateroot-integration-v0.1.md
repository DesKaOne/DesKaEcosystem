# IndoChain Block StateRoot Integration v0.1

> Status: Development implementation baseline

## Purpose

Connect block execution with the deterministic development StateRoot.

## Execution Rule

After all block transactions execute successfully on the working state, the executor computes the working state's root.

If the block header contains a non-zero `StateRoot`, it must match the computed root.

Only after this check succeeds is the working state committed.

## Flow

~~~text
Block
  |
  v
Header validation
  |
  v
Execute transactions on snapshot
  |
  v
Compute StateRoot
  |
  +---- mismatch ---> rollback
  |
  v
Commit state
~~~

## Compatibility Note

A zero StateRoot remains accepted by the development executor so existing development blocks can execute before StateRoot is populated.

This is temporary implementation behavior, not a final protocol rule.

## Deferred

The integration does not yet define:

- final StateRoot algorithm;
- canonical serialization;
- TransactionsRoot;
- block hash;
- state proofs;
- persistent storage;
- consensus validation.