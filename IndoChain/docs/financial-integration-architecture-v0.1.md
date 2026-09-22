# IndoChain Financial Integration Architecture v0.1

## Purpose

Define the intended relationship between IndoChain and financial applications in the DesKaEcosystem, especially DesKaCash, so future implementation does not accidentally treat IndoChain as the operational database for every end-user financial transaction.

This document is an architectural decision record for the `indochain-v0.1` development line.

## Core Decision

IndoChain is the **decentralized blockchain and settlement layer** of the DesKaEcosystem.

Financial applications such as `DesKaCash`, `DesKaBank`, `DesKaCard`, and future DesKa services are **application/financial layers**. They may maintain their own operational financial ledger and use IndoChain for settlement, interoperability, native-asset ownership, and other blockchain-level functions.

IndoChain MUST NOT be designed on the assumption that every end-user financial operation inside a DesKa application must become an on-chain transaction.

## Two Different Ledger Roles

### 1. Application / Financial Ledger

A financial application may maintain an operational ledger for:

- user accounts
- application balances
- internal transfers
- merchant transactions
- payment records
- fees charged by the application
- accounting and reconciliation data
- KYC/account metadata
- application-specific business rules

This ledger belongs to the application domain and may use its own database/storage technology.

For example:

~~~text
DesKaCash
│
├── Alice      100 dIDR
├── Bob        250 dIDR
├── Merchant   500 dIDR
└── ...
~~~

These records do not automatically imply that Alice, Bob, and Merchant each have independent IndoChain addresses.

### 2. IndoChain Ledger

IndoChain maintains the blockchain-level state for native assets, settlement, cryptographic ownership, and protocol-defined transactions.

For an application-controlled balance model, IndoChain may see a service/treasury account such as:

~~~text
DesKaCash Treasury
└── 1,000,000 dIDR
~~~

while DesKaCash maintains the allocation of that balance across its application users.

## DesKaCash User Model

DesKaCash is not required to expose IndoChain private keys or native blockchain addresses to ordinary application users.

A DesKaCash user can have:

~~~text
DesKaCash User
│
├── application account
├── application balance
└── application transaction history
~~~

without storing:

~~~text
private_key
~~~

in the DesKaCash user database.

A user private key MUST NOT be stored as ordinary user data in PostgreSQL, Redis, or equivalent application databases.

If the financial service controls blockchain accounts, the signing key material belongs to the service infrastructure and MUST be isolated from the application database. Future infrastructure MAY use a dedicated signer, KMS, HSM, or equivalent secure key-management architecture.

## IndoChainWallet Is Different

`IndoChainWallet` is a native blockchain wallet.

Its purpose is direct interaction with IndoChain and therefore its security model includes blockchain key material, address, signing, nonce, and native dIDR operations.

The distinction is:

~~~text
IndoChainWallet
= direct blockchain wallet
= user controls/signs with blockchain key material

DesKaCash / DesKaBank / DesKaCard
= financial application
= blockchain details abstracted from ordinary users
= application ledger + IndoChain settlement
~~~

## Internal Transfers

A transfer between two users of the same financial service does not have to be an IndoChain transaction.

Example:

~~~text
Alice (DesKaCash)
    │
    │ 10 dIDR
    ▼
Bob (DesKaCash)
~~~

The application may process this as an internal ledger transaction:

~~~text
Alice balance:  100 dIDR →  90 dIDR
Bob balance:     50 dIDR →  60 dIDR
~~~

There is no requirement to create:

~~~text
Alice address → Bob address
~~~

on IndoChain for every such operation.

This keeps high-volume application activity inside the system designed to handle the application's operational needs.

## Settlement

IndoChain is used when an application needs blockchain settlement.

Examples include:

- movement of dIDR between service-controlled accounts
- settlement between different DesKa financial services
- movement between a financial service and a native IndoChain wallet
- reserve/treasury operations
- interoperability with other blockchain participants
- protocol-level transactions explicitly defined by IndoChain

Conceptually:

~~~text
DesKaCash internal ledger
        │
        │ settlement
        ▼
DesKaCash Treasury / Service Account
        │
        ▼
     IndoChain
~~~

A financial service may settle in batches or at other protocol/application-defined boundaries rather than submitting one blockchain transaction for every internal user operation.

The exact batching and reconciliation protocol is a future design item and MUST NOT be assumed to be implemented by `v0.1` yet.

## Transaction Fees

A user-visible application transaction fee and an IndoChain protocol fee are separate concepts.

An internal DesKaCash transfer can have:

~~~text
User-visible blockchain fee: 0
IndoChain transaction: none
~~~

because the operation is internal to DesKaCash.

For transactions that do reach IndoChain, the protocol may require a fee.

A future IndoChain implementation MAY support application-sponsored transactions, where:

~~~text
Sender:      application/service user
Recipient:   blockchain account
Fee payer:   financial service / relayer
~~~

This would allow a financial application to expose a zero-fee user experience while the blockchain fee is paid by an authorized service account.

This is a protocol design direction, not a claim that fee sponsorship is already implemented in `v0.1`.

## Recommended Service Account Model

Financial applications SHOULD be designed around explicit service-controlled blockchain accounts rather than creating a blockchain private key for every application user by default.

Possible account roles include:

~~~text
DesKaCash
│
├── Treasury Account
├── Settlement Account
├── Reserve Account
└── Operational/Relayer Account
~~~

Exact account roles, permissions, and signing policies remain subject to future protocol and security design.

## Accounting Invariant

If DesKaCash represents user balances as claims against service-controlled on-chain funds, the application MUST define and enforce reconciliation between:

~~~text
Application Ledger
        ↕
Settlement / Reserve Accounting
        ↕
IndoChain On-chain State
~~~

The specific legal, reserve, redemption, custody, and accounting model is outside the scope of this document and must be specified separately before production financial deployment.

## dIDR

The native IndoChain asset is **dIDR — decentralized Indonesian Rupiah**.

Current project denomination:

~~~text
1 dIDR     = Rp1.000
0.1 dIDR   = Rp100
0.01 dIDR  = Rp10
0.001 dIDR = Rp1
~~~

This denomination is a project/protocol design rule. It does not by itself define a fiat backing, redemption guarantee, legal status, or economic peg.

Financial applications MAY present dIDR in Rupiah-oriented UI and application accounting while preserving the canonical blockchain denomination required by IndoChain.

## What IndoChain Must Not Assume

IndoChain architecture MUST NOT assume:

1. Every DesKaCash user needs an IndoChain private key.
2. Every DesKaCash user needs an independent on-chain address.
3. Every internal financial transfer must be broadcast to the blockchain.
4. DesKaCash must store user private keys in its database.
5. A zero-fee user experience means the IndoChain protocol transaction itself has no cost.
6. DesKaCash, DesKaBank, or DesKaCard business logic belongs inside IndoChain core.

## Integration Boundary

Financial applications integrate through documented IndoChain APIs/protocols, not by importing IndoChain internal packages.

Preferred integration paths remain:

1. JSON-RPC
2. WebSocket/event interfaces
3. stable public Go APIs where explicitly promoted
4. protocol test vectors and canonical serialization

This follows the existing IndoChain module and integration boundary.

## Architectural Summary

~~~text
                         DesKaEcosystem
                               │
                    ┌──────────┴──────────┐
                    │                     │
              IndoChainWallet       Financial Services
                    │                (DesKaCash/etc.)
                    │                     │
             direct blockchain      application ledger
                    │                     │
                    │                settlement only
                    │                     │
                    └──────────┬──────────┘
                               ▼
                          IndoChain
                    decentralized settlement
                         + native dIDR
~~~

## Status

This document records the intended architecture for `indochain-v0.1`.

It defines boundaries and design direction. It does not claim that all components described here are already implemented.

Future implementation documents MUST reference this document when introducing financial-service accounts, settlement, transaction sponsorship, custody/signing infrastructure, or DesKa application integration.
