# IndoChain Security

> Status: Draft / Security Design Baseline

## 1. Security Goal

Security must be treated as a protocol requirement from the first implementation, not as a final audit phase.

## 2. Threat Model

Consider:

- malicious validators;
- malicious peers;
- transaction spam;
- malformed blocks;
- consensus equivocation;
- key compromise;
- RPC abuse;
- storage corruption;
- denial of service;
- supply/accounting bugs;
- smart-contract vulnerabilities;
- bridge/oracle risks.

## 3. Trust Boundaries

Important boundaries include:

~~~text
Wallet ↔ RPC
RPC ↔ Node
Node ↔ P2P
P2P ↔ Consensus
Consensus ↔ Execution
Execution ↔ State
Node ↔ Storage
Faucet ↔ Testnet RPC
Explorer ↔ Indexer
~~~

Every boundary requires validation and bounded resource usage.

## 4. Cryptographic Security

Requirements include:

- canonical encoding;
- domain separation;
- strict signature verification;
- replay protection;
- secure key storage;
- no secret leakage in logs;
- shared Go/Dart test vectors.

## 5. Consensus Security

Test:

- equivocation;
- conflicting proposals;
- conflicting votes;
- delayed messages;
- offline validators;
- network partition;
- validator restart;
- malformed consensus messages.

## 6. P2P Security

Protect against:

- connection exhaustion;
- oversized messages;
- malformed messages;
- gossip amplification;
- peer impersonation;
- invalid chain messages;
- synchronization abuse.

## 7. RPC Security

Public endpoints require:

- rate limiting;
- request-size limits;
- method restrictions;
- TLS at deployment boundary;
- authentication for privileged methods;
- separation of public/admin APIs.

## 8. Wallet Security

Wallet requirements include:

- secure private-key storage;
- explicit transaction review;
- domain/chain validation;
- safe signing;
- backup/recovery;
- hardware-wallet support where adopted.

## 9. Faucet Security

Faucet-specific requirements:

- Devnet/Testnet only;
- network identity validation;
- rate limiting;
- isolated faucet account;
- secret management;
- monitoring;
- fail-closed Mainnet protection.

## 10. Smart Contract Security

If VM support is enabled, testing must include:

- reentrancy;
- authorization;
- integer/bounds errors;
- gas exhaustion;
- storage consistency;
- deterministic execution;
- malformed bytecode;
- host-function abuse.

## 11. Testing Strategy

Required categories:

- unit tests;
- integration tests;
- fuzz tests;
- property tests;
- deterministic replay tests;
- network/chaos tests;
- crash/recovery tests;
- adversarial consensus tests;
- interoperability tests.

## 12. Security Release Gate

Before Mainnet:

- protocol review;
- cryptographic review;
- consensus review;
- P2P testing;
- wallet signing tests;
- state/storage recovery tests;
- external security review where feasible;
- incident-response plan;
- backup/recovery drills.
