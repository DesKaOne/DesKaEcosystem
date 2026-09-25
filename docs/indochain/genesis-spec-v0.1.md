# IndoChain Genesis Specification v0.1

> Status: Draft Specification
> Scope: Devnet protocol baseline

## 1. Purpose

Genesis defines the initial identity, configuration, validator set, and state of an IndoChain network.

## 2. Genesis Structure

~~~text
Genesis
├── ChainID
├── NetworkProfile
├── ProtocolVersion
├── Timestamp
├── InitialValidators
├── InitialAccounts
├── InitialAllocations
└── ProtocolParameters
~~~

The exact serialized format is governed by the canonical serialization specification.

## 3. Chain Identity

The genesis identity must be deterministic and reproducible from the canonical genesis configuration.

Nodes must reject a genesis that does not match the configured chain identity/profile.

## 4. Network Profiles

Separate genesis configurations are required for:

- Devnet;
- Testnet;
- Mainnet.

A Devnet genesis may use development-only validators, allocations, mining, and faucet functionality.

Testnet may use controlled public-test parameters.

Mainnet must not inherit Devnet/Testnet private keys or faucet configuration.

## 5. Initial Validators

Where the selected consensus profile requires validators, genesis may define the initial validator set and associated public identities/stake configuration.

Exact validator fields remain TBD.

## 6. Initial State

Genesis may initialize:

- accounts;
- balances;
- contract/system state;
- validator state;
- protocol parameters.

Every initialized value must be deterministic and serializable.

## 7. Native Asset

The proposed native asset is dIDR. Initial supply/allocation rules are not frozen by this document.

## 8. Faucet Isolation

Faucet accounts/configuration are permitted only in Devnet/Testnet profiles. Mainnet genesis and runtime configuration must not enable the faucet path.

## 9. Verification

Genesis verification should check:

- canonical encoding;
- chain ID;
- protocol version;
- initial state commitment;
- validator configuration;
- parameter validity.

## 10. Test Requirements

Provide deterministic genesis fixtures for Devnet and Testnet profiles and verify that independent nodes compute identical genesis identity and initial state commitment.

## 11. Open Items

- exact genesis schema;
- chain IDs;
- timestamp policy;
- initial validators;
- allocations;
- parameter encoding;
- initial state commitment.
