# DesKaProvider v0.1 Development Status

**Branch target:** `dev/deskaprovider-v0.1`  
**Architecture baseline:** documented on `main`  
**Current state:** IN PROGRESS — provider-neutral foundation and DigiFlazz adapter implemented; live credential validation remains pending because credentials are not stored in the repository or exposed to source control.

## 1. Purpose

DesKaProvider is the provider integration and routing layer for DesKaEcosystem. It isolates external provider APIs from DesKaCash so provider-specific protocols, credentials, status codes, callbacks, pricing, and errors do not leak into the DesKaCash domain.

Initial provider domains:

- Payment / collection
- PPOB / digital products
- Payout / disbursement
- Future wallet/provider capabilities

DesKaCash is the first intended consumer of DesKaProvider.

## 2. Core Boundary

```text
DesKaCash
    |
    | Internal API / service boundary
    v
DesKaProvider
    |
    +-- Provider adapters
    +-- Provider router
    +-- Provider balance cache
    +-- Provider health
    +-- Reconciliation / operational state
    |
    +-- External providers
```

Provider-specific APIs must remain inside adapters.

## 3. Initial PPOB Adapter Direction

Candidate providers currently tracked:

- XP SINDONESIA
- Digiflazz
- IAK
- PortalPulsa
- RCB

These are candidates, not commitments. Production activation, KYC, API availability, commercial terms, and capability scope must be verified before enabling a provider.

Target abstraction:

```go
type PPOBProvider interface {
    GetProducts(ctx context.Context, req ProductRequest) ([]Product, error)
    Inquiry(ctx context.Context, req InquiryRequest) (InquiryResult, error)
    Purchase(ctx context.Context, req PurchaseRequest) (PurchaseResult, error)
    GetStatus(ctx context.Context, ref string) (PurchaseStatus, error)
    HandleWebhook(ctx context.Context, payload []byte) error
}
```

## 4. Provider Balance Cache

DesKaProvider will periodically synchronize provider balances into its own database.

Target behavior:

- configurable polling interval, initially 30–60 seconds
- per-provider balance record
- `last_checked_at`
- provider health/status
- error count / sync failure information
- optional immediate refresh after important provider transactions

DesKaCash internal API reads provider balance state from the DesKaProvider database instead of calling each external provider directly for every balance query.

The cached balance is an operational snapshot, not an authoritative replacement for the provider's transaction result.

## 5. Provider Routing

The router should select a provider using operational data such as:

- product availability
- cached provider balance
- provider health
- provider response/error state
- provider priority
- price/cost
- latency and success history where available

Example:

```text
Transaction
    |
    v
Provider Router
    |
    +-- IAK          balance sufficient / healthy
    +-- PortalPulsa  insufficient balance
    +-- Digiflazz    balance sufficient / healthy
    |
    v
Selected Provider
```

Provider failover may be used when the selected provider is unavailable or cannot process the transaction, subject to transaction safety and idempotency rules.

## 6. Provider Liquidity Monitoring

Admin-only APIs and dashboard data should expose:

- total provider liquidity
- balance per provider
- last synchronization time
- provider health
- customer balance aggregate from DesKaCash (read through an internal/admin boundary)
- expected provider outflow
- safety buffer
- liquidity gap / warning state

Important: total customer balance is not automatically equal to required provider liquidity. Required liquidity must consider expected provider outflow, reservations, transaction volume, and safety buffer.

Example:

```text
Customer balances       Rp8,000,000
Provider liquidity       Rp7,000,000
Expected provider outflow Rp6,500,000
Safety buffer             Rp1,000,000
Required liquidity        Rp7,500,000
Liquidity gap                Rp500,000
```

Provider funding/top-up remains an explicit administrative operation. The system should warn or recommend funding; it must not silently move money without an explicitly designed authorization flow.

## 7. Admin API Direction

Planned internal/admin endpoints include:

- `GET /admin/providers/balances`
- `GET /admin/liquidity/summary`

These endpoints are admin-only and must not be exposed to ordinary DesKaCash users.

## 8. Security Baseline

Provider credentials must never be committed to Git.

Expected controls:

- environment/secret-manager based credentials
- HTTPS
- static public IP where a provider requires IP allowlisting
- restricted firewall
- SSH key access
- webhook authentication/signature validation where supported
- webhook idempotency
- audit logging
- rate limiting
- database access restricted to trusted network boundaries

## 9. Planned Development Sequence

1. Create `dev/deskaprovider-v0.1` from the documented baseline.
2. Establish Go service/module structure.
3. Define provider-neutral domain contracts.
4. Implement provider registry.
5. Implement provider credential/config abstraction.
6. Implement provider balance synchronization worker.
7. Implement provider balance persistence.
8. Implement provider health state.
9. Implement provider router.
10. Implement mock provider for deterministic tests.
11. Implement first real PPOB adapter.
12. Implement callback/webhook normalization.
13. Implement idempotency and transaction correlation.
14. Implement admin balance/liquidity API.
15. Add observability and reconciliation.
16. Integrate with DesKaCash through an internal API boundary.

## 10. Non-Goals for the Initial Baseline

The documentation does not authorize:

- treating any provider as a consumer wallet
- treating provider deposit balances as DesKaCash customer balances
- automatic provider funding
- exposing provider-specific API contracts to DesKaCash
- assuming sandbox access implies production approval
- assuming a provider's website product catalog is automatically available through its API

## 11. Status

**PLANNED**

No production implementation has been started from this baseline. Implementation should begin on:

```text
dev/deskaprovider-v0.1
```


## 12. Milestone Update — Provider Foundation + DigiFlazz Adapter

**Date:** 2026-09-24

Completed:

- repository secret hygiene baseline:
  - root `.gitignore`
  - `DesKaProvider/backend/.env.example`
- Go CI workflow for `DesKaProvider/backend`
- provider-neutral PPOB contracts
- environment-backed DigiFlazz configuration
- DigiFlazz Buyer topup request mapping
- MD5 signature generation using `md5(username + apiKey + ref_id)`
- provider-neutral mapping for `Sukses`, `Pending`, and `Gagal`
- prepaid status lookup using the same transaction endpoint and original `ref_id`
- DigiFlazz webhook payload normalization
- HMAC-SHA1 webhook signature validation when a webhook secret is configured
- deterministic unit tests using `httptest`

### DigiFlazz contract validation

The official DigiFlazz documentation confirms:

- Buyer topup endpoint: `https://api.digiflazz.com/v1/transaction`
- required request fields include `username`, `buyer_sku_code`, `customer_no`, `ref_id`, and `sign`
- signature formula: `md5(username + apiKey + ref_id)`
- transaction responses expose `status`, `rc`, `message`, `sn`, `buyer_last_saldo`, and `price`
- prepaid pending status is checked by repeating the topup request with the same `ref_id`
- webhook requests can be authenticated with `X-Hub-Signature` using HMAC-SHA1

These facts were verified against the official DigiFlazz documentation before implementation.

### CS test case validation

The supplied test tuple:

`buyer_sku_code = xld10`
`customer_no = 087800001232`

matches DigiFlazz's official prepaid test case for **Gagal**, with expected `rc = 02`. It is therefore not a success test case; the adapter test intentionally verifies the failure mapping.

### Live API validation

Live validation was **not executed in this environment**.

Reason:

- no real DigiFlazz username/API key is committed or embedded in source;
- the supplied conversation credentials are redacted;
- the repository now requires `DIGIFLAZZ_USERNAME` and `DIGIFLAZZ_API_KEY` through environment configuration.

A live test must be executed only with credentials supplied through the runtime environment and must use the documented test tuple above.

### Architectural note

The initial baseline interface used `GetStatus(ctx, ref string)` and `HandleWebhook(ctx, payload []byte)`. DigiFlazz's documented prepaid status flow requires the original product code and customer number, while webhook authentication requires request headers/secret context. The implementation therefore uses provider-neutral `StatusRequest` and `WebhookRequest` structures rather than leaking DigiFlazz-specific fields into DesKaCash.

### Next milestone

1. add provider registry;
2. add deterministic mock provider;
3. add DigiFlazz integration test harness driven only by environment variables;
4. validate the CS test case against the real DigiFlazz API;
5. then continue toward balance synchronization and provider health.


### Verification after implementation fixes

- Local command executed: `go test ./...`
- Local result: **PASS**
- The local runtime available for verification was Go 1.23.2; the repository module declares Go 1.25.1, so the exact CI toolchain still needs to provide the declared version.
- No DigiFlazz live request was executed from this environment.
- Latest repository CI status exposed through the GitHub integration currently has no reported status/run for the latest commit; it is therefore recorded as **not yet reported**, not as green.


### 13. Milestone Update — Provider Registry

**Date:** 2026-09-24

Completed:

- added a provider-neutral `Registry`
- normalized provider names case-insensitively and with surrounding whitespace trimmed
- rejected duplicate provider registration
- rejected empty provider names and nil implementations
- exposed provider lookup through `Get`
- exposed registered names through `Names`
- added deterministic registry unit tests

The registry stores `PPOBProvider` implementations only. It does not know DigiFlazz-specific types or protocols, preserving the provider boundary.

### Verification

The registry tests are designed for `go test ./...`. CI status for the latest branch commit is not yet reported by the available GitHub status integration.

### Next milestone

1. add deterministic mock provider;
2. add DigiFlazz environment-driven integration test harness;
3. validate the CS test tuple against the real DigiFlazz API;
4. then implement provider balance synchronization and provider health.


### 14. Milestone Update — Deterministic Mock Provider + CI

**Date:** 2026-09-24

Completed:

- added Provider/Mock as a provider-neutral deterministic test implementation;
- supports the full PPOBProvider contract;
- deterministic configurable purchase status, provider code, message, and price;
- records purchase results so status lookup can be tested without an external API;
- deterministic unknown-product and webhook parsing tests;
- repaired the registry test imports so the package compiles under CI;
- standardized the GitHub Actions workflow at .github/workflows/deskaprovider.yml;
- CI now runs on pushes and pull requests targeting main and dev/** when DesKaProvider/** or the workflow changes;
- CI uses the module-declared Go 1.25.1 and runs go mod tidy, go test ./..., and go vet ./....

### Verification

The repository workflow is now configured to surface test/vet failures through GitHub Actions. The next check is the CI result for this branch/PR.

### Next milestone

1. add the DigiFlazz environment-driven integration test harness;
2. run the real CS test tuple only when runtime credentials are supplied through environment variables;
3. then continue toward provider balance synchronization and provider health.


### 15. Milestone Update — DigiFlazz Integration Test Harness

**Date:** 2026-09-24

Completed:

- added an environment-driven live integration test for the documented CS failure tuple `xld10` + `087800001232`;
- the test requires `DIGIFLAZZ_USERNAME`, `DIGIFLAZZ_API_KEY`, and `DIGIFLAZZ_INTEGRATION=1` at runtime;
- no DigiFlazz credentials are stored in source control;
- the integration assertion expects normalized `failed` status and provider code `02`;
- CI keeps the normal unit-test job credential-free and adds a separate optional integration job;
- the integration job can run when repository secrets are configured or through manual `workflow_dispatch`.

### Verification

The live integration request has not been executed from this environment because no runtime DigiFlazz credentials are available. The harness is therefore intentionally environment-gated.

### Next milestone

1. execute the live CS test when runtime secrets are available;
2. then implement provider balance synchronization and provider health state.


### 16. Milestone Update — CI Restoration + Operational Balance/Health Foundation

**Date:** 2026-09-24

Completed:

- fixed the GitHub Actions integration job so the workflow does not gate a job directly on the `secrets` context;
- the live DigiFlazz test now skips cleanly when runtime credentials are unavailable, keeping the CI workflow visible and deterministic without requiring secrets;
- added optional provider-neutral `BalanceProvider` capability;
- added an operational snapshot model for provider balance and health state;
- added thread-safe in-memory operational storage as the deterministic v0.1 persistence boundary;
- added balance synchronization service with configurable failure threshold;
- successful sync records balance, `last_checked_at`, `last_success_at`, and healthy state;
- failed sync preserves the last known balance and records the error, with degraded/unhealthy state based on consecutive failure count;
- added deterministic unit tests for success, health escalation, and unsupported balance capability.

### Verification

The new operational package is designed for `go test ./...` and `go vet ./...`. A live provider balance endpoint is intentionally not implemented yet because the current source API catalog does not establish a concrete balance endpoint/response contract for DigiFlazz or the other providers.

### Next milestone

1. define the concrete provider balance adapter contract from verified provider API documentation;
2. implement the first real provider balance adapter;
3. replace/integrate the in-memory operational store with the documented persistence layer;
4. add the periodic 30–60 second worker;
5. then continue toward provider routing using cached balance and health.


### 17. Milestone Update — Verified DigiFlazz Balance Adapter + Sync Worker

**Date:** 2026-09-24

Completed:

- verified the official DigiFlazz Buyer "Cek Deposit" API documentation;
- verified balance endpoint `POST https://api.digiflazz.com/v1/cek-saldo`;
- verified request fields `cmd=deposit`, `username`, and signature `md5(username + apiKey + "depo")`;
- verified response field `data.deposit`;
- implemented `DigiFlazz.Client.GetBalance` using that contract;
- added configurable `DIGIFLAZZ_BALANCE_ENDPOINT` with the official endpoint as default;
- added deterministic `httptest` coverage for request path, payload, signature, and parsed deposit balance;
- added the periodic operational sync worker with immediate first synchronization and configurable interval;
- documented the new balance endpoint configuration in `.env.example`.

### Verification

The DigiFlazz balance contract is supported by the official documentation. https://developer.digiflazz.com/api/buyer/cek-saldo/

No live balance request was executed because runtime DigiFlazz credentials are not available. The adapter is covered by deterministic HTTP tests.

### Next milestone

1. define the production persistence implementation for operational snapshots;
2. wire the periodic worker into the service lifecycle;
3. add persistence tests and recovery behavior;
4. then continue toward provider routing using cached balance and health.


### 18. Milestone Update — CI Failure Fix: Sync Worker Compile Error

**Date:** 2026-09-24

CI run DesKaProvider CI #33 failed in the test job during go test ./....

Root cause:

- SyncAll returns a single map[string]error;
- Run incorrectly assigned two return values from s.SyncAll(ctx);
- this caused a compile error at Provider/operational/operational.go:112.

Fix applied:

- changed the immediate synchronization call to _ = s.SyncAll(ctx);
- changed periodic synchronization to explicitly discard the returned error map with _ = s.SyncAll(ctx);
- retained the intended behavior that individual provider sync failures do not terminate the worker.

The separate integration job in the same CI run completed successfully.

### Verification

The fix is committed on dev/deskaprovider-v0.1 at commit 17ac0f36f5083a165412198364dc88a1b7deb53c. A new CI run is expected for this commit and must be verified before proceeding. The previous run was red because of the compile error above.

### Next milestone

1. verify CI for the fix;
2. continue with production persistence only after the test suite is green;
3. then wire the sync worker into the service lifecycle and add recovery tests.


### 19. Milestone Update — Durable Operational Snapshot Persistence

**Date:** 2026-09-24

Completed:

- verified CI run #37 for the previous fix: **success**;
- changed the operational Store contract so persistence failures are returned instead of being silently ignored;
- added durable JSON file storage for operational snapshots;
- implemented atomic temp-file replacement and restrictive 0600 file permissions;
- added automatic parent-directory creation;
- added recovery on service/process restart by loading persisted snapshots;
- added deterministic tests for persistence/recovery and corrupt-state rejection;
- retained the in-memory store for deterministic unit tests;
- documented that JSON persistence is an interim v0.1 runtime boundary, while PostgreSQL remains the documented deployment target.

### Verification

The previous CI fix is verified green in GitHub Actions run #37. The new persistence changes have not yet received a CI result and must be verified before the next implementation step.

### Next milestone

1. verify CI for the durable persistence changes;
2. wire the operational sync worker into the service lifecycle;
3. add lifecycle/shutdown and restart behavior tests;
4. then continue toward provider routing using persisted balance and health.


### 20. Milestone Update — CI Failure: Malformed JSON Store Test Import

**Date:** 2026-09-24

CI run DesKaProvider CI #44 failed in the test job during `go test ./...`.

Root cause:

- `DesKaProvider/backend/Provider/operational/json_store_test.go` contained literal escaped newline/tab sequences in the import block;
- Go reported `illegal character U+005C '\\'` at line 4 before the operational package tests could compile;
- the integration job still completed successfully.

Fix applied:

- repaired the import block so `os`, `path/filepath`, `testing`, and `time` are valid Go imports;
- no production persistence behavior was changed.

### Verification

CI run #44 is confirmed red for the malformed test source. The fix is committed on `dev/deskaprovider-v0.1` at commit `4a5e97bdeb542f24232501f6c3da730773457707`. A new CI run must be verified before proceeding.

### Next milestone

1. verify CI for commit `4a5e97bdeb542f24232501f6c3da730773457707`;
2. if green, wire the operational sync worker into the service lifecycle;
3. add lifecycle/shutdown and restart behavior tests.


### 21. Milestone Update — CI Follow-up: Previous Import Fix Was Incomplete

**Date:** 2026-09-24

CI run DesKaProvider CI #47 remained red. The test job still reported the same parser error at `Provider/operational/json_store_test.go:4:6`: `illegal character U+005C '\\'`.

Follow-up root cause:

- the previous edit did not remove the literal escaped newline/tab sequence from the import block; the source still contained `"os"\\n\\t"path/filepath"` as literal characters.

Fix applied:

- rewrote the complete import block and test source with real Go newlines/tabs;
- production code was not changed.

### Verification

CI run #47 is confirmed red for the still-malformed test source. The corrected source is committed on `dev/deskaprovider-v0.1` at commit `b47a39acd8feb11af44901c18ef94a2c6e3f8256`. A new CI run must be verified before proceeding.



### 22. Milestone Update — Service Lifecycle Wiring + Graceful Shutdown

**Date:** 2026-09-24

Completed:

- verified DesKaProvider CI run #49 for the corrected JSON store test source: **success**;
- added a runtime assembly package at `backend/runtime`;
- added environment-driven operational runtime configuration:
  - `DESKAPROVIDER_OPERATIONAL_STORE_PATH`
  - `DESKAPROVIDER_OPERATIONAL_CURRENCY`
  - `DESKAPROVIDER_BALANCE_SYNC_INTERVAL`
  - `DESKAPROVIDER_BALANCE_FAILURE_THRESHOLD`
- defaulted the operational sync interval to 30 seconds and failure threshold to 3;
- wired the DigiFlazz provider, durable JSON operational store, and balance sync service into the runtime assembly;
- replaced the empty service `main.go` with a signal-aware lifecycle using SIGINT/SIGTERM cancellation;
- added lifecycle tests covering configuration validation, immediate balance synchronization, graceful context cancellation, and environment-based durable service construction;
- documented the runtime configuration in `backend/.env.example`.

### Verification

CI run #49 is green for the corrected test source. The lifecycle changes were then committed to `dev/deskaprovider-v0.1`; the new lifecycle commit still requires its own CI result before the next implementation step.

The runtime intentionally does not introduce an HTTP server or admin API yet. It only wires the already-documented provider balance synchronization lifecycle.

### Next milestone

1. verify CI for the lifecycle wiring;
2. add an explicit restart/recovery lifecycle test using the durable JSON store;
3. then continue toward provider routing using persisted balance and health.



### 23. Milestone Update — CI #59 Root Cause Fixed

**Date:** 2026-09-24

CI run #59 failed in the `Tidy` step before tests executed.

Root cause from the GitHub Actions log:

- `backend/cmd/deskaprovider/main.go` imported `github.com/DesKaOne/DesKaEcosystem/DesKaProvider/backend/runtime`;
- the Go module root is `DesKaProvider/backend`, with module path `github.com/DesKaOne/DesKaEcosystem/DesKaProvider`;
- therefore the runtime package import path must be `github.com/DesKaOne/DesKaEcosystem/DesKaProvider/runtime`.

The import path was corrected in commit `91f688e7bba21480b9fd81f5a21a22029973470b`.

The previous CI failure was an import-path/module-boundary issue, not a runtime test failure. A new CI run is required to verify the correction.

### 24. Milestone Update — Explicit Restart/Recovery Lifecycle Test

**Date:** 2026-09-24

Completed:

- verified DesKaProvider CI run #64 for the previous runtime import-path fix: **success**;
- added an explicit restart/recovery lifecycle test using the durable JSON operational store;
- the test creates a first service instance, performs an immediate provider balance synchronization, and persists the healthy snapshot;
- a second, distinct service/store instance reloads the same snapshot after the simulated restart boundary;
- the recovered balance and health are asserted before the second synchronization;
- the second synchronization updates the persisted balance and confirms healthy state with zero consecutive failures;
- the test remains provider-neutral by using the existing deterministic mock balance provider.

### Verification

The restart/recovery test is committed on `dev/deskaprovider-v0.1` at commit `a82757cf227286422a38f6af41e6a6a9bdf96c80`.

A new CI run has not yet been reported for this commit at the time of this update. It must be verified before proceeding.

### Next milestone

1. verify CI for the restart/recovery test;
2. design the provider routing contract around persisted balance and health;
3. implement deterministic routing tests before adding provider-specific routing behavior.

### 25. Milestone Update — Deterministic Provider Routing Foundation

**Date:** 2026-09-24

Completed:

- verified DesKaProvider CI run #68 for the restart/recovery milestone: **success**;
- added a provider-neutral routing package at `backend/routing`;
- routing requires a persisted operational snapshot with healthy provider state and sufficient cached balance;
- routing verifies that the selected provider exposes the requested product through the existing provider-neutral `GetProducts` contract;
- provider priority is configurable through the router constructor;
- equal-priority candidates use provider name as a deterministic tie-break;
- unavailable, unhealthy, insufficient-balance, and missing-product candidates are excluded;
- invalid route requests and the no-provider condition have explicit errors;
- added deterministic tests for priority selection, operational exclusions, product availability, tie-breaking, and request validation.

### Routing boundary

The router consumes:

- provider-neutral `Registry`;
- operational `Store`;
- cached balance and health snapshots;
- provider-neutral product availability.

It does **not** implement provider-specific API logic, pricing logic, or automatic provider funding.

### Verification

Routing implementation and tests are committed on `dev/deskaprovider-v0.1`. A new CI run is required for the routing changes before the next implementation step.

### Next milestone

1. verify CI for the routing foundation;
2. integrate routing into a provider selection service without changing DesKaCash ledger semantics;
3. add transaction safety/idempotency boundaries around provider selection and purchase execution.


### 26. Milestone Update — Routed Provider Purchase Execution Boundary

**Date:** 2026-09-24

Completed:

- added `routing.Service` as the provider-neutral purchase execution boundary above the deterministic router;
- purchase requests now carry product code, customer number, reference ID, amount, and testing flag;
- the service validates the required request fields before provider selection;
- provider selection still uses the existing cached balance, health, product availability, and priority rules;
- the selected provider is resolved from the neutral registry and receives the neutral `PurchaseRequest`;
- provider purchase results are returned together with the selected provider name;
- provider-specific errors are wrapped without translating them into DesKaCash ledger operations;
- intentionally does **not** perform automatic fallback/retry after a provider purchase has been submitted, avoiding duplicate external transactions until an explicit idempotency/correlation boundary exists;
- added deterministic tests covering routed purchase execution, provider failure propagation without fallback, and request validation.

### Transaction safety boundary

This milestone deliberately stops at one selected provider purchase call:

`Select -> Purchase -> return provider result`

There is no automatic retry, cross-provider failover, customer ledger mutation, or provider funding in this service. The next idempotency layer must establish how a single reference ID is correlated with provider attempts and how repeated requests are resolved before any retry/failover behavior is introduced.

### Verification

The implementation and deterministic tests are committed on `dev/deskaprovider-v0.1`. CI verification for the new routing service changes is still required before the next implementation step.

### Next milestone

1. verify CI for the routed purchase execution boundary;
2. design and implement provider transaction correlation/idempotency state around the existing reference ID;
3. ensure repeated requests cannot create unintended duplicate provider purchases before considering controlled failover.


## 27. Provider Verification and Implementation Sequence

**Date:** 2026-09-24

The v0.1 provider roadmap is now explicitly separated into implementation progress and provider verification status.

### Provider implementation status

| Provider | v0.1 target | Current direction |
|---|---:|---|
| DigiFlazz | 50% or 100% completion is acceptable before moving on | **Current work** |
| IAK | 100% | Next after DigiFlazz checkpoint |
| XP SINDONESIA | 100% | After IAK |
| Midtrans | 100% | After XP SINDONESIA |
| RCB | 100% | After Midtrans |
| PortalPulsa | Skip | Not planned for the current sequence |

The DigiFlazz milestone does not need to reach 100% before the next provider begins. Once the DigiFlazz implementation reaches a stable **50% checkpoint**, or is completed to 100%, development may continue to IAK.

### Agreed implementation order

```text
DigiFlazz
    ↓
IAK
    ↓
XP SINDONESIA
    ↓
Midtrans
    ↓
RCB
```

PortalPulsa is explicitly skipped for this v0.1 provider sequence.

### Provider verification status

Current verified/active providers:

- IAK — verified/active
- Midtrans — verified/active
- XP SINDONESIA — verified/active

Verification pending:

- RCB — verification pending

Deferred verification until the application is running and can satisfy the provider's business/application evidence requirements:

- DOKU
- Ezeelink
- Digiflazz

This verification status must not be confused with adapter implementation status. A provider can have development work underway while production verification remains pending.

### Scope note

The immediate engineering focus remains DigiFlazz. After the agreed 50% checkpoint or 100% completion, development moves to IAK, then XP SINDONESIA, Midtrans, and RCB. Provider-specific credentials must remain outside source control.


### 28. Milestone Update — CI Failure Fix: Routing Mock Test Import

**Date:** 2026-09-24

CI run #87 failed in the `Test` step while compiling `DesKaProvider/routing`.

Root cause:

- `routing/service_test.go` imported the Mock provider package without an alias;
- the package declaration exposes the identifier `mock`, while the tests referenced `Mock`, producing an unused-import and undefined-identifier build failure.

Fix applied:

- explicitly aliased the import as `Mock` so the existing test references resolve correctly;
- no production routing behavior was changed.

### Verification

The failure is confirmed from GitHub Actions run #87. The correction is committed on `dev/deskaprovider-v0.1` at commit `0b66672a90933a96ac73d42c9dc75ece67ac53fd`.

A new CI run is required before proceeding to the transaction correlation/idempotency milestone.

### Next milestone

1. verify CI for the routing test fix;
2. only after CI is healthy, implement provider transaction correlation/idempotency state around the neutral reference ID.


### 29. Milestone Update — CI Failure Follow-up: Mock Import Alias Still Missing

**Date:** 2026-09-24

CI run #94 remained red in the `Test` step.

Root cause confirmed from the GitHub Actions log:

- `routing/service_test.go` still imported the Mock provider package without an explicit alias;
- the package declaration is `mock`, while the tests reference `Mock.Provider`;
- Go therefore reported the import as unused and `Mock` as undefined.

Fix applied:

- explicitly aliased the import as `Mock`;
- production routing behavior remains unchanged.

### Verification

The failure is confirmed in CI run #94. The source fix is committed on `dev/deskaprovider-v0.1` at commit `cf04d96f705c6c00c2a1a55db0691db7e81e7d0e`. A fresh CI run must be verified before starting transaction correlation/idempotency work.

### Next milestone

1. verify CI after the import-alias fix;
2. only after CI is healthy, implement provider transaction correlation/idempotency state around the neutral reference ID.


### 30. Milestone Update — CI Failure Follow-up: Mock Construction Boundary

**Date:** 2026-09-24

CI run #98 passed the import-alias issue but failed compiling `routing/service_test.go` because the test attempted to initialize unexported fields of `mock.Provider` from another package.

Root cause:

- `Provider/Mock` exposes configuration through `mock.Config` and constructor `mock.New`;
- the routing test used a direct struct literal with fields that are intentionally private inside the Mock package.

Fix applied:

- changed routing tests to construct mocks through `Mock.New(Mock.Config{...})`;
- no production routing behavior changed;
- the provider encapsulation boundary is preserved.

Commit: `3e633a5d40bee515a58d52e92f1d9146d042ef54`.

### Verification gate

CI run #98 is confirmed red. A fresh CI result for the constructor fix is mandatory before transaction correlation/idempotency work begins.


### 31. Milestone Update — CI Failure Follow-up: No-Fallback Test Assertion

**Date:** 2026-09-24

CI run #100 reached the routing tests successfully but failed in `TestServicePurchaseDoesNotFallbackAfterProviderError`.

Root cause:

- the test expected the second provider to return a pending transaction when no fallback occurred;
- the deterministic Mock provider correctly returns `ErrTransactionNotFound` because it never received a purchase.

Fix applied:

- changed the assertion to require `Mock.ErrTransactionNotFound` from the second provider;
- this directly verifies that no fallback purchase was submitted.

Commit: `049b403658023b63f09302c6722fbaa39b7d06a0`.

### Verification gate

CI #100 is confirmed red. The new commit must reach a green CI result before transaction correlation/idempotency implementation begins.
