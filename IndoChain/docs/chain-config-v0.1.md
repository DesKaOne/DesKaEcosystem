# Chain Configuration v0.1

## Purpose

The chain configuration boundary keeps network identity and execution validation rules together so node code does not duplicate Devnet constants or transaction policy.

## Configuration

ChainConfig currently contains:
- NetworkProfile
- ChainID
- ProtocolVersion
- transaction validation rules

Devnet derives its identity values from genesis/devnet.Default().

## Validation

ChainConfig.Validate rejects an empty network profile, zero chain ID, zero protocol version, or transaction rules whose chain ID or protocol version differs from the chain configuration.

## Block execution rules

ChainConfig.BlockRules converts the chain configuration into block.ExecutionRules.

The public key remains an explicit argument because the v0.1 transaction model does not contain a canonical sender public-key field. This avoids inventing a protocol field before transaction and crypto serialization are frozen.

## Node boundary

The node can use one ChainConfig as the source for network identity, protocol version, transaction validation policy, and block execution rules.

## Not frozen

This package does not freeze production chain IDs, transaction schema, canonical serialization, signature algorithm, fee formula, validator/finality rules, or VM selection. Those remain subject to the v0.1 protocol freeze.