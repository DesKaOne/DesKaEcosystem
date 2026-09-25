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


## 15. PostgreSQL AtomicTransactionStore Adapter

**Date:** 2026-09-25

Milestone #78 adds the first concrete PostgreSQL adapter behind the existing provider-neutral AtomicTransactionStore contract:

DesKaProvider/backend/routing/postgres_transaction_store.go

The adapter deliberately uses the standard library database/sql boundary and does not select or embed a PostgreSQL driver. The runtime may inject a PostgreSQL-compatible database/sql implementation without changing the routing/service contract.

The adapter provides:

- Get for durable transaction reconstruction;
- Put for initial insertion and pending-to-terminal transition semantics;
- All for restart recovery;
- PutIfCurrent for atomic compare-and-transition.

The atomic transition uses the migration's conditional update shape and treats zero affected rows as ErrTransactionStateConflict. The predicate includes reference ID, version, original request identity, selected provider, and status='pending'.

The adapter does not retry a failed transition, resubmit a provider purchase, mutate the customer ledger, or perform provider funding.

Because the existing TransactionStore interface is intentionally context-free, the concrete adapter currently uses context.Background() for store operations. A future contract revision may introduce explicit context propagation if the repository standard requires it.

The adapter's unit tests use a deterministic database stub to verify successful atomic transition, zero-row conflict behavior, and request-identity rejection. Live PostgreSQL integration remains a separate verification boundary.


## 16. PostgreSQL Integration Harness & Real Concurrency Verification Boundary

**Date:** 2026-09-25

Milestone #79 adds an isolated PostgreSQL integration harness for the concrete transaction store.

The harness:

- uses github.com/jackc/pgx/v5/stdlib only from the integration-test package to exercise the existing database/sql adapter boundary;
- reads DESKAPROVIDER_POSTGRES_DSN at runtime and skips when no database is configured;
- loads and verifies the repository migration artifact before database execution;
- applies the migration to the isolated test database;
- verifies durable pending insertion and reconstruction;
- runs two concurrent PutIfCurrent calls against the same pending state and requires exactly one success plus one ErrTransactionStateConflict;
- reconnects to PostgreSQL and verifies terminal-state recovery after the original database handle is closed;
- verifies the migration contains the required primary key, status constraint, optimistic-concurrency version, and conditional-transition predicates.

CI provisions a dedicated PostgreSQL service for both the normal test and race jobs and supplies only an ephemeral test DSN. No production credentials or provider secrets are involved.

The harness is verification infrastructure only. It does not change provider routing, retry/failover policy, transaction resubmission behavior, customer-ledger mutation, or provider funding.

The current adapter still has the context-free TransactionStore contract and therefore uses context.Background(). Explicit context propagation remains a separate contract decision.


## 17. Context-Aware Persistence Boundary

**Date:** 2026-09-25

Milestone #80 adds an optional context-aware persistence contract without removing the existing provider-neutral TransactionStore interface.

ContextTransactionStore extends the persistence boundary with GetContext, PutContext, AllContext, and PutIfCurrentContext.

The routing service prefers the context-aware methods when the configured store implements the contract. Existing stores remain compatible through the original interface, and legacy fallback paths remain available.

The PostgreSQL adapter now passes the caller context to database/sql operations instead of hard-coding context.Background() on context-aware service paths. The context-free methods remain compatibility wrappers using context.Background().

Cancellation and deadlines therefore reach PostgreSQL persistence operations on purchase, webhook, and reconciliation paths. Context cancellation does not authorize retry, failover, provider resubmission, ledger mutation, or provider funding.

The constructor startup All() recovery path remains context-free because it is initialization work rather than a request-scoped operation. Explicit startup lifecycle context can be considered separately if runtime startup/shutdown requires it.


## 18. Context Cancellation & Deadline Persistence Hardening

**Date:** 2026-09-25

Milestone #81 verifies the behavioral safety of the ContextTransactionStore boundary under canceled and expired request contexts.

The deterministic tests establish that:

- canceled memory-store reads do not expose transaction state through GetContext or AllContext;
- canceled/expired memory-store mutation calls return the context error before changing transaction state;
- PostgreSQL PutIfCurrentContext receives the caller context at the database/sql boundary;
- a canceled persistence operation does not become a successful state transition.

The cancellation boundary does not authorize any second provider submission. A caller receiving context.Canceled or context.DeadlineExceeded must continue to treat the transaction reference as requiring the existing durable-state/reconciliation rules, not as permission to retry the provider purchase.

The current context-aware read signatures remain a known observability limitation:

    GetContext(ctx context.Context, referenceID string) (TransactionState, bool)
    AllContext(ctx context.Context) []TransactionState

These signatures cannot distinguish cancellation/database failure from not-found/empty results. This is intentionally documented as a follow-up contract decision rather than silently changing the provider-neutral interface in the hardening milestone.

The PostgreSQL write path already propagates context to database/sql; future contract work may add explicit read errors if required by service/recovery semantics.


## 19. Context-Aware Read Error Observability

**Date:** 2026-09-25

Milestone #82 adds an additive read-error-aware extension:

    type ContextReadTransactionStore interface {
        ContextTransactionStore
        GetContextE(ctx context.Context, referenceID string) (TransactionState, bool, error)
        AllContextE(ctx context.Context) ([]TransactionState, error)
    }

The existing ContextTransactionStore remains unchanged for compatibility. Memory and PostgreSQL stores implement the extension.

The PostgreSQL adapter now preserves database read failures on the new methods instead of collapsing them into not-found/empty results. The existing GetContext and AllContext methods remain compatibility wrappers and retain their original state-only behavior.

Service reconciliation conflict recovery uses the error-aware read path when available. If the durable reload fails because of cancellation or a database error, reconciliation returns that error rather than interpreting the missing state as a reference conflict. This distinction is important for recovery safety: a read failure never authorizes a second provider purchase.

Startup recovery remains on the context-free All() constructor path. This is the next explicit boundary to harden so startup database failures cannot be mistaken for an empty transaction set.


## 20. Context-Aware Startup Recovery Boundary

**Date:** 2026-09-25

Milestone #83 adds an explicit initialization-context constructor: NewServiceWithStoreContext(ctx, router, store).

When the store implements ContextReadTransactionStore, service startup loads durable transactions through AllContextE. Database/read errors are returned as initialization failures rather than being collapsed into an empty transaction set.

The existing NewServiceWithStore remains compatible and deliberately supplies context.Background() as its initialization context. This keeps existing callers stable while providing an explicit migration path for production composition roots that own startup cancellation and timeout policy.

A canceled initialization context fails before transaction reconstruction. No provider call is made and no transaction is resubmitted.

Legacy stores without context-aware reads continue through their existing All() path. This is a compatibility boundary, not a claim of context-aware startup behavior for legacy implementations.


## 21. Production Startup Context Wiring

**Date:** 2026-09-25

Milestone #84 connects the process lifecycle context to service initialization.

The production command creates the signal-aware context before runtime composition, then passes it through NewFromEnvironmentContext into NewServiceWithStoreContext. When the configured transaction store implements ContextReadTransactionStore, startup persistence reads therefore participate in process lifecycle cancellation.

NewFromEnvironment remains a compatibility wrapper and supplies context.Background(). The explicit context-aware constructor is the production composition path.

The runtime still selects the JSON transaction store today. Therefore this milestone does not claim PostgreSQL-backed runtime startup verification; the existing PostgreSQL integration harness remains the persistence adapter verification boundary. PostgreSQL runtime selection and its startup failure/cancellation integration are deferred to the next persistence-composition milestone.


## 22. Production PostgreSQL Transaction-Store Selection

**Date:** 2026-09-25

Milestone #85 establishes the production persistence selection boundary without leaking database details into routing.

Configuration:

- DESKAPROVIDER_TRANSACTION_STORE_DRIVER=json (default) selects the existing JSON store;
- DESKAPROVIDER_TRANSACTION_STORE_DRIVER=postgres selects PostgresTransactionStore;
- DESKAPROVIDER_POSTGRES_DSN is mandatory for the PostgreSQL selection.

Runtime composition performs sql.Open followed by PingContext using the same initialization context that flows into NewServiceWithStoreContext. A failed database connection therefore aborts startup before transaction reconstruction.

The routing/service layer remains dependent only on the provider-neutral TransactionStore and its context-aware extensions. PostgreSQL-specific connection lifecycle is contained in runtime composition.

The runtime service closes the database handle on shutdown. No retry or automatic reconnection policy is introduced in this milestone.


## 23. PostgreSQL Restart Recovery & Reconciliation Verification

**Date:** 2026-09-25

Milestone #86 verifies the durable pending transaction boundary end-to-end through the routing service.

A real PostgreSQL store is populated by Service.Purchase, leaving the provider result pending. A new Service instance is then constructed from the same PostgreSQL store. Startup reconstruction uses AllContextE, recreates the pending transaction call state, and Reconcile queries the selected provider status without invoking Purchase again.

The integration assertion requires the mock provider purchase count to remain exactly one across the reconstruction and reconciliation step. This establishes the intended restart invariant:

durable pending state -> reconstruct -> reconcile -> no second provider submission

No retry, failover, customer-ledger mutation, or automatic funding behavior is implied by this boundary.


## 24. PostgreSQL Terminal-State Recovery & Stale Transition Hardening

**Date:** 2026-09-25

Milestone #87 extends the real PostgreSQL verification boundary from pending recovery to terminal-state recovery.

The integration contract now verifies:

- a successful provider result is durably persisted;
- a new Service instance reconstructs the terminal transaction from PostgreSQL;
- reconciliation of an already-terminal transaction is idempotent when the provider reports the same result;
- the provider purchase operation is not called again after restart;
- an identical terminal compare-and-transition is accepted without changing the durable result;
- a stale pending transition cannot overwrite the terminal row and returns ErrTransactionStateConflict;
- a terminal transition to a different result is rejected as a transaction reference/state conflict.

The safety invariant is:

pending -> terminal is accepted once by the durable conditional boundary; stale pending -> terminal attempts after restart are conflicts, while identical terminal observations remain idempotent.

This milestone does not add retry counters, failover state, automatic resubmission, customer-ledger postings, or provider funding behavior. PostgreSQL persistence remains an internal transaction-state boundary, not a financial ledger.

## 25. PostgreSQL Failed-State Recovery & Webhook Convergence

**Date:** 2026-09-25

Milestone #88 extends restart verification to terminal failed transactions.

The real PostgreSQL integration boundary now verifies:

- failed provider results are durably persisted;
- a new Service instance reconstructs the failed transaction;
- reconciliation of an already-failed transaction is idempotent;
- the provider purchase operation is not called again;
- an identical terminal failed webhook is accepted as an idempotent convergence event;
- the durable failed result remains unchanged through restart, reconciliation, and webhook handling.

The invariant is:

`terminal failed + identical observation -> same terminal state, no provider submission`

A terminal webhook is therefore part of state convergence only. It is not a trigger for retry or failover.

Conflicting terminal observations remain subject to the existing reference-conflict boundary and do not authorize a provider purchase.

No retry counters, failover state, automatic resubmission, customer-ledger postings, or provider funding are introduced.


## 26. PostgreSQL Terminal Conflict Recovery After Restart

**Date:** 2026-09-25

The PostgreSQL transaction-store integration boundary now verifies that terminal conflicts remain non-resubmitting after service reconstruction.

### Verified flow

Service.Purchase -> PostgreSQL terminal SUCCESS -> Service reconstruction -> conflicting provider status or conflicting terminal webhook -> ErrWebhookReferenceConflict -> no provider Purchase -> durable terminal state unchanged.

The test uses the real PostgresTransactionStore, an isolated PostgreSQL schema, and the deterministic mock provider. The provider's recorded transaction observation is deliberately changed after reconstruction to model a conflicting external observation without introducing provider-specific retry behavior.

### Safety invariant

A persisted terminal transaction is authoritative for local transaction identity and provider selection. A later conflicting reconciliation or webhook observation is a conflict signal requiring investigation/reconciliation, not authorization to submit the provider transaction again.

The conflict boundary guarantees:

- terminal SUCCESS/FAILED is not overwritten by a conflicting observation;
- ErrWebhookReferenceConflict is returned for service-level convergence conflicts;
- provider Purchase is never called by conflict handling;
- provider identity and request identity remain immutable;
- no ledger mutation or automatic funding is performed by this boundary.

### Verification boundary

Real PostgreSQL integration coverage verifies the durable terminal state before and after service reconstruction and conflicting observations. CI test, vet, race, and PostgreSQL integration must remain GREEN before this milestone is considered closed.

### Limitation

The integration test models a conflicting provider observation using the deterministic mock provider. It does not claim to reproduce the exact behavior of an external provider after a physical process crash or network partition. Provider-specific conflict semantics remain outside the provider-neutral contract.


## 27. Transaction Audit Persistence Boundary

**Date:** 2026-09-26

DesKaProvider now defines an additive append-only transaction audit boundary separate from TransactionState and the financial ledger.

### Contract

```text
Transaction lifecycle / reconciliation observation
                  |
                  v
       TransactionAuditStore
                  |
          append-only record
                  |
                  v
      operational audit history
```

The provider-neutral record contains reference identity, action, previous/next transaction status, provider identity, message, and creation timestamp. It is operational history only; it is not a balance projection, ledger posting, or authorization record.

### Persistence invariant

Audit rows are append-only. The PostgreSQL migration creates `provider_transaction_audit` with a generated audit identifier and a reference/time index. The application contract intentionally exposes append and filtered read only; no update or delete operation is provided.

### Financial safety invariant

Audit persistence is deliberately separated from TransactionState persistence. Adding an audit record cannot transition a transaction, mutate a customer ledger, move provider liquidity, or authorize a provider Purchase call. Future event wiring must preserve this invariant even when an audit write fails.

### Current boundary

Milestone #90 verifies the contract and migration shape but does not yet wire Service lifecycle events or select a PostgreSQL audit adapter at runtime. Those are deferred until the event taxonomy and failure semantics are explicitly verified.

## 28. Transaction Lifecycle Audit Event Wiring

**Date:** 2026-09-26

Milestone #91 wires the provider-neutral TransactionAuditStore into the routing service without making audit persistence part of transaction authorization.

### Event boundary

The service emits operational audit observations for:

- PURCHASE_PENDING when the durable pending state is persisted before provider submission;
- PURCHASE_RESULT after the provider result is durably persisted;
- PURCHASE_PROVIDER_ERROR when provider execution fails while the durable transaction remains pending;
- PURCHASE_RESULT_PERSIST_FAILURE when the provider result cannot be durably committed and the pending state remains authoritative;
- WEBHOOK_PENDING for a pending webhook transition;
- WEBHOOK_TERMINAL for a terminal webhook transition;
- WEBHOOK_TERMINAL_CONFLICT for a conflicting terminal webhook observation;
- RECONCILIATION for a durable reconciliation transition;
- RECONCILIATION_TERMINAL_CONFLICT for a conflicting terminal reconciliation observation.

The event taxonomy records reference identity, previous/next status, provider identity, message, and creation time. It remains provider-neutral.

### Audit failure semantics

Audit writes are observational and cannot authorize a provider operation.

    persist transaction state
            |
            +--> audit append fails
            |       |
            |       +--> transaction state remains authoritative
            |       +--> no rollback to provider state
            |       +--> no retry/failover/resubmission
            |
            v
    existing transaction safety rules continue

For the purchase path, the pending audit failure does not block the first provider submission because the pending transaction state is already durable. If the terminal transaction has already been durably persisted and the terminal audit append fails, the service returns the durable terminal result together with the audit error; it does not revert the transaction to pending and does not authorize a second provider submission. A repeated purchase uses the existing transaction correlation and therefore does not submit the provider again.

For webhook and reconciliation transitions, the durable state is committed before the audit append. An audit error is therefore reported without reverting the committed transaction state.

### Construction boundary

Existing service constructors now install an in-memory audit store by default for deterministic/local operation. An additive constructor accepts an explicit TransactionAuditStore, allowing future durable implementations without changing routing or provider contracts.

### Financial safety

Audit history is not a financial ledger and cannot mutate:

- customer balances or ledger postings;
- provider operational balance snapshots;
- treasury state;
- transaction authorization state.

The audit store remains append-only and has no update/delete contract.

### Current limitation

Milestone #91 does not add a PostgreSQL audit adapter or production runtime selection. PostgreSQL audit persistence remains the next boundary after event semantics and failure behavior are stable.


## 29. PostgreSQL Transaction Audit Store Adapter

**Date:** 2026-09-26

Milestone #92 adds a PostgreSQL implementation of the provider-neutral TransactionAuditStore boundary.

The adapter:

- uses the existing database/sql DBTX boundary;
- inserts audit events only;
- reads events by reference ID ordered by created_at and audit_id;
- propagates context cancellation/deadline errors through context-aware methods;
- performs no UPDATE or DELETE operation;
- does not participate in transaction-state authorization or provider submission decisions.

The adapter is intentionally separate from PostgresTransactionStore. A transaction-state write can therefore remain authoritative even when an audit append fails.

### Durability and restart boundary

Real PostgreSQL integration coverage verifies:

1. two audit events are appended;
2. a new adapter instance reads the same durable events;
3. event ordering and contents remain unchanged;
4. unrelated reference IDs do not leak events;
5. the audit table remains append-only at the application contract level.

### Failure safety

The integration boundary also verifies that a canceled audit append does not mutate an already durable terminal transaction and cannot authorize a second provider submission.

Production runtime selection of PostgreSQL audit storage remains deferred until the audit failure-injection matrix for webhook and reconciliation is complete.


## 30. Webhook & Reconciliation Audit Failure Injection Hardening

**Date:** 2026-09-26

Milestone #93 closes the remaining deterministic audit-failure coverage gap for webhook and reconciliation transitions.

### Verified invariant

For both paths:

```
provider observation
      |
      v
persist TransactionState
      |
      +--> audit append fails
      |
      v
durable terminal state remains authoritative
      |
      v
no retry / failover / resubmission
```

Dedicated tests inject an unavailable TransactionAuditStore after the provider transaction has already reached a pending durable state. A terminal webhook or reconciliation transition is persisted first, then the audit append fails.

The verification requires:

- the returned execution remains terminal;
- the PostgreSQL-independent transaction store still contains the terminal state;
- provider PurchaseCount remains exactly one;
- a later identical observation converges idempotently without invoking Purchase again.

### Runtime boundary

This milestone does not yet select PostgreSQL audit storage in production. The audit adapter remains independently testable and is not allowed to become part of provider submission authorization.

### Limitation

The failure injection is deterministic at the audit-store abstraction. It does not reproduce a real PostgreSQL connection failure, transaction rollback, or network partition during an INSERT. Those concerns belong to runtime integration coverage in the next milestone.


## 31. Production PostgreSQL Audit-Store Runtime Selection

Milestone #94 establishes the runtime boundary for the transaction audit persistence selected by configuration.

Runtime configuration selects either the in-memory audit store or the PostgreSQL append-only audit store. When both transaction and audit stores use PostgreSQL, they may share the same database connection pool. When only the audit store uses PostgreSQL, runtime opens a dedicated PostgreSQL connection using the configured DSN.

Operational boundaries:

- `DESKAPROVIDER_AUDIT_STORE_DRIVER` accepts `memory` or `postgres`;
- PostgreSQL selection requires `DESKAPROVIDER_POSTGRES_DSN`;
- runtime validates the PostgreSQL connection with `PingContext` before constructing the service;
- runtime closes PostgreSQL resources on shutdown and initialization failure paths;
- migration execution remains outside runtime startup;
- audit persistence remains observational and append-only, separate from financial transaction state.

The audit store does not become a financial source of truth and cannot authorize retry, failover, resubmission, ledger mutation, or provider funding.


## 32. Runtime Durable Audit Reconstruction & Shutdown Integration Hardening

**Date:** 2026-09-26

Milestone #95 verifies that PostgreSQL transaction persistence and PostgreSQL audit persistence survive a service reconstruction boundary together.

The integration sequence is:

1. construct PostgreSQL transaction and audit stores;
2. construct a routing service with both durable stores;
3. execute a successful purchase and persist the pending and terminal lifecycle events;
4. close the PostgreSQL connection;
5. reopen PostgreSQL using the configured DSN and restore the isolated schema context;
6. reconstruct both stores and a new routing service;
7. reconcile the already-terminal transaction;
8. verify the terminal result and audit history are unchanged;
9. process an identical terminal webhook without another provider submission.

The key invariant is that restart/reconstruction is a persistence boundary, not a resubmission boundary. A terminal transaction remains authoritative after restart, and audit history is reconstructed as durable evidence without becoming an authorization source for provider actions.

### Shutdown and failure boundary

The runtime owns PostgreSQL resources opened for transaction and audit persistence. Shared transaction/audit usage reuses the transaction database connection; audit-only PostgreSQL usage owns a dedicated connection. Initialization failures close resources already opened by runtime before returning the error.

No audit read/write failure may authorize retry, failover, resubmission, ledger mutation, or provider funding. Migration deployment remains outside runtime startup.

## 33. PostgreSQL Audit/Transaction Lifecycle Shutdown Failure Hardening

**Date:** 2026-09-26

Milestone #96 strengthens runtime resource ownership after PostgreSQL transaction/audit handles are opened.

The production composition path now establishes a deferred cleanup guard before subsequent initialization steps. If provider-state persistence, state reconstruction, router construction, or service construction fails, runtime closes database handles already acquired instead of returning while retaining open PostgreSQL resources.

When transaction and audit stores share one PostgreSQL database, the transaction database is the resource owner and is closed once. When audit storage is PostgreSQL-only, runtime owns a dedicated audit database handle and closes that handle independently.

The lifecycle boundary is:

```
open transaction DB
        |
        +--> open audit DB (shared or dedicated)
        |
        +--> remaining service initialization
        |
        +--> failure -> close acquired DB resources
        |
        +--> success -> transfer ownership to runtime Service
        |
        +--> shutdown -> close runtime-owned DB resources
```

Cleanup does not participate in transaction-state authorization. A close failure is not a provider result and must not trigger retry, failover, resubmission, ledger mutation, treasury movement, or provider funding. Audit persistence remains separate from the financial source of truth.

Real PostgreSQL integration coverage verifies that both shared and dedicated database handles become unusable after the runtime cleanup helper closes them. This is a resource-lifecycle verification boundary, not a claim about physical process termination or network-partition behavior.

## 34. Runtime Shutdown Error Propagation & Ownership Observability Hardening

**Date:** 2026-09-26

Milestone #97 makes PostgreSQL resource shutdown failures explicit at the runtime lifecycle boundary.

Service.Run now closes runtime-owned transaction/audit database handles through a close-error-aware helper. A close failure is returned to the caller rather than silently discarded. When shutdown already has a primary error, such as context.Canceled, runtime combines the errors so both remain discoverable through errors.Is. If cleanup succeeds, the original primary error is returned unchanged.

Shared transaction/audit database ownership remains deduplicated: one underlying handle is closed once. Dedicated audit storage remains independently owned and closed.

The semantic boundary is:

shutdown trigger
      |
      +--> primary lifecycle error (if any)
      |
      +--> close transaction DB
      |
      +--> close audit DB
      |
      v
lifecycle result
  - primary error preserved
  - close error observable
  - no provider/financial action

A database close failure is not a provider response, transaction result, or authorization signal. It therefore cannot trigger retry, failover, resubmission, ledger mutation, treasury movement, or provider funding.

Deterministic runtime tests verify close-error propagation, preservation of primary cancellation identity, and no double-close for shared handles. PostgreSQL integration continues to verify actual resource closure; deterministic injection covers the error-propagation semantics that a real database shutdown cannot reliably force on demand.


## 35. Runtime Initialization Cleanup Error Observability & Explicit Ownership Diagnostics

**Date:** 2026-09-26

Milestone #98 makes the runtime initialization cleanup boundary observable after PostgreSQL transaction/audit resources have been acquired.

The ownership sequence is:

```
open transaction DB
        |
        +--> open audit DB
        |
        +--> remaining initialization
        |       |
        |       +--> failure -> close acquired resources
        |                       |
        |                       +--> cleanup error observable
        |                       +--> primary init error preserved
        |
        +--> success -> transfer DB ownership to Service
```

### Error semantics

Initialization cleanup uses the same error-composition boundary as shutdown. When initialization fails and cleanup also fails, the returned error preserves both signals so callers can discover the original initialization error and the cleanup error. When cleanup succeeds, the original initialization error remains unchanged.

This distinction is intentional: a cleanup failure does not become a provider transaction result and does not alter transaction authorization semantics.

### Ownership semantics

The deferred initialization guard is active only until runtime construction succeeds. On success, ownership transfers to the runtime Service, which closes the database resources during shutdown. Shared transaction/audit database usage remains de-duplicated so one underlying handle is never closed twice.

### Safety invariant

```
initialization failure
      |
      +--> cleanup acquired resources
      |
      +--> cleanup error (if any) is observable
      |
      +--> no provider retry/failover/resubmission
      +--> no ledger mutation
      +--> no treasury movement
      +--> no provider funding
```

Deterministic tests verify primary-error preservation, cleanup-error observability, and shared-handle de-duplication. PostgreSQL integration continues to verify the underlying resource closure boundary.
