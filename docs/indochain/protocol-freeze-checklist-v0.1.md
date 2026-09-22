# IndoChain Protocol Freeze Checklist v0.1

> Status: Pre-Implementation Checklist
> Branch policy: implementation work must use a non-main development branch

## 1. Purpose

This checklist is the gate between protocol documentation and large-scale implementation.

## 2. Confirmed Architectural Decisions

- Go is the primary node/core language.
- Flutter/Dart is the primary wallet stack.
- Account Model is the primary state-model direction.
- Consensus is modular.
- PoS+BFT is the production direction.
- PoW/mining is an optional profile/module.
- P2P, mempool, storage, VM, RPC, and consensus remain modular subsystems.
- Faucet functionality is restricted to Devnet/Testnet.
- Mainnet must not depend on development faucet configuration.

## 3. Protocol Freeze Gates

Before declaring v0.1 implementation-ready, explicitly resolve:

### Identity

- [ ] exact chain ID format
- [ ] network profile identifiers
- [ ] protocol version encoding
- [ ] genesis identity algorithm

### Cryptography

- [ ] hash algorithm
- [ ] signature algorithm
- [ ] public-key format
- [ ] signature format
- [ ] address derivation
- [ ] signing domains

### Serialization

- [ ] canonical encoding format
- [ ] field ordering
- [ ] integer widths/encoding
- [ ] byte/string encoding
- [ ] optional-field encoding
- [ ] maximum object sizes

### Transaction

- [ ] exact transaction fields
- [ ] nonce rules
- [ ] sender derivation/verification
- [ ] replay protection
- [ ] fee fields
- [ ] validation order
- [ ] transaction hash

### Block

- [ ] exact header fields
- [ ] block hash
- [ ] transaction-root algorithm
- [ ] state-root algorithm
- [ ] timestamp rules
- [ ] proposer authorization
- [ ] consensus evidence format

### State

- [ ] account schema
- [ ] state storage abstraction
- [ ] state commitment
- [ ] native asset accounting
- [ ] transaction execution semantics
- [ ] failure/revert semantics

### Consensus

- [ ] exact BFT algorithm
- [ ] proposer selection
- [ ] voting phases
- [ ] quorum/finality rule
- [ ] epoch model
- [ ] validator-set transitions
- [ ] slashing evidence

### P2P

- [ ] transport
- [ ] node identity
- [ ] handshake
- [ ] wire encoding
- [ ] message types
- [ ] peer discovery
- [ ] sync protocol
- [ ] rate limits

### Genesis

- [ ] exact genesis schema
- [ ] Devnet genesis
- [ ] Testnet genesis
- [ ] Mainnet genesis process
- [ ] initial validator configuration
- [ ] initial allocation configuration

## 4. Test Vector Gate

- [ ] crypto vectors
- [ ] address vectors
- [ ] transaction vectors
- [ ] block vectors
- [ ] state-transition vectors
- [ ] consensus vectors
- [ ] genesis vectors
- [ ] negative/error vectors
- [ ] Go compatibility tests
- [ ] Dart compatibility tests where applicable

## 5. Implementation Rule

If a protocol item is not frozen, the Go implementation must represent it behind an explicit interface/configuration boundary rather than silently choosing a consensus-critical behavior.

## 6. Branch Rule

The `main` branch is protected by workflow convention for this project.

Implementation branches should use a clear prefix, for example:

~~~text
feature/indochain-<component>
dev/indochain-v0.1
fix/indochain-<issue>
~~~

Large implementation work should be developed outside `main`, tested, reviewed, and merged through a pull request.

## 7. Current Development Branch

The initial implementation branch is:

~~~text
dev/indochain-v0.1
~~~

## 8. Completion Criteria

Protocol v0.1 is implementation-ready only when the required freeze gates are explicitly resolved and corresponding test vectors exist.
