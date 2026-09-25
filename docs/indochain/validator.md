# IndoChain Validators

> Status: Draft / Validator Design Baseline

## 1. Purpose

Validators participate in consensus and are responsible for proposing, validating, voting on, and committing blocks according to protocol rules.

## 2. Validator Components

A validator node conceptually contains:

~~~text
Node
├── P2P
├── Consensus Engine
├── Execution Engine
├── State / Block Storage
├── Validator Key
├── RPC (optional / restricted)
└── Metrics
~~~

## 3. Validator Registration

A validator lifecycle may include:

1. account creation;
2. stake/deposit;
3. registration;
4. eligibility check;
5. activation at an epoch boundary;
6. consensus participation.

Exact registration transactions remain to be specified.

## 4. Validator Keys

Separate key purposes are recommended:

- account/staking key;
- consensus signing key;
- node identity/network key.

Operational procedures for rotation, backup, and recovery must be documented before testnet.

## 5. Proposer

The proposer is selected according to the consensus algorithm.

The proposer must:

- construct a valid block;
- execute candidate transactions;
- produce correct commitments;
- sign the proposal;
- broadcast it through P2P.

## 6. Voting

Validators verify proposals before voting.

Votes must include enough context to prevent replay across:

- chain;
- height;
- round;
- protocol version.

## 7. Rewards and Slashing

Validator rewards and penalties are protocol/economics concerns. The validator module should expose accounting hooks without hard-coding tokenomics prematurely.

## 8. Validator Security

Recommended operational controls:

- isolated signing key;
- restricted RPC;
- firewall/network policy;
- encrypted backups;
- monitoring;
- duplicate-sign prevention;
- safe restart procedure.

## 9. Testnet Requirements

Testnet validator tooling should provide:

- deterministic genesis;
- validator key generation;
- registration/bootstrap;
- node startup;
- health checks;
- logs and metrics;
- snapshot restore;
- controlled reset.

## 10. Open Decisions

- minimum stake;
- validator set size;
- delegation model;
- commission;
- unbonding period;
- key rotation;
- reward distribution;
- slashing penalties.
