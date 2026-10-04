# Genesis Implementation v0.1

> Status: Development implementation
> Scope: Devnet only
> Canonical genesis serialization: not frozen

## 1. Purpose

This implementation turns the genesis specification into a deterministic Devnet fixture without freezing unresolved protocol choices.

## 2. Devnet Defaults

The current development defaults are:

- Network profile: `devnet`;
- Chain ID: `1001`;
- Protocol version: `1`;
- Timestamp: `2026-01-01T00:00:00Z`;
- Initial validators: empty;
- Initial allocations: empty;
- Protocol parameters: empty.

These are implementation defaults for the Devnet baseline, not a production/mainnet commitment.

## 3. Genesis State

`Genesis.State()` creates the initial account state from `InitialAllocations`.

Each allocation is represented as:

`address -> Account{Balance, Nonce=0}`

The state commitment uses the existing development `State.Root()` implementation. That root is deterministic but is not yet the final protocol StateRoot.

## 4. Genesis Block

`Genesis.Block()` creates height-zero block metadata:

- protocol version from genesis;
- chain ID from genesis;
- height `0`;
- fixed Devnet timestamp;
- zero previous hash;
- empty transaction list;
- deterministic empty transactions root;
- initial state root;
- no proposer;
- no consensus evidence.

The existing development block hash can therefore produce a deterministic identifier for the genesis block.

## 5. Genesis Configuration Identity

`Genesis.Hash()` produces a deterministic development identity from:

- chain ID;
- network profile;
- protocol version;
- timestamp;
- sorted validator identities;
- sorted account identities;
- sorted initial allocations;
- sorted protocol parameters.

Map insertion order does not affect the result.

This hash is intentionally separate from the block hash. The final protocol must define whether genesis identity is the canonical hash of the full serialized genesis document, the genesis block, or another committed representation.

## 6. Verification Boundary

The current implementation intentionally does not enforce unresolved rules such as:

- final validator schema;
- stake/delegation semantics;
- final allocation format;
- final protocol parameter schema;
- canonical genesis serialization;
- final chain ID registry;
- production timestamp policy.

Those remain governed by `genesis-spec-v0.1.md` and the protocol freeze process.

## 7. Test Coverage

The Devnet fixture tests:

- deterministic genesis identity;
- deterministic initial state;
- allocation application;
- height-zero block shape;
- zero previous hash;
- state-root consistency;
- non-zero development block hash;
- map-order-independent genesis identity;
- configuration mutation changing identity;
- input immutability during hashing.
