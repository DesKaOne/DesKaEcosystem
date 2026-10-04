# Canonical Commit Failure-Atomicity Regression Matrix v0.1

## Purpose
Milestone 4.48 verifies the existing canonical storage commit boundary under controlled commit failure.

The goal is to prove that a failed `ChainStore.CommitBlockState` attempt does not publish a new block head or state, and that the existing persisted snapshot remains unchanged when the storage implementation fails before the atomic rename boundary.

## Regression matrix
| Store | Scenario | Expected result |
|---|---|---|
| FileStore | Successful initial commit | block head and state become canonical |
| FileStore | Commit fails at destination rename | in-memory head/state remain unchanged |
| FileStore | Commit fails at destination rename | persisted snapshot remains unchanged |
| MemoryStore | Successful commit | block head and state are available together |

The failure test redirects the test instance's file target to a directory so the final rename cannot succeed. The original canonical file remains intact and is reopened after the failure to verify the persisted snapshot.

## Atomicity invariant
For `FileStore.CommitBlockState`:

```text
build candidate snapshot
        |
encode temporary file
        |
sync temporary file
        |
close temporary file
        |
atomic rename
        |
publish in-memory data
```

The in-memory `FileStore.data` assignment occurs only after the rename succeeds.

Therefore a failure before rename completion must not advance the in-memory canonical head/state.

## Scope
This milestone verifies the current development `FileStore` implementation and a successful `MemoryStore` commit.

It does not claim:
- distributed transactional semantics;
- database-level transactions;
- process-crash testing;
- filesystem crash consistency beyond the existing temp-file + sync + rename implementation;
- WAL/snapshot recovery activation;
- production BFT completion.

## Relationship to 4.47
4.47 defined the consensus-to-storage handoff contract.

4.48 verifies the storage side of that contract for the current development `FileStore`: a failed canonical commit is an error, not a successful publication, and the store does not publish the candidate snapshot before its atomic replacement boundary succeeds.

## Known limitations
`MemoryStore.CommitBlockState` currently has no independently injectable post-block/pre-state failure point because its state operations do not return a persistence failure after `SaveBlock`.

Future storage implementations should expose equivalent failure-atomicity tests around their own durable commit boundary without making test-only fault controls part of the protocol contract.