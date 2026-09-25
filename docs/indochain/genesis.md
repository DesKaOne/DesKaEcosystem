# IndoChain Genesis

> Status: Draft / Genesis Design Baseline

## 1. Purpose

Genesis is the deterministic root configuration of an IndoChain network.

## 2. Genesis Responsibilities

Genesis should define:

- chain ID;
- network name;
- protocol version;
- genesis timestamp;
- initial state;
- initial validators;
- initial protocol parameters;
- initial native-asset allocation where applicable.

## 3. Determinism

The same genesis configuration must produce the same genesis identity.

Conceptually:

~~~text
Genesis Config
     ↓
Canonical Encoding
     ↓
Genesis Hash / Identity
~~~

## 4. Network Profiles

At minimum, design separate profiles for:

- Devnet;
- Testnet;
- Mainnet.

Each profile must have a distinct network identity.

## 5. Initial Validators

Development networks may bootstrap validators from a deterministic genesis configuration.

Production validator onboarding should use the finalized validator protocol rather than relying on hidden operator state.

## 6. Native Asset

The proposed native asset is dIDR. Genesis allocation and monetary policy are not finalized and must be specified in economics/tokenomics documentation.

## 7. Genesis Validation

A node should validate:

- configuration schema;
- chain identity;
- protocol version;
- initial state;
- validator configuration;
- parameter bounds;
- genesis hash.

## 8. Reproducibility

Genesis tooling should be able to:

- generate a configuration;
- validate it;
- calculate the genesis identity;
- export deterministic artifacts;
- initialize a node.

## 9. Security

Production genesis artifacts must be reviewed and checksummed. Secret material should never be embedded in public genesis configuration.
