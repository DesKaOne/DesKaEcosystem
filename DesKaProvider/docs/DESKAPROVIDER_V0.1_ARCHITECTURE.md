# DesKaProvider v0.1 Architecture

## 1. Overview

DesKaProvider is the provider gateway/integration layer between DesKaCash and external payment, PPOB, payout, and future provider infrastructure.

```text
DesKaCash
    |
    v
DesKaProvider
    |
    +-- Router
    +-- Provider Registry
    +-- Balance Sync
    +-- Health / Operational State
    +-- Webhook Normalizer
    +-- Provider Adapters
    |
    +-- IAK
    +-- PortalPulsa
    +-- XP SINDONESIA
    +-- Digiflazz
    +-- RCB
    +-- future providers
```

## 2. Separation of Responsibilities

### DesKaCash

Owns:

- customer wallet/account domain
- customer ledger
- customer balance
- transaction business semantics
- reservations/holds
- P2P/internal transfers
- customer-facing business rules

### DesKaProvider

Owns:

- external provider API integration
- provider credentials/configuration
- provider request/response mapping
- provider status mapping
- provider transaction references
- callback/webhook normalization
- provider balance synchronization
- provider health
- provider routing/failover
- provider operational liquidity information

## 3. Provider Balance Synchronization

A background worker periodically calls each provider's balance endpoint and stores the latest operational snapshot.

```text
Provider Balance Worker
        |
        +--> IAK API
        +--> PortalPulsa API
        +--> Digiflazz API
        +--> other enabled providers
        |
        v
provider_balances
```

DesKaCash and the router can read the cached snapshot without repeatedly calling external provider APIs.

The sync interval is configurable per provider. Initial target: 30–60 seconds.

The v0.1 foundation separates the balance capability from the transaction contract:

```go
type BalanceProvider interface {
    GetBalance(context.Context) (int64, error)
}
```

This is an optional provider capability. Providers that do not expose a verified balance operation remain valid `PPOBProvider` implementations.

Operational snapshots currently contain:

- provider name
- balance
- currency
- health state
- last checked time
- last successful synchronization time
- last synchronization error
- consecutive failure count

The current implementation provides both a thread-safe in-memory store for deterministic tests and a durable JSON file store with atomic replacement for single-process development/runtime persistence. The JSON store is an interim v0.1 persistence implementation; PostgreSQL remains the deployment-direction persistence target.

Health transitions are deterministic:

- successful synchronization -> `healthy`, failures reset to zero;
- synchronization failure before the configured threshold -> `degraded`;
- synchronization failure at or above the configured threshold -> `unhealthy`.

A failed synchronization does not erase the last known balance.

## 4. Routing Model

Routing is provider-neutral:

```text
Purchase Request
      |
      v
Provider Registry
      |
      v
Provider Router
      |
      +-- capability/product check
      +-- balance snapshot check
      +-- health check
      +-- priority/cost
      +-- failover policy
      |
      v
Provider Adapter
      |
      v
External Provider
```

The provider adapter remains responsible for translating the neutral request into the provider's protocol.

## 5. Liquidity Model

Provider liquidity and customer funds are separate concepts.

```text
DesKaCash customer funds
        !=
DesKaProvider provider deposits
```

The admin dashboard may compare customer-balance aggregates with expected provider outflow and safety-buffer requirements, but a simple customer-balance-minus-provider-balance calculation must not be treated as an automatic funding obligation.

## 6. Admin Operations

Admin-only capabilities should include:

- view total provider liquidity
- view provider-specific balances
- view last sync and health state
- inspect liquidity warnings
- inspect provider routing state
- manually record/execute provider funding according to an explicitly authorized workflow

No ordinary customer API should expose provider balances, credentials, routing internals, or liquidity data.

## 7. Reliability Principles

- external provider failures must not corrupt the DesKaCash ledger
- provider callbacks must be idempotent
- transaction correlation IDs must be preserved end-to-end
- retries must respect provider semantics
- provider-specific status codes stay inside adapters
- cached provider balances are advisory operational data
- final transaction state comes from the provider transaction flow, not only the balance cache

## 8. Deployment Direction

A VPS deployment is compatible with providers that require a stable public IP.

```text
Internet
   |
   v
[ VPS / DesKaProvider ]
   |
   +-- HTTPS API
   +-- Provider adapters
   +-- Background workers
   +-- PostgreSQL
   +-- Secret configuration
   |
   +-- static public IPv4
```

Database ports should not be exposed publicly. Provider credentials should be stored outside source control.

## 9. Future Open API Direction

DesKaProvider may later expose a stable public API to external developers.

```text
Developer / DesKaCash
        |
        v
DesKaProvider API v1
        |
        +-- Payment
        +-- PPOB
        +-- Payout
        +-- future capabilities
        |
        v
External Providers / DesKa Infrastructure
```

This is a future direction, not part of the initial v0.1 implementation.

## 10. v0.1 Provider Contract Notes

The provider-neutral PPOB contract intentionally carries the minimum transaction identity required to preserve correlation across external providers:

- product code
- customer number
- provider-neutral reference ID
- normalized transaction status
- provider response code/message
- optional serial number and price

For status checks, the neutral `StatusRequest` retains the original product code, customer number, and reference ID. This is required because DigiFlazz's documented prepaid status flow repeats the topup request with the same `ref_id`, rather than exposing a separate prepaid status endpoint.

For webhooks, the neutral `WebhookRequest` carries body and authentication context so provider adapters can validate signatures before normalizing the event. DigiFlazz currently uses `X-Hub-Signature` with HMAC-SHA1 when a webhook secret is configured.

These fields are provider-neutral at the DesKaProvider boundary; DigiFlazz-specific JSON field names, signature construction, response codes, and HTTP behavior remain inside the adapter.

## 11. Balance Adapter Boundary

A concrete balance adapter must only be implemented after the provider's balance endpoint and response contract are verified from its API documentation.

The current source API catalog lists provider documentation URLs but does not define a verified balance endpoint for the active v0.1 provider set. Therefore:

- the balance capability is defined now;
- operational state and health handling are implemented now;
- provider-specific balance HTTP calls remain pending;
- no provider balance is fabricated from transaction responses;
- the future worker will invoke only providers that implement `BalanceProvider`.

This preserves the separation between verified provider API behavior and the provider-neutral operational model.


## 12. v0.1 Provider Roadmap Update

**Date:** 2026-09-24

The implementation sequence is now fixed for the current v0.1 scope:

```text
DigiFlazz → IAK → XP SINDONESIA → Midtrans → RCB
```

DigiFlazz is the current implementation focus. It may be considered sufficient to move forward at either a stable **50% checkpoint** or **100% completion**. PortalPulsa is explicitly skipped for this sequence.

Provider verification is tracked independently from adapter implementation:

- **Verified/active:** IAK, Midtrans, XP SINDONESIA
- **Verification pending:** RCB
- **Verification deferred until the application is running / required business evidence is available:** DOKU, Ezeelink, Digiflazz

The provider-neutral architecture remains unchanged: provider-specific protocols stay inside adapters, while DesKaCash consumes a stable provider-neutral boundary.


## 13. Transaction Persistence Concurrency Boundary

**Date:** 2026-09-25

The v0.1 transaction-store contract distinguishes ordinary persistence from an atomic compare-and-transition capability.

The provider-neutral persistence boundary is:

```go
type AtomicTransactionStore interface {
    TransactionStore
    PutIfCurrent(referenceID string, previous, next TransactionState) error
}
```

The atomic operation must verify the expected current state and apply the next state as one conditional transition. A database-backed implementation must map this to a single database transaction or conditional update so concurrent DesKaProvider processes cannot overwrite a transaction based on stale state.

Required invariants:

- transaction reference ID remains stable;
- original purchase request identity remains stable;
- selected provider identity remains stable;
- only `pending -> success/failed` transitions are accepted;
- terminal states are immutable except for idempotent identical writes;
- a stale expected state must fail the atomic transition rather than overwrite the newer state;
- concurrent reconciliation of the same terminal provider result must remain idempotent.

The current memory and JSON stores implement this boundary for deterministic single-process behavior. The JSON store remains an interim persistence implementation and does not provide cross-process file locking. PostgreSQL remains the deployment-direction target for production transaction persistence.

No retry/failover policy is derived from this contract. Atomic persistence protects transaction identity and state transitions; it does not authorize a second provider submission.


## 14. PostgreSQL Transaction-Store Schema & Conditional Transition Boundary

**Date:** 2026-09-25

Milestone #77 defines the concrete PostgreSQL persistence shape without introducing a database driver into the current Go module.

The schema artifact is:

`DesKaProvider/backend/migrations/001_provider_transactions.sql`

The table `provider_transactions` stores the minimum durable identity needed to reconstruct a pending purchase after restart:

- `reference_id` — primary key and stable transaction identity;
- `product_code`, `customer_no`, `amount`, `testing` — original purchase request identity;
- `provider_name` — selected provider identity, immutable for the transaction;
- `status` — constrained to `pending`, `success`, or `failed`;
- provider result fields — `provider_code`, `message`, `serial_number`, `price`;
- `version` — optimistic-concurrency version, incremented exactly once per accepted transition;
- `created_at`, `updated_at` — persistence timestamps.

The production database adapter must map `AtomicTransactionStore.PutIfCurrent` to one PostgreSQL transaction or one conditional `UPDATE ... WHERE reference_id = ? AND version = ? ...` operation. The conditional update must also verify the original request identity and selected provider.

A zero-row conditional update is a concurrency/state conflict. The adapter must reload the current state and follow the existing reconciliation conflict semantics; it must never interpret the conflict as permission to submit the provider purchase again.

Terminal states are immutable. A repeated identical terminal observation is handled idempotently by the service/reconciliation layer rather than mutating the terminal record into a different result.

The schema deliberately does not add retry counters, failover state, automatic funding fields, or customer-ledger postings. Those concerns remain outside this persistence boundary.

PostgreSQL row-level locking/conditional-update behavior is consistent with the database's concurrency model: concurrent updates to the same row are serialized by PostgreSQL, and the update predicate is re-evaluated against the current row version.

Current limitation: the SQL migration is a concrete schema/locking contract only. The Go PostgreSQL adapter, connection management, migration runner, and live PostgreSQL integration tests are not yet implemented.
