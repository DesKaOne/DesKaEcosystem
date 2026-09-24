# Consensus ↔ Block Candidate Proposal Bridge v0.1

## Purpose

The block-candidate boundary now has an explicit in-memory bridge to the existing consensus proposal/vote runtime.

`BlockProposal` binds a validated `block.Block` candidate to the deterministic development block hash returned by `ValidateProducedBlock`.

## Flow

```text
Canonical State
     ↓
BuildBlockCandidate
     ↓
Block Candidate
     ↓
NewBlockProposal
     ├── ValidateProducedBlock
     └── Development Block Hash
              ↓
       Opaque Message Payload
              ↓
       ValidatorRuntime
```

The bridge allows a block candidate to enter the existing `ValidatorRuntime` proposal/vote lifecycle without making the runtime parse or execute an arbitrary block payload.

## Implemented invariants

`NewBlockProposal`:

- validates the candidate against the current block-production context;
- derives the same deterministic development block hash used by `ValidateProducedBlock`;
- retains the candidate in an in-memory `BlockProposal`;
- exposes a cloned hash payload for consensus `Message` values.

`BlockProposal` does not mutate the candidate or consensus state.

## Deliberate boundary

The proposal payload is currently an opaque 32-byte development block hash. The runtime therefore attests to the identity of the candidate payload, not to a canonical serialized block carried inside the consensus message.

The block itself remains available to the block-production/validation caller. This keeps canonical serialization, P2P proposal encoding, and block execution out of the consensus message boundary until those protocol specifications are frozen.

## Not frozen

This milestone does not define:

- canonical block serialization;
- consensus wire encoding;
- P2P proposal transport;
- proposal signatures beyond the existing message signature boundary;
- block execution/commit during consensus finalization;
- timeout or round-change behavior;
- production BFT semantics;
- validator-set lifecycle;
- proposer priority/randomness.

## Tests

Tests cover candidate-to-payload bridging, rejection of invalid candidates, payload round-trip, and payload cloning.
