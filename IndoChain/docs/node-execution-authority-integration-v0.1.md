# Consensus ↔ Node Execution Authority Integration v0.1

## Purpose

The explicit execution-authority handoff is now connected to the node block-execution path without creating a canonical validator registry.

The node exposes `Node.ImportBlockWithAuthority(block, resolver)`.

The resolver is owned by the execution/node layer and supplies a public key for each transaction sender. The block execution rules retain the existing legacy `PublicKey` input for compatibility, while the new resolver path takes precedence when present.

```text
Finalized / Candidate Block
        ↓
Node
        ↓
TransactionAuthorityResolver
        ↓
Public Key per Sender
        ↓
ApplyTransaction
        ↓
State Snapshot
        ↓
Atomic Store Commit
```

## Safety

The resolver path executes the entire block against a working state before committing the block/state pair. Resolver failures or transaction validation failures therefore do not advance the node head.

The node integration does not infer that a consensus validator identifier is the same thing as a transaction sender address. Those are separate authority domains until a future protocol specification explicitly binds them.

## Compatibility

The existing `ImportBlock(block, publicKey)` API remains available for the current v0.1 single-key development path. `ImportBlockWithAuthority` is the explicit multi-sender authority integration boundary.

## Not frozen

This milestone does not define a persistent validator registry, address/key canonical serialization, validator-to-account binding, staking authority, production BFT semantics, fee/gas rules, or EVM account mapping.