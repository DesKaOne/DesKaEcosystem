# IndoChain Faucet

> Status: Draft / Devnet & Testnet Only

## 1. Purpose

The faucet distributes test dIDR for development and testing.

## 2. Network Restriction

The faucet is explicitly limited to:

- Devnet;
- Testnet.

The production Mainnet must not expose a public faucet distribution mechanism.

The service should enforce network identity at the protocol/API layer so a deployment cannot accidentally fund Mainnet.

## 3. Basic Flow

~~~text
Developer
   ↓
Select Network
   ↓
Provide Test Address
   ↓
Validate Address / Network
   ↓
Rate Limit / Quota
   ↓
Create Faucet Transaction
   ↓
Submit to RPC
   ↓
Track Inclusion
   ↓
Return Transaction ID
~~~

## 4. Anti-Abuse

Candidate controls:

- per-address cooldown;
- per-IP/request quota;
- daily allocation limit;
- configurable claim amount;
- request logging;
- optional CAPTCHA or challenge;
- optional authentication for private devnet faucet.

Exact limits are deployment configuration, not consensus rules.

## 5. Faucet Accounts

The faucet should use a dedicated funded account/key with restricted permissions.

Private keys must not be stored in source code, public configuration, logs, or frontend bundles.

## 6. Network Safety

The faucet must validate:

- expected chain ID;
- expected network profile;
- destination address format;
- node RPC endpoint;
- transaction result.

A production deployment should fail closed if network identity does not match the configured Devnet/Testnet identity.

## 7. API Direction

Possible endpoints:

~~~text
GET  /health
GET  /network
POST /claim
GET  /claim/{id}
~~~

Exact API format will be defined during service implementation.

## 8. Observability

Track:

- claim count;
- successful/failed claims;
- rate-limit events;
- distribution amount;
- RPC failures;
- transaction confirmation latency.

## 9. Devnet Mode

Devnet faucet may support:

- higher claim limits;
- local/private access;
- automated test accounts;
- reset on network recreation.

These capabilities must never be copied into Mainnet deployment configuration.

## 10. Testnet Mode

Testnet faucet should use stricter public quotas and operational monitoring.

## 11. Security Requirements

- isolated faucet wallet;
- encrypted secret management;
- rate limiting;
- input validation;
- network identity checks;
- audit logs without private key material;
- explicit Mainnet-disabled configuration.
