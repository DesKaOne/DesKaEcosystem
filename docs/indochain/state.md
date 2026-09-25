# IndoChain State

> Status: Draft / State Design Baseline

## 1. Purpose

State represents the canonical data required to deterministically evaluate the next block.

## 2. Account Model

The current architecture selects the Account Model as the primary direction.

A conceptual account contains:

- address;
- native asset balance;
- nonce;
- optional contract code reference;
- contract/storage commitment where applicable;
- protocol metadata where required.

The exact serialization and storage layout remain open.

## 3. State Transition

~~~text
Parent State
    ↓
Block Transactions
    ↓
Validate + Execute
    ↓
State Changes
    ↓
State Root
    ↓
Commit
~~~

The same input state and valid block must always produce the same resulting state.

## 4. State Root

The state root commits to canonical state. The protocol must later define:

- tree/trie structure;
- key encoding;
- value encoding;
- hashing algorithm;
- proof format;
- empty-state representation;
- versioning.

## 5. Native Balance

Native asset balances are part of canonical account state.

The proposed native asset name is dIDR. Supply, emission, allocation, and monetary policy remain separate design decisions.

## 6. Nonce

Nonce prevents accidental replay and provides transaction ordering semantics for the account model.

Final nonce behavior must define:

- initial value;
- increment rules;
- failed transaction behavior;
- replacement behavior;
- interaction with reorganization/finality.

## 7. Contract State

If smart contracts are enabled, contract storage must be deterministic and committed by the state root.

Contract execution must not depend on:

- local filesystem state;
- local environment variables;
- wall-clock values not provided by protocol;
- external mutable APIs.

## 8. State Access API

The execution layer should use a narrow interface such as:

- Get(account/key)
- Set(account/key, value)
- Delete(account/key)
- Begin/Commit/Revert transaction scope
- Read-only access for simulation

The exact interface belongs to the VM/state implementation specification.

## 9. Snapshots

The node should support state snapshots for:

- fast synchronization;
- recovery;
- development networks;
- testnet reset/bootstrap.

Snapshots must be authenticated by state commitments and protocol metadata.

## 10. Testing

Required tests include:

- deterministic state transitions;
- balance conservation;
- nonce progression;
- invalid state mutation;
- state-root reproducibility;
- snapshot restore;
- rollback/revert behavior.
