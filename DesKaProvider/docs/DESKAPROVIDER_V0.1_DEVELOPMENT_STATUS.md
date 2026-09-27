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


### 32. Milestone Update — Purchase Reference Correlation / Idempotency

**Date:** 2026-09-24

CI #104 is confirmed green before this milestone.

Implemented a provider-neutral in-process purchase correlation boundary in `routing.Service` using the existing `ReferenceID`:

- first request for a reference executes normally;
- repeated identical request returns the original execution result without submitting another provider purchase;
- a different request reusing an existing `ReferenceID` is rejected with `ErrReferenceConflict`;
- concurrent duplicate requests share the same in-flight completion;
- no automatic retry or provider failover is introduced after submission;
- DesKaCash ledger/customer balance remains outside this layer.

Deterministic tests cover successful repeated requests and reference conflicts. The implementation is intentionally an in-process v0.1 boundary; durable PostgreSQL-backed transaction state remains a later deployment-direction concern.

Commits: `cef6f075a21acbaa59dd69904be16d00c125be47`, `357012076274707c760d4e59496e7121da9bce33`, `41c36336e9d090ac8ad78294c77256af3bf2b43c`.

### Verification gate

A fresh CI result for the idempotency implementation is required before moving to the next reliability milestone.


### 33. Milestone Update — Concurrent Idempotency Verification

**Date:** 2026-09-24

CI #112 is confirmed green before this follow-up.

Strengthened the in-process purchase idempotency boundary with deterministic concurrency coverage:

- the Mock provider now records purchase submission counts for test verification;
- 16 concurrent identical requests using the same `ReferenceID` are exercised together;
- the test requires all callers to receive the same execution result;
- the test requires exactly one provider purchase submission;
- the shared transaction state remains protected by the service mutex and completion channel.

This milestone validates the duplicate-submission safety property before any controlled retry/failover design is considered.

Commits: `a0e65e33d30b6f0435e8a7652ddf2b1265b6df5b`, `cda41646425f09a6192e856f832502f4222bb17f`.

### Verification gate

A fresh CI result for the concurrent idempotency test and documentation update is required before the next reliability milestone.

### 34. Milestone Update — CI Race-Detector Hardening

**Date:** 2026-09-24

CI run #122 is confirmed green for the concurrent idempotency documentation commit before this milestone.

Added a dedicated GitHub Actions race-detector job for the DesKaProvider backend:

- uses the module-declared Go 1.25.1 toolchain;
- runs `go test -race ./...`;
- executes alongside the existing unit-test/vet job;
- specifically hardens verification of the mutex/channel-based concurrent purchase idempotency boundary;
- does not introduce retry, failover, ledger mutation, or new provider behavior.

The race job is a verification-only CI hardening step. Provider-specific integration remains separately credential-gated.

### Verification gate

The workflow change and this milestone documentation require a fresh GitHub Actions run. The branch must remain blocked from the next reliability implementation until the complete CI run, including the race-detector job, is green.

### Next milestone

1. verify the complete CI run including `go test -race ./...`;
2. then continue with provider transaction callback/webhook correlation at the neutral routing boundary, without moving ledger semantics into DesKaProvider.

### 35. Milestone Update — Provider Webhook Correlation Boundary

**Date:** 2026-09-24

CI #126 is confirmed green, including the dedicated `go test -race ./...` job, before this milestone.

Implemented a provider-neutral webhook correlation boundary in `routing.Service`:

- webhook events are correlated by the existing `ReferenceID`;
- product code and customer number must match the original purchase request;
- unknown references are rejected instead of creating transaction state;
- pending purchases can transition to success or failed through a normalized webhook event;
- repeated identical terminal webhook events are idempotent;
- conflicting terminal events are rejected;
- invalid webhook status/identity data is rejected;
- provider-specific signature/authentication remains inside the provider adapter;
- no DesKaCash ledger mutation, retry, failover, or provider funding is introduced;
- the implementation remains in-process v0.1 state, so webhook correlation after process restart is not yet durable.

Deterministic tests cover pending-to-success correlation, duplicate webhook delivery, unknown references, and identity conflicts.

### Verification gate

The webhook correlation implementation and documentation require a fresh complete CI run. The test, vet, and race-detector jobs must all be green before the next reliability milestone.

### Next milestone

1. verify the complete CI run for webhook correlation;
2. then review transaction status reconciliation semantics and durable transaction-state requirements before introducing any retry/failover behavior.

### 36. Milestone Update — Provider Status Reconciliation Boundary

**Date:** 2026-09-24

CI #132 is confirmed green, including the race-detector job, before this milestone.

Implemented an explicit provider-status reconciliation operation in `routing.Service`:

- reconciliation uses the existing transaction `ReferenceID`;
- the selected provider is queried through the neutral `GetStatus` contract;
- returned reference ID, product code, and customer number must match the original request;
- reconciliation updates only the in-process provider transaction result;
- terminal states remain idempotent when the provider returns the same terminal result;
- conflicting terminal state is rejected rather than silently overwritten;
- reconciliation never resubmits the purchase;
- unknown references are rejected;
- provider-specific status semantics remain inside adapters;
- no DesKaCash ledger mutation or automatic retry/failover is introduced.

The current implementation remains intentionally in-memory. A process restart loses the transaction correlation state, so durable transaction persistence is still required before production-grade recovery or automatic retry/failover can be designed safely.

### Verification gate

The reconciliation implementation and documentation require a fresh complete CI run. The unit test, vet, and race-detector jobs must all be green before the next milestone.

### Next milestone

1. verify the complete CI run for status reconciliation;
2. define the minimum durable transaction-state model needed for restart-safe correlation/reconciliation;
3. defer automatic retry/failover until that durable state and provider-specific idempotency semantics are explicitly established.

### 37. Milestone Update — Durable Transaction-State Correlation Boundary

**Date:** 2026-09-24

Completed:

- added a provider-neutral `TransactionStore` boundary for routed purchase correlation;
- added an in-memory transaction store for deterministic service behavior;
- added a durable JSON transaction store with:
  - automatic parent-directory creation;
  - atomic temp-file replacement;
  - 0600 transaction-state file permissions;
  - deterministic sorted reads;
  - corrupt JSON rejection;
- extended `routing.Service` with `NewServiceWithStore`;
- retained `NewService` as the in-memory convenience constructor;
- restored persisted transaction states into completed in-memory correlation calls during service initialization;
- persisted provider transaction results after a successful provider submission;
- persisted webhook status transitions before updating in-memory state;
- persisted reconciliation status transitions before updating in-memory state;
- preserved the existing no-resubmission rule: restart recovery returns the persisted transaction state and does not submit the same provider purchase again;
- added restart/recovery coverage using a fresh service, fresh registry, fresh provider instance, and the same durable transaction store;
- replaced the previous reconciliation identity test with a real provider-status identity mismatch test;
- kept retry/failover out of scope because durable state alone does not establish provider-specific idempotency semantics for ambiguous submission failures;

### Verification

- CI run #137 for commit `57079560eeb4fe8291c74c9c53d3013e52d17a46` is **success** and was verified before this milestone.
- The durable transaction-state implementation commits are:
  - `e1a1e879ae788f63f7669f85100c09ccd66899ba`
  - `cfca16a84cb965094d68dfec745f45880784de96`
  - `5ff980f9de690894ec3ec3ea7695faab7f2ecebf`
  - `15fa727fac4b04b75cea715e31ab17e79e559ad7`
  - `8da3eecce2e511aa47dd2d68d576cc3e605bd466`
- CI #149 exposed a test-build regression in `service_test.go` (missing `filepath` import and reconciliation test helper). Fixed in commit `0cec25cc6791f0fdf1093c984ec56d3640b1511b`. The milestone remains pending until the subsequent CI run is green.

### Safety Boundary

The durable transaction store records provider transaction correlation and normalized result state only. It does not:

- mutate the DesKaCash ledger;
- automatically retry an ambiguous provider submission;
- automatically fail over to another provider after submission;
- fund providers;
- replace provider-specific idempotency/correlation guarantees.

### Next milestone

1. verify CI for this durable transaction-state milestone;
2. if green, continue with the next reliability boundary without introducing automatic retry/failover prematurely;
3. evaluate the minimum runtime wiring needed to use the durable transaction store in the service composition.

### CI Fix Follow-up

- CI #153 failed because the reconciliation test helper was accidentally placed before the Go `package` declaration.
- Fixed in `a5e37a6996bb5b1acdfe16d5145051b0a4216059`.
- CI must be re-run and verified green before continuing.

- CI #157 failed due to a duplicate `path/filepath` import in `routing/service_test.go`; corrected in `136d8c95538732fc2fced5b45152bd48da9bd6c2`.

### CI Fix Follow-up — Restart Recovery Test

**Date:** 2026-09-24

CI #160 failed in both the unit-test and race-detector jobs at `TestServiceRestartRecoversDurableTransactionState`.

Root cause:

- the restart test intentionally creates a fresh Mock provider instance to verify that the durable transaction store prevents a duplicate purchase submission;
- the fresh Mock instance has no provider-side transaction record for the recovered reference ID;
- the test then called `Reconcile`, which correctly queried the fresh provider and returned `Mock.ErrTransactionNotFound`;
- this was a test-fixture mismatch, not a routing production failure.

Fix:

- keep the restart test focused on the durable correlation/idempotency guarantee: the recovered service returns the persisted transaction state and does not resubmit the purchase;
- remove the reconciliation call from that fresh-provider restart fixture;
- retain separate reconciliation coverage for provider status queries on a provider instance that owns the transaction state.

The production reconciliation boundary is unchanged.

### Verification gate

CI #160 is red. The fix must reach a complete green CI result, including `test` and `race`, before continuing to the next milestone.

### 38. Milestone Update — Runtime Durable Transaction-Service Wiring

**Date:** 2026-09-24

CI #164 is confirmed green before this milestone:

- `test` — success (`go test ./...` and `go vet ./...`);
- `race` — success (`go test -race ./...`).

Completed the minimum runtime composition needed to make the durable transaction-state boundary usable by the actual runtime:

- added `TransactionStorePath` to runtime configuration;
- added default durable transaction state path `data/provider-transactions.json`;
- added environment override `DESKAPROVIDER_TRANSACTION_STORE_PATH`;
- `NewFromEnvironment` now opens the durable transaction store;
- runtime now composes the existing provider registry + operational store + neutral router + `routing.Service` with the durable transaction store;
- exposed the composed neutral purchase service through `PurchaseService()`;
- kept provider-specific logic inside adapters and customer ledger semantics outside DesKaProvider;
- automatic retry/failover remains out of scope.

Deterministic runtime tests now verify the new configuration default and that environment-based construction produces a non-nil routed purchase service.

### Safety Boundary

Runtime wiring only composes existing boundaries. It does not introduce automatic resubmission, cross-provider failover, DesKaCash ledger mutation, or provider funding.

### Verification

- CI #164 for commit `5b00148f83aee9bfacaebd277a5e95019c53108e` is **success**.
- Both `test` and `race` jobs were independently verified green.
- Runtime wiring commits: `1c3d1d6d7b935e450ca4e0e4eac55a5e3eb53da0`, `14ed03b0e0d779c6102f21ea3f9c08a0d2d0ce7c`.


### 39. Milestone Update — Concurrent Reconciliation Atomic Convergence

**Date:** 2026-09-26

Completed:

- audited the PostgreSQL-backed service reconciliation path across independently constructed service instances;
- repaired integration-test variable scope so PostgreSQL store handles remain local to their owning test;
- preserved deterministic concurrency coverage proving two concurrent reconciliations converge on the same terminal provider result;
- verified concurrent reconciliation does not resubmit the provider purchase and the durable PostgreSQL transaction remains terminally successful;
- updated PR CI checkout to validate the actual pull-request head SHA instead of a stale synthetic merge ref;
- verified exact implementation HEAD `96210d68e3c47b90687fa4dc2d694c76b50fc6a6` with CI `test`, `race`, and `vet` all successful;
- preserved transaction persistence as authoritative state while audit and operational evidence remain non-authoritative.

### Verification

- implementation HEAD: `96210d68e3c47b90687fa4dc2d694c76b50fc6a6`;
- CI #952 / run `36210907716`: **GREEN**;
- CI jobs `test`: success;
- CI jobs `race`: success;
- CI step `vet`: success.

### Safety Boundary

- concurrent reconciliation converges on one durable terminal state and never creates a second provider purchase;
- persistence conflicts remain a state-transition concern and do not authorize application-level resubmission;
- audit or operational evidence cannot authorize retry, failover, resubmission, ledger mutation, treasury movement, or provider funding.

### Next milestone

1. verify the CI result for runtime wiring;
2. then continue with the next reliability/provider boundary without introducing automatic retry/failover prematurely.


### CI Fix Follow-up — Runtime Transaction Store Default

**Date:** 2026-09-24

CI #170 failed in both `test` and `race` for the runtime wiring commit series.

Root cause:

- `runtime.LoadConfig()` referenced `defaultTransactionStorePath`;
- the runtime implementation patch had added the new configuration field and environment handling but omitted the constant declaration;
- the failure is a compile-time wiring omission, not a transaction-store behavior failure.

Fix:

- restored the missing `defaultTransactionStorePath = "data/provider-transactions.json"` constant;
- normalized `runtime.go` formatting while preserving the existing runtime composition;
- no retry/failover, ledger mutation, or provider-specific behavior was changed.

Fix commit: `274df50f624426746324108e6ec6f8ba37a5d03b`.

### Verification Gate

CI #170 is **failure**. The branch must not advance to the next milestone until a fresh CI run verifies both `test` and `race` green.


### 39. Milestone Update — DigiFlazz Prepaid Product Catalog Boundary

**Date:** 2026-09-24

CI #174 is confirmed green before this milestone:

- `test` — success (`go test ./...` and `go vet ./...`);
- `race` — success (`go test -race ./...`).

The next provider boundary addresses a functional gap in the existing runtime routing path: the neutral router performs a provider product-capability check through `GetProducts`, while the DigiFlazz adapter previously returned `ErrUnsupportedOperation`. That meant the composed DigiFlazz purchase route could not pass the product-capability stage.

Completed:

- added DigiFlazz prepaid price-list endpoint configuration;
- added default `https://api.digiflazz.com/v1/price-list`;
- added `DIGIFLAZZ_PRICE_LIST_ENDPOINT` override support;
- implemented DigiFlazz `GetProducts` against the documented Buyer prepaid price-list operation;
- uses the documented `prepaid` command and `md5(username + apiKey + "pricelist")` signature;
- maps `buyer_sku_code` and `product_name` into the provider-neutral product contract;
- supports the existing neutral category filter;
- supports the existing neutral active filter using `buyer_product_status`;
- added deterministic HTTP test coverage for endpoint, request fields, signature, response mapping, and active filtering.

The official DigiFlazz documentation states that the Buyer price-list endpoint is `https://api.digiflazz.com/v1/price-list`, uses `cmd=prepaid), and signs with `md5(username + apiKey + "pricelist")`. It also notes that price-list checks are rate-limited and recommends storing the list locally and updating it periodically. citeturn1search0

### Safety Boundary

This milestone only closes the provider product-capability boundary. It does not:

- introduce automatic retry or failover;
- mutate the DesKaCash ledger;
- fund providers;
- move provider-specific protocol fields outside the DigiFlazz adapter.

The documented price-list rate limitation is an explicit reason not to treat this direct `GetProducts` call as the final production catalog architecture. A subsequent catalog-cache/synchronization boundary should be considered before high-volume production routing. citeturn1search0

### Verification Gate

- CI #174 for commit `13b57f81d90c0011ea608c8f8127a596c408ed02` is **success**.
- Both `test` and `race` jobs were independently verified green before starting this milestone.
- Product-catalog implementation commits:
  - `21c35a5e6c7bb49908dfd12488a87a3e0d0e1d5b`
  - `8e3719186f3093f134ca8a1b9dda5f99c27a0f07`
  - `cd1d0cebd5ff51547e53e44f51d56239e4e92e4c`

### Next Milestone

1. verify CI for the DigiFlazz product-catalog implementation;
2. if green, design the minimum catalog-cache/synchronization boundary so routing does not repeatedly hit a rate-limited provider price-list endpoint;
3. keep automatic retry/failover deferred until provider-specific idempotency semantics are explicitly established.

### 40. Milestone Update — DigiFlazz Product Catalog Cache Boundary

**Date:** 2026-09-25

CI run #180 for the previous DigiFlazz product-catalog implementation is confirmed **green** before this milestone:

- test — success;
- vet — success;
- race — success.

Implemented the minimum catalog-cache boundary needed to avoid repeatedly calling the DigiFlazz rate-limited price-list endpoint during provider routing:

- added DigiFlazz.CachedClient as a provider-adapter wrapper;
- caches successful GetProducts results in memory by neutral category + active filter;
- uses a 15-minute runtime TTL;
- returns defensive product slices so callers cannot mutate cached state;
- serializes concurrent cache misses per client, preventing duplicate price-list requests during the same refresh;
- preserves provider-specific price-list behavior inside the DigiFlazz adapter;
- leaves purchase, status, webhook, and balance operations delegated directly to the underlying DigiFlazz client;
- cache refresh failures are returned to the caller and do not silently serve expired catalog data;
- wired the cached client into runtime composition;
- documented the 15-minute cache TTL in backend/.env.example;
- added deterministic HTTP tests proving repeated product lookups within the TTL issue only one provider request and that distinct active-filter requests have separate cache entries.

The cache is intentionally in-memory and non-durable for v0.1. The current scope is to prevent repeated price-list calls inside a running process; a future durable catalog synchronization boundary can be introduced if deployment requirements require restart persistence or scheduled refresh independent of routing traffic.

### Safety Boundary

This milestone does not:

- introduce automatic retry or failover for transactions;
- mutate the DesKaCash ledger;
- fund providers;
- move DigiFlazz-specific protocol behavior outside the adapter;
- treat the cached product list as a transaction result or customer balance.

### Verification Gate

The cache implementation commits are:

- 4ed7a7feaedc96771ecd6b3bd11d9c713b47c442
- 9e4a1d08acd490ca48370c061de399d4ceba8039
- 3b408764e3d0c3284e414a0bc07164450f8ae2ac
- eaa2f8c866abd52754028f9438f3124dee978cb2
- 3dad3977fdcf5a66eaa35c3cba72c57f45c2c00e

CI run #192 for the cache implementation is confirmed **green**:

- test — success (`go test ./...` and `go vet ./...`);
- race — success (`go test -race ./...`).

The cache milestone is therefore verified and the branch is clear for the next development step.

### Next Milestone

1. verify the complete CI result for the cache boundary;
2. if green, review whether a neutral durable catalog synchronization layer is required before adding the next provider;
3. keep transaction retry/failover deferred until provider-specific idempotency semantics are explicitly established.

### 41. Milestone Update — Neutral Durable Catalog Synchronization Boundary

**Date:** 2026-09-25

The previous cache milestone was already verified green before this work:

- CI #196 — success;
- test — success;
- vet — success;
- race — success.

Implemented the minimum neutral catalog synchronization layer so routing no longer needs to call a provider price-list endpoint on every route selection:

- added provider-neutral `catalog.Snapshot` containing provider name, product list, and synchronization timestamp;
- added thread-safe in-memory catalog store;
- added durable JSON catalog store with:
  - missing/empty file initialization;
  - corrupt JSON rejection;
  - deterministic sorted reads;
  - atomic temporary-file replacement;
  - restrictive 0600 file permissions;
  - defensive product-slice copies;
- added provider-neutral `catalog.SyncService` that obtains `GetProducts` snapshots from registered providers and stores them with a synchronization timestamp;
- added `routing.NewWithCatalog`;
- routing now uses the synchronized catalog when one is supplied and no longer performs a provider `GetProducts` request during selection;
- retained the existing router constructor behavior for tests/legacy composition that do not provide a catalog;
- runtime now composes the durable catalog store and catalog synchronization service;
- runtime performs an initial catalog synchronization and repeats it on a configurable 15-minute default interval;
- added `DESKAPROVIDER_CATALOG_STORE_PATH`;
- added `DESKAPROVIDER_CATALOG_SYNC_INTERVAL`;
- default catalog path is `data/product-catalog.json`;
- documented the new settings in `backend/.env.example`;
- added deterministic tests for catalog synchronization, durable persistence/corrupt-file handling, defensive copies, and catalog-backed routing.

The catalog is now a neutral operational snapshot boundary. Provider-specific protocol details remain inside provider adapters. The catalog does not represent customer balances, transaction state, or DesKaCash ledger state.

### Safety Boundary

This milestone does not:

- introduce automatic transaction retry or failover;
- resubmit ambiguous provider transactions;
- mutate the DesKaCash ledger;
- fund providers;
- treat catalog availability as proof of transaction success;
- move provider-specific price-list protocol behavior into the routing layer.

### Verification Gate

CI run #217 is confirmed **green** for the catalog synchronization implementation:

- `test` — success (`go test ./...` and `go vet ./...`);
- `race` — success (`go test -race ./...`).

The durable catalog synchronization milestone is verified and the branch is clear for the next development step.

### Next Milestone

After CI is green, review catalog freshness/error semantics and provider-adapter readiness before proceeding to the next provider. Keep transaction retry/failover deferred until provider-specific idempotency semantics are explicitly established.


### 42. Milestone Update — Catalog Freshness / Stale-Data Routing Boundary

**Date:** 2026-09-25

CI #217 was confirmed green for the previous durable catalog synchronization milestone before this work.

Implemented a neutral freshness policy on top of the durable catalog boundary:

- added a 30-minute default maximum catalog age for routing;
- added `DESKAPROVIDER_CATALOG_MAX_AGE` runtime configuration;
- runtime now constructs the router with the configured catalog maximum age;
- catalog-backed routing rejects provider snapshots older than the configured freshness window;
- preserved the legacy direct-provider product lookup path for routers created without a catalog store;
- added deterministic tests for stale catalog rejection and invalid freshness configuration;
- documented the new environment setting in `.env.example`.

The policy deliberately treats stale catalog data as insufficient for a new route selection. It does not refresh the provider inline and does not silently reuse expired catalog data, preserving the separation between scheduled synchronization and transaction routing.

### Safety Boundary

This milestone does not:

- introduce automatic transaction retry or failover;
- resubmit ambiguous provider transactions;
- mutate the DesKaCash ledger;
- fund providers;
- treat a stale catalog as proof of product availability;
- move provider-specific catalog protocol behavior outside provider adapters.

### Verification Gate

The changes require a fresh complete CI run after the implementation commits. The `test`, `vet`, and `race` jobs must all be green before the next provider-development step.

### Next Milestone

1. verify the fresh CI result for the catalog freshness boundary;
2. if green, review DigiFlazz adapter completeness against the neutral PPOB contract;
3. determine the minimum verified provider-specific work needed before starting the next roadmap provider;
4. keep automatic transaction retry/failover deferred until provider-specific idempotency semantics are explicitly established.


### CI Fix Follow-up — Catalog Freshness Config Declaration

CI #231 failed in the `test` job after the freshness implementation because `runtime.go` referenced `CatalogMaxAge` and `defaultCatalogMaxAge` before those declarations were present in the committed runtime file. The routing and test packages themselves compiled successfully; the failure was isolated to runtime configuration wiring.

Fix applied in commit `909ebb5fe013179cea9682818982cab0ef8944af`:

- declared `defaultCatalogMaxAge = 30 * time.Minute`;
- added `CatalogMaxAge time.Duration` to runtime `Config`;
- preserved the existing environment parsing and router wiring.

CI must be re-verified green, including `test`, `vet`, and `race`, before continuing.


### 43. Milestone Update — DigiFlazz PLN Inquiry Capability

**Date:** 2026-09-25

CI run #235 for the previous catalog-freshness configuration fix is confirmed **green** before this milestone:

- `test` — success;
- `race` — success;
- the workflow completed successfully.

Reviewed the official DigiFlazz Buyer documentation before extending the adapter. The documentation exposes a dedicated PLN inquiry endpoint and defines the request signature as `md5(username + apiKey + customer_no)`. The response provides normalized status, response code, and message fields. citeturn3view0

Implemented the minimum provider-specific Inquiry capability supported by that documented contract:

- added `DIGIFLAZZ_INQUIRY_PLN_ENDPOINT` configuration with the documented endpoint as default;
- implemented `DigiFlazz.Client.Inquiry` for the explicit neutral product code `pln` only;
- sends `username`, `customer_no`, and the documented MD5 signature;
- maps the documented response status/code/message into `provider.InquiryResult`;
- leaves unsupported product inquiry operations as `provider.ErrUnsupportedOperation` rather than inventing a generic DigiFlazz inquiry protocol;
- added deterministic HTTP coverage for endpoint, payload, signature, successful mapping, and unsupported-product behavior;
- documented the endpoint override in `.env.example`.

This milestone is intentionally scoped to the verified PLN inquiry operation. It does not claim that every DigiFlazz product supports a generic inquiry operation.

### Safety Boundary

This milestone does not:

- introduce transaction retry or automatic failover;
- resubmit ambiguous transactions;
- mutate the DesKaCash ledger;
- treat inquiry success as purchase success;
- infer unsupported inquiry protocols for other product types.

The official DigiFlazz status documentation also warns against repeated status calls for the same transaction within less than one minute and states that prepaid status is obtained by repeating the topup with the same `ref_id`; therefore the existing transaction correlation and no-automatic-retry boundary remains unchanged. citeturn2view0

### Verification Gate

The implementation commits are:

- `d7c2d0c850e4acb6b511e5b658851e36fc805597` — configuration;
- `071b413f821335ba8fdb8845dbf9a66c73ba7d0f` — adapter;
- `926b00e76e9895dde1d8b31d962490fc953ee4c3` — tests;
- `4443b4fa65b85b81ddec4c1b60d4e66af8a72a09` — environment documentation.

A fresh CI run is required and must be green (`test`, `vet`, and `race`) before continuing to the next provider-readiness milestone.

### Next Milestone

1. verify the fresh CI result for the PLN inquiry capability;
2. if green, review DigiFlazz adapter readiness against the documented v0.1 roadmap checkpoint;
3. decide whether DigiFlazz has enough verified coverage to move to IAK, without claiming live production verification;
4. keep live credential validation separate from source-level adapter completeness.


### 44. Milestone Update — IAK Prepaid Adapter Foundation

**Date:** 2026-09-25

CI run #247 for the DigiFlazz PLN inquiry milestone is confirmed **green** before starting this milestone:

- workflow — success;
- `test` — success (`go test ./...` and `go vet ./...`);
- `race` — success (`go test -race ./...`).

The DigiFlazz adapter now covers the main verified v0.1 neutral capabilities currently implemented in source: prepaid product catalog, PLN inquiry, purchase, status, webhook normalization, and balance synchronization. Live provider credential validation remains separate and has not been claimed.

Based on the documented roadmap checkpoint, the next provider is IAK. Official IAK documentation confirms a prepaid REST API, MD5 authentication using `md5(username+api_key+additional)`, prepaid price list, PLN inquiry, top-up, check-status, callback, and balance capabilities. The v2 prepaid documentation also states that the first top-up response is processing and that reusing the same `ref_id` becomes a status check rather than a new transaction. citeturn0search3turn1search1turn2search0turn3view0

Implemented:

- added environment-backed `IAKConfig`;
- added IAK prepaid base URL and endpoint overrides;
- implemented provider-neutral IAK `GetProducts` against the documented prepaid price-list operation;
- implemented IAK PLN inquiry using the documented `customer_id` and MD5 signature;
- implemented IAK prepaid top-up mapping;
- implemented IAK prepaid status lookup by `ref_id`;
- implemented IAK balance lookup;
- implemented IAK callback/webhook normalization with the documented `md5(username+api_key+ref_id)` signature when a signature secret is supplied;
- added deterministic `httptest` coverage for price list, inquiry, purchase, status, balance, webhook signature, and unsupported inquiry;
- registered IAK in runtime only when IAK credentials are configured, preserving existing DigiFlazz-only test/runtime setups;
- documented IAK environment settings in `.env.example`.

The adapter intentionally targets the verified **prepaid v2-style endpoints**. Postpaid billing inquiry/payment is not included in this milestone because those operations have a different API contract and must remain a separate provider capability boundary. citeturn2search1turn1search6

### Safety Boundary

This milestone does not:

- introduce automatic transaction retry or cross-provider failover;
- resubmit ambiguous transactions outside the existing transaction-correlation service;
- mutate the DesKaCash ledger;
- fund providers;
- treat IAK sandbox access as production approval;
- claim live IAK credential or commercial verification.

The IAK documentation explicitly describes prepaid processing as asynchronous and supports callback or status-check flows. The adapter therefore preserves the existing pending/status/callback model instead of adding an unsafe immediate retry loop. citeturn1search7turn1search0

### Verification Gate

Implementation commits:

- `58fbd9f599d9a215476279bb04c78172f07fa805` — IAK configuration;
- `f1998fa136ee501d17d9f9ebc96907ff59949f9d` — IAK adapter;
- `0eb27dcbb3ef647e33e56717c83bd075f8fc145f` — deterministic adapter tests;
- `e781430555a0457381437daa9f3ecdb9b89e6652` — optional IAK runtime registration;
- `cd4929bb183d63c11fc96178a6c8d2e0c3dd0bbf` — environment documentation.

A fresh CI run for this IAK milestone is required. `test`, `vet`, and `race` must all be green before the next implementation step.

### 45. Milestone Update — IAK Credential-Gated Read-Only Integration Harness

**Date:** 2026-09-25

CI #259 for the IAK adapter foundation is confirmed **green** before this milestone:

- workflow — success;
- `test` — success (`go test ./...` and `go vet ./...`);
- `race` — success (`go test -race ./...`).

Implemented a live integration-test harness that is intentionally read-only and credential-gated:

- added `IAK_INTEGRATION=1` as the explicit opt-in;
- requires `IAK_USERNAME` and `IAK_API_KEY` from the runtime environment;
- validates `LoadIAKConfig()` and adapter construction;
- calls IAK balance lookup;
- calls IAK prepaid price-list lookup;
- does not submit a purchase/top-up transaction;
- skips cleanly when integration mode or credentials are absent, so normal CI remains credential-free;
- no credentials or provider secrets are stored in Git.

The harness is a validation mechanism, not proof of production activation. A live run still requires valid IAK runtime credentials and the provider account to be enabled/configured for API access.

### Safety Boundary

The live harness deliberately avoids purchase execution. This keeps source-level verification separate from any financial transaction and avoids creating a real provider-side order merely to prove connectivity.

### Verification Gate

- previous IAK foundation CI #259 — **green**;
- integration harness commit: `ac56cf40e9084903572dd2027e01251ecc6ea3b4`;
- a fresh CI run for this commit is required and must have `test`, `vet`, and `race` green.

### Next Milestone

1. verify the fresh CI result for the IAK read-only harness;
2. if green, review IAK catalog freshness and operational balance semantics against the existing neutral boundaries;
3. only after that decide whether additional IAK capability work is required before the next roadmap provider;
4. keep live credentials, commercial activation, and production verification separate from source-level adapter completeness.


### 46. Milestone Update — IAK Balance Contract Hardening

**Date:** 2026-09-25

CI #263 for the IAK read-only integration harness is confirmed **green** before this milestone:

- workflow — success;
- `test` — success;
- `race` — success.

Reviewed the existing neutral catalog freshness and operational balance boundaries against the IAK adapter. No IAK-specific catalog freshness override is required: IAK product data is stored and routed through the same provider-neutral catalog snapshot and maximum-age policy. IAK balance is likewise an operational snapshot, not customer funds or a ledger balance.

Hardened the IAK balance adapter so malformed or incomplete provider responses cannot silently become a valid zero balance:

- `data.balance` is now required;
- numeric JSON balances are accepted;
- numeric string balances are accepted;
- missing, malformed, or unsupported balance values return an error;
- added deterministic tests for missing, invalid, and string balance responses;
- added an explicit capability assertion that the IAK client implements both `PPOBProvider` and `BalanceProvider`.

This prevents a malformed IAK balance response from being persisted as a healthy `0` balance by the operational synchronization layer.

### Safety Boundary

This milestone does not:

- add automatic transaction retry or cross-provider failover;
- treat IAK balance as customer balance;
- mutate the DesKaCash ledger;
- submit live purchases;
- store provider credentials;
- bypass the existing catalog freshness boundary.

### Verification Gate

Implementation commits:

- `936df6292567464bb97e572fb800773af4eeddbc` — validate IAK balance responses;
- `38e03536003ea03fc33f9779b0a1d4bfaece5725` — deterministic balance/capability tests.

A fresh CI run for this hardening change is required. `test`, `vet`, and `race` must all be green before the next provider-development step.

### Next Milestone

1. verify the fresh CI result for IAK balance hardening;
2. if green, continue IAK contract review only where a verified provider API gap exists;
3. otherwise keep the adapter stable and avoid speculative provider-specific behavior;
4. keep live credentials and commercial activation separate from source-level completeness.


### Verification Follow-up — IAK Balance Hardening CI

CI run #267 for the IAK balance hardening implementation is confirmed **green**:

- workflow — success;
- `test` — success (`go test ./...` and `go vet ./...`);
- `race` — success (`go test -race ./...`).

The verified implementation commit for the code change is `38e03536003ea03fc33f9779b0a1d4bfaece5725`. The status-document-only follow-up commit does not change executable code.

The IAK balance hardening milestone is therefore cleared for the next development step.


### 47. Milestone Update — IAK Response Envelope Hardening

**Date:** 2026-09-25

CI #271 is confirmed **green** before this milestone. The existing IAK balance hardening and status-document follow-up are therefore cleared.

Official IAK documentation was reviewed for the currently implemented prepaid capabilities. The documented price-list response requires a `pricelist` array and exposes `status`, `product_code`, `product_description`, and `product_category`; the documented PLN inquiry response requires `status`, `customer_id`, `message`, and `rc`. IAK also documents REST/HTTPS usage and JSON responses. citeturn5view0turn6view0turn4view0

Hardened the adapter so an HTTP 2xx response cannot silently be treated as a valid provider result when the required response envelope is absent:

- `GetProducts` now requires `data.pricelist` to be an array;
- `Inquiry` now requires a recognized IAK status (`1` or `2`);
- unknown inquiry status values are rejected instead of leaking arbitrary provider values into the neutral transaction-status field;
- deterministic tests cover malformed product-list responses, missing inquiry status, and unknown inquiry status.

This keeps provider-level failures distinguishable from valid empty product catalogs and valid inquiry outcomes. The change does not infer undocumented response semantics.

### Safety Boundary

No transaction retry, failover, ledger mutation, provider funding, or live purchase execution was added.

### Verification Gate

Implementation commits:

- `5027fdf2efb19cbe98c7b28fea83da42541cd60f` — response envelope validation;
- `26e9ab7bfb41430abef937ae8a6a91e02e4b404a` — deterministic response-validation tests.

A fresh CI run for this hardening change is required. `test`, `vet`, and `race` must all be green before the next step.

### Next Milestone

1. verify fresh CI for the IAK response-envelope hardening;
2. if green, continue only with verified IAK capability gaps;
3. keep postpaid, OVO, game inquiry, and other capabilities outside the prepaid v0.1 adapter until their neutral contract is explicitly designed and provider documentation is verified.


### CI Correction — IAK Response Hardening

CI #275 exposed a compile failure in the response-hardening change before the tests could run. The failure was caused by an incorrectly escaped newline inserted into `iak.go`; the resulting parser error also produced a cascading test-file diagnostic.

Correction committed as `a6aeb2d1af49aa2972e02a239a7db2941fe5ba8d`. No provider behavior was changed by this correction; it only restores valid Go syntax for the helper introduced in the previous milestone.

CI #275 is recorded as **red** and is not accepted as the verification gate. A fresh CI run on the correction commit is required and must be fully green before proceeding.


### Verification Follow-up — IAK Response Envelope Hardening CI

CI run #285 for the corrected IAK response-validation implementation is confirmed **green**:

- workflow — success;
- `test` — success (`go test ./...` and `go vet ./...`);
- `race` — success (`go test -race ./...`).

The implementation is therefore cleared for the next IAK capability review. The earlier red runs #275, #279, and #281 are retained as historical failures and are not treated as successful verification.


### 48. Milestone Update — IAK Transaction Envelope and Identity Hardening

**Date:** 2026-09-25

CI run #287 for the previous status-document head is confirmed **green** before this implementation step:

- workflow — success.

Reviewed the current official IAK prepaid v2 documentation. IAK requires `ref_id`, `product_code`, `customer_id`, and transaction `status` in top-up responses; check-status likewise returns transaction identity and status. IAK callbacks return `ref_id`, `code`, `hp`, and status, with only success/failed callbacks documented. citeturn0search0turn0search2turn0search3

Hardened the IAK adapter so valid HTTP/JSON envelopes cannot be accepted when transaction identity or status is malformed:

- `Purchase` now requires response `ref_id`, `customer_id`, and `product_code`;
- purchase status must be one of `PROCESS`, `SUCCESS`, or `FAILED`;
- purchase response identity is checked against the original request;
- `GetStatus` now requires transaction identity and a recognized status;
- status response identity is checked against the requested reference and, when supplied, customer/product fields;
- webhook normalization now rejects missing transaction identity and unknown status values;
- added deterministic tests for malformed purchase/status envelopes, identity mismatch, and malformed webhook status.

This closes a provider-boundary validation gap without inventing additional IAK response semantics.

### Safety Boundary

This milestone does not:

- introduce automatic transaction retry or cross-provider failover;
- resubmit ambiguous transactions;
- mutate the DesKaCash ledger;
- fund providers;
- treat provider balance as customer balance;
- claim live IAK credential or production activation.

### Verification Gate

Implementation commits:

- `a5cf1545b621656230bd233ed8949573019ebb38` — transaction response validation and identity checks;
- `e22d5b4d59cc888037e25df2d9f1ef78a430f169` — deterministic transaction-envelope tests.

A fresh CI run for the current head is required. `test`, `vet`, and `race` must all be green before the next development step.

### Next Milestone

1. verify the fresh CI result for IAK transaction envelope hardening;
2. if green, review remaining verified IAK capability gaps only;
3. avoid speculative postpaid/game/OVO/eSIM expansion until a neutral contract is explicitly required;
4. keep live credentials and commercial activation separate from source-level adapter completeness.


### 49. Milestone Update — IAK Pricelist Item Contract Hardening

**Date:** 2026-09-25

The previous milestone's current-head CI is confirmed **green** before this follow-up: CI run #293 completed successfully.

Official IAK prepaid v2 price-list documentation requires each `pricelist` item to provide `product_code`, `product_description`, `product_details`, `product_nominal`, `product_price`, `product_type`, `active_period`, `status`, `icon_url`, and `product_category`. citeturn0search0

The adapter previously validated only that `data.pricelist` was an array. It now also rejects malformed list entries and entries missing the neutral fields needed by DesKaProvider:

- each pricelist entry must be a JSON object;
- `product_code` is required;
- `product_description` is required;
- malformed entries cannot silently become an empty `Product`;
- deterministic tests cover invalid item types and missing required neutral fields.

The adapter still deliberately does not copy provider-specific fields such as nominal, price, icon, or active-period into the neutral `Product` contract because those fields are not currently part of the provider-neutral interface.

### Safety Boundary

No transaction retry/failover, ledger mutation, provider funding, or live purchase execution was added.

### Verification Gate

Implementation commits:

- `fbe2a3424774ce5c18e09decd55f94b8fa054311` — pricelist item validation;
- `3f987fa4e58dd74dd154396b772e42547824f706` — deterministic tests.

A fresh CI run for the current head is required. `test`, `vet`, and `race` must all be green before continuing.

### Next Milestone

1. verify the current-head CI;
2. if green, review whether any remaining verified IAK v0.1 gap is material to the existing neutral contract;
3. otherwise keep IAK stable and prepare the next roadmap provider rather than expanding the neutral contract speculatively.

### 50. Milestone Update — IAK Required Transaction Field Hardening

**Date:** 2026-09-25

CI run #299 for the previous current head is confirmed **green** before this implementation step.

Official IAK prepaid v2 documentation requires transaction responses to include `ref_id`, `status`, product/customer identity, `price`, `message`, `balance`, `tr_id`, and `rc`. The neutral DesKaProvider transaction contracts currently expose identity, normalized status, provider code/message, serial number, and price; provider-only balance and transaction ID remain outside that neutral contract. https://api.iak.id/api/prepaid/core/v2/transaction/top-up https://api.iak.id/api/prepaid/core/v2/check-status

Hardened the IAK adapter only for required fields that map directly to the existing neutral contract:

- PLN inquiry now rejects a missing `message` field;
- purchase responses now require a valid `price` and non-empty `message`;
- check-status responses now require a valid `price` and non-empty `message`;
- webhook payloads now require a valid `price` and non-empty `message`;
- numeric price values are accepted from JSON numbers or numeric strings;
- deterministic tests cover missing message/price cases for purchase and status plus inquiry/webhook field validation.

The implementation deliberately does **not** add IAK `balance` or `tr_id` to the neutral contracts merely to mirror provider-specific response fields. Those values remain provider-specific until a concrete neutral use case requires them.

### Safety Boundary

No transaction retry/failover, ledger mutation, provider funding, or live purchase execution was added.

### Verification Gate

Implementation commits:

- `3707dc54fd282632743af804796079e28f43756b` — required IAK transaction field validation;
- `63280ec2a549f6af53a3fe5cab58f17fcdcd43d8` — deterministic validation tests.

A fresh CI run for the current head is required. `test`, `vet`, and `race` must all be green before the next provider-development step.

### Next Milestone

1. verify the current-head CI;
2. if green, keep IAK stable unless another verified gap materially affects the existing neutral contract;
3. prepare the next roadmap provider, XP SINDONESIA, only after its concrete API contract is verified;
4. do not invent an XP adapter endpoint/signature/request schema from the public product pages alone.

### CI Correction — IAK Required Transaction Field Hardening

CI run #303 exposed fixture failures after the new required `price` validation was added. Production adapter behavior was not the failure: the existing deterministic purchase/status/webhook success fixtures omitted the now-required provider field.

Correction committed as `01501cb25d1bd5435908dd2fb2b672515aad09f0`:

- updated valid purchase fixture with `price`;
- updated valid status fixture with `price`;
- updated valid webhook fixture with `price`;
- no production adapter behavior was changed by this correction.

CI #303 is recorded as **red** and is not accepted as the verification gate. A fresh CI run for the correction commit must be fully green before proceeding.
### 51. Milestone Update — IAK Gate Cleared / XP SINDONESIA Contract Discovery

**Date:** 2026-09-25

CI run **#307 is confirmed GREEN** for correction commit `01501cb25d1bd5435908dd2fb2b672515aad09f0`. The previous IAK hardening fixture failure is therefore cleared.

IAK v0.1 is kept stable after the verified response-field hardening. No further neutral-contract expansion is justified by the currently verified IAK gaps.

Next roadmap provider review: **XP SINDONESIA**.

External research confirms the public XP SINDONESIA site exposes live product/catalog pages and public product codes/pricing, including prepaid products and other PPOB categories. However, the public pages reviewed do not provide a sufficiently concrete H2H API contract for safely implementing an adapter: no verified request endpoint, authentication/signature formula, transaction request schema, status schema, or webhook authentication contract was established from the public material reviewed. The public catalog is therefore treated as product evidence, not API authorization.

The repository currently contains only the XP adapter placeholder `DesKaProvider/backend/Provider/XPSindonesia/xp_sindonesia.go`; it contains no implementation contract to preserve or extend.

### Implementation Decision

- do **not** invent XP endpoints, credentials, signatures, or JSON schemas from the public catalog;
- do **not** implement a speculative `PPOBProvider` adapter;
- do **not** add provider-specific fields to the neutral contract just to accommodate an unverified API;
- keep XP adapter implementation pending until official/API-access documentation or concrete provider credentials/documentation establish the H2H contract;
- continue using deterministic tests and the existing provider-neutral boundary once the contract is verified.

### Safety Boundary

No live XP transaction, credential handling, retry/failover, ledger mutation, provider funding, or speculative API integration was added.

### Verification Gate

- CI #307 — **success**;
- IAK hardening gate — **cleared**;
- XP SINDONESIA API contract — **not yet sufficiently verified for implementation**.

### Next Milestone

1. obtain/verify the concrete XP SINDONESIA H2H API documentation or provider-issued integration specification;
2. define only the minimum neutral mapping required by the existing `PPOBProvider` contract;
3. implement XP adapter + deterministic HTTP tests;
4. add credential-gated read-only integration checks where the verified API supports them;
5. require fresh CI `test`, `vet`, and `race` green before moving to the next provider.

### 52. Milestone Update — CI Gate Re-verified / XP SINDONESIA Discovery Reconfirmed

**Date:** 2026-09-25

Before continuing XP SINDONESIA work, the current branch head was re-checked. GitHub Actions workflow run **#311** for commit `71170c0513c6f4c2bf3f78383506d746e93374a0` completed successfully.

The required CI jobs are both green:

- `test` — success (`go mod tidy`, `go test ./...`, `go vet ./...`);
- `race` — success (`go test -race ./...`).

The CI gate is therefore **GREEN** and development can continue.

### XP SINDONESIA Contract Discovery — Current Finding

A fresh review of the public XP SINDONESIA material confirms that the site exposes public product catalogs with product codes, prices, and availability/status. The public catalog includes prepaid products such as Telkomsel, Indosat, Axis, Three, and XL, as well as data products. These pages are useful as product/catalog evidence. citeturn0search1turn0search0

The public homepage also confirms that XP SINDONESIA offers automated payments for products including internet/data, GSM/CDMA pulsa, PLN postpaid checking/payment, PLN tokens, and game vouchers. citeturn1search0

However, the reviewed public material still does **not** establish a concrete H2H API contract suitable for safe adapter implementation. In particular, no verified provider API endpoint, authentication/signature formula, transaction request schema, transaction/status response schema, or webhook authentication contract was established from the public pages reviewed.

Therefore the implementation boundary remains unchanged:

- do **not** invent XP endpoints, credentials, signatures, or JSON schemas;
- do **not** implement a speculative `PPOBProvider` adapter;
- do **not** infer H2H API behavior from the public product catalog;
- keep the existing XP adapter placeholder until official/API-access documentation or a provider-issued integration specification is available.

### Safety Boundary

No live XP transaction, credential handling, retry/failover, ledger mutation, provider funding, or speculative API integration was added.

### Verification Gate

- Current branch CI run **#311 — GREEN**;
- `test` — **GREEN**;
- `race` — **GREEN**;
- XP SINDONESIA H2H API contract — **not sufficiently verified for implementation**.

### Next Milestone

1. obtain/verify concrete XP SINDONESIA H2H API documentation or provider-issued integration specification;
2. define only the minimum neutral mapping required by the existing `PPOBProvider` contract;
3. implement XP adapter + deterministic HTTP tests once the contract is verified;
4. add credential-gated read-only integration checks where the verified API supports them;
5. require fresh CI `test`, `vet`, and `race` green before moving to the next provider.

### 53. Milestone Update — XP SINDONESIA API Entry Point Identified, Contract Still Unverified

**Date:** 2026-09-25

After the CI gate was re-verified green, the XP SINDONESIA public site was inspected beyond the product catalog.

The public homepage explicitly exposes an **API** navigation entry pointing to:

`https://xp.sindonesia.net/api/`

This confirms that an API-facing area exists on the provider site. citeturn3view0

However, the API page itself could not be retrieved by the available web source because the page returned a cache/fetch miss. Therefore the following contract elements remain unverified:

- API endpoint(s) and HTTP method(s);
- authentication/credential format;
- signature or hashing formula;
- product/pricelist request and response schema;
- transaction request/response schema;
- transaction status semantics;
- callback/webhook endpoint and authentication;
- balance endpoint and response schema.

An older third-party search result also mentions `xp.sindonesia.net/api.php`, but it is historical, external material and does not establish the current provider contract. It is therefore **not** used as implementation evidence.

### Implementation Decision

The XP adapter remains intentionally unimplemented. The newly identified `/api/` entry point is a lead for contract acquisition, not sufficient authorization to invent or infer the protocol.

No speculative endpoint, credential, signature, request schema, status mapping, webhook parser, or balance adapter was added.

### Verification Gate

- CI run **#313 — GREEN** for status-document commit `b625b000f71cbc47890dd01be484e2b4a6a85f4e`;
- `test` — **GREEN**;
- `race` — **GREEN**;
- XP API entry point — **identified**;
- XP H2H contract — **still not sufficiently verified for implementation**.

### Next Milestone

1. obtain the actual XP API documentation/content or provider-issued integration specification;
2. verify authentication, transaction, status, callback, and balance contracts;
3. implement only the minimum existing `PPOBProvider` mapping supported by verified documentation;
4. add deterministic HTTP tests;
5. add credential-gated read-only integration checks where supported;
6. require fresh CI `test`, `vet`, and `race` green.

### 54. Milestone Update — XP API Research Boundary Reconfirmed

**Date:** 2026-09-25

CI run **#315** for the previous status-document commit is confirmed **GREEN**:

- `test` — success (`go mod tidy`, `go test ./...`, `go vet ./...`);
- `race` — success (`go test -race ./...`).

Additional XP SINDONESIA API research was performed after the green gate.

The official public site continues to provide product/catalog evidence, including product codes, prices, and availability/status. citeturn2search0turn2search1

The official homepage exposes an API navigation entry, but direct retrieval of `https://xp.sindonesia.net/api/` still fails through the available web source with a cache/fetch miss. No official API contract content was therefore obtained.

A historical third-party search result references an older `xp.sindonesia.net/api.php` endpoint and mentions POST/JSON behavior. This is treated only as historical lead material, not as current API documentation or authorization to implement the adapter. citeturn2search3

### Implementation Decision

XP SINDONESIA remains blocked at contract-discovery stage.

Do not implement based on the historical third-party reference because it does not verify the current:

- endpoint and HTTP contract;
- authentication/credential format;
- signature formula;
- product/pricelist schema;
- transaction request/response schema;
- status semantics;
- callback/webhook contract;
- balance contract.

No speculative XP adapter code was added.

### Verification Gate

- Previous CI run **#315 — GREEN**;
- current XP contract — **still insufficiently verified for implementation**.

### Next Milestone

1. obtain the current XP API documentation/content or provider-issued integration specification;
2. verify authentication, transaction, status, callback, and balance contracts;
3. implement the minimum existing `PPOBProvider` mapping;
4. add deterministic HTTP tests;
5. add credential-gated read-only integration checks where supported;
6. require fresh CI `test`, `vet`, and `race` green.

### 55. Milestone Update — Official XP API Link Reconfirmed; Contract Still Unavailable

**Date:** 2026-09-25

The previous status commit `2675853bdc95af0a5ddbba5163dfc4a8d2e9ab8b` has now passed CI **#317 — GREEN**:

- `test` — success;
- `vet` — success;
- `race` — success.

Fresh official-site research confirms the XP SINDONESIA homepage exposes an **API** navigation entry linking directly to `https://xp.sindonesia.net/api/`. citeturn1view0

The same official site provides current public product catalog evidence for prepaid pulsa and internet/data products, including product codes, prices, and READY/KOSONG status. citeturn0search2turn0search3

However, the official API page itself still cannot be retrieved by the available web source (cache/fetch miss). Therefore the following remain **unverified**:

- HTTP method and exact endpoint contract;
- authentication and credential fields;
- request signature/hash formula;
- pricelist response schema;
- transaction request/response schema;
- transaction status semantics;
- callback/webhook authentication and payload;
- balance endpoint and response schema.

### Implementation Decision

No XP adapter implementation is added in this milestone.

Public catalog data is sufficient to confirm XP is a relevant PPOB product source, but it is **not sufficient to safely derive an H2H integration contract**. The existing `PPOBProvider` interface therefore remains unchanged and no speculative authentication, endpoint, status mapping, or webhook behavior is introduced.

### Verification Gate

- CI #317 — **GREEN**;
- official API navigation — **confirmed**;
- official API contract content — **not yet retrievable / insufficient for implementation**.

### Next Milestone

Proceed only when the current XP API contract is obtained from the provider's API documentation or provider-issued integration material. Then implement the minimum existing `PPOBProvider` mapping with deterministic HTTP tests and credential-gated read-only integration checks where supported.

### 56. Milestone Update — XP Configuration Contract Added

**Date:** 2026-09-25

The provider-supplied XP SINDONESIA documentation in `docs/XP_SINDONESIA_API_DOC.md` is now the implementation basis.

Verified configuration fields from that document:

- numeric member `id`;
- `key`;
- `api`;
- separate POST endpoints for saldo, harga, daftar harga, and order.

Added `backend/config/xp_sindonesia.go` with environment-backed configuration and documented endpoint defaults.

No credentials or secret values are committed.

### Verification Gate

- CI #319 for the previous commit — **GREEN**;
- XP documentation — available in repository and ready for adapter implementation;
- configuration layer — added;
- XP transaction adapter — not yet implemented in this milestone.

### Next Milestone


### 61. Milestone Update — Owned Sync Worker Lifecycle & Restart Recovery

**Date:** 2026-09-25

Completed:

- added `SyncWorkerLifecycle` as the explicit owner of the operational sync worker goroutine;
- startup creates a child cancellation context and starts the existing `SyncService.Run` worker;
- duplicate startup is rejected with `ErrSyncWorkerRunning`;
- shutdown explicitly cancels the owned worker and waits for clean termination;
- normal `context.Canceled` termination is treated as a clean shutdown result;
- lifecycle ownership remains outside `SyncService`, preserving `SyncService.Run` as the synchronization primitive;
- added deterministic lifecycle tests for immediate startup synchronization, duplicate-start protection, idempotent shutdown, and durable snapshot recovery across service restart;
- restart coverage verifies that a persisted operational balance is loaded first and then refreshed by a new worker instance.

### Verification Gate

- prior HEAD `f1b5ad0dff82fb884dd5dce5aa7c00455996b222` — CI #348 **GREEN** (`test`, `vet`, `race`);
- lifecycle implementation and tests are committed in the current branch;
- a fresh CI run for the new implementation is required and must be **GREEN** before the next feature milestone;
- no provider request, credential, retry/failover, automatic funding, or ledger mutation was added.

### Safety Boundary

- lifecycle cancellation is explicit and owned by the service lifecycle wrapper;
- worker shutdown does not mutate financial state beyond the already-defined operational snapshot synchronization;
- persisted provider balance remains an operational snapshot and is not customer balance or treasury state.

### Known Limitations

- the lifecycle wrapper currently owns only the provider balance synchronization worker;
- broader application startup/shutdown coordination for HTTP/Admin API and future workers is not yet implemented;
- PostgreSQL remains the documented production persistence target; JSON persistence remains the interim v0.1 runtime boundary.

### Next Milestone

1. verify fresh CI `test`, `vet`, and `race` for this milestone;
2. if green, add the production-oriented separation of provider lifecycle (`ENABLED`/`DISABLED`) from health (`HEALTHY`/`DEGRADED`/`UNHEALTHY`) and capabilities;
3. then use those states as prerequisites for provider routing.


### 62. Milestone Update — Provider Lifecycle, Capability, and Health Separation

**Date:** 2026-09-25

CI gate was re-verified before continuing:

- CI run #354 — GREEN for commit 5dc93bf4ba7d9a80d3491a52bef0002a1e5c2673;
- test — success;
- vet — success;
- race — success.

Completed after the green gate:

- added a provider-neutral ProviderState model;
- lifecycle is explicitly limited to ENABLED / DISABLED;
- provider capabilities are modeled independently as PAYMENT, PPOB, PAYOUT, BALANCE, and WEBHOOK;
- lifecycle defaults to DISABLED, preventing a newly registered provider from becoming routable merely by registration;
- added a thread-safe ProviderStateStore with normalized provider names and deterministic ordering;
- defensive copies prevent callers from mutating stored capability state through returned slices;
- invalid lifecycle values are rejected;
- deterministic tests cover lifecycle defaults, capability separation, defensive copies, and invalid lifecycle rejection.

### Architectural Boundary

Provider health remains in the existing operational Snapshot.Health model. It is deliberately not merged into lifecycle state:

- lifecycle answers whether the provider is administratively enabled;
- health answers whether recent operational synchronization is healthy/degraded/unhealthy;
- capabilities answer which provider-neutral operations are available.

Routing must require all applicable dimensions rather than treating one state as a substitute for another.

### Safety Boundary

- no automatic enablement;
- no automatic funding;
- no provider-specific routing logic;
- no retry/failover;
- no ledger mutation;
- no credential changes;
- no live external provider requests.

### Verification Gate

The lifecycle/capability implementation is now committed, but a fresh CI run for the new commits is required. test, vet, and race must all be green before provider routing is implemented.

### Next Milestone

1. verify fresh CI for the provider-state implementation;
2. if green, integrate lifecycle/capability/health state into provider selection prerequisites;
3. only then continue with provider routing behavior and deterministic selection tests.


### 63. Milestone Update — State-Aware Provider Routing Prerequisites

**Date:** 2026-09-25

CI gate before implementation:

- CI run #360 — **GREEN** for commit `e0cb4dbbba97efcc9c7e43c6e282ad47478ca816`;
- `test` — success;
- `vet` — success;
- `race` — success.

Completed:

- routing can now optionally receive `ProviderStateStore`;
- state-aware constructors were added without changing the existing provider-neutral Router contract;
- when provider state is configured, a candidate must be administratively `ENABLED`, explicitly capable of `PPOB`, operationally `HEALTHY`, funded above the requested amount, and present in a fresh catalog when catalog routing is enabled;
- lifecycle, capability, and operational health remain separate prerequisites;
- existing deterministic priority and provider-name tie-breaking remain unchanged;
- deterministic tests cover disabled providers, wrong capabilities, and an eligible provider;
- no automatic fallback/retry/failover was introduced.

### Routing Safety Boundary

The state-aware router is intentionally an eligibility gate, not a provider-specific policy engine. It does not inspect provider names or external API fields.

A provider being registered is insufficient for routing. Explicit administrative enablement and capability declaration are required when the state store is configured.

### Verification Gate

The state-aware routing changes are committed, but a **fresh CI run for the new commits is required**. `test`, `vet`, and `race` must all be GREEN before continuing to routing execution/idempotency changes or Admin API work.

### Next Milestone

1. verify fresh CI for state-aware routing;
2. if green, integrate the state-aware router into the existing purchase service construction path without bypassing the state gate;
3. then add deterministic routing eligibility tests covering health degradation and stale operational snapshots.


### 64. Milestone Update — Runtime State Gate Integration

**Date:** 2026-09-25

CI prerequisite verified:

- CI #368 — **GREEN** for commit `d974976586600b27d4c5dfb43b879709f0760b4c`;
- `test` — success;
- `vet` — success;
- `race` — success.

Completed:

- `runtime.NewFromEnvironment` now creates a `ProviderStateStore` and passes it into the catalog-aware Router;
- every runtime-registered provider receives an explicit capability declaration for the currently supported PPOB, balance, and webhook interfaces;
- providers remain `DISABLED` by default and are therefore not routable until an administrative enablement mechanism exists;
- runtime tests verify the state store is present, capabilities are declared, and automatic enablement does not occur;
- existing provider-neutral routing, operational health, catalog freshness, and transaction persistence remain intact.

Safety boundary:

- runtime registration is not equivalent to administrative enablement;
- no automatic provider activation was introduced;
- no provider-specific routing condition was introduced;
- no retry/failover or financial ledger mutation was introduced.

Verification gate:

- fresh CI is required for the new runtime integration before the next milestone;
- `test`, `vet`, and `race` must all be GREEN.

Next milestone:

1. verify fresh CI for runtime state-gate integration;
2. then define the minimal administrative state mutation boundary needed to enable/disable providers safely;
3. only after that, continue with routing health/freshness eligibility and Admin API integration.



### 65. Milestone Update — Minimal Administrative Provider Lifecycle Boundary

**Date:** 2026-09-25

CI gate before implementation:

- CI #374 — **GREEN** for commit `b105b434f271a1574ab47e04365f6b077b0b29fa`;
- workflow completed successfully;
- the runtime state-gate milestone therefore passed the mandatory CI gate before feature work continued.

Completed:

- added `ProviderAdminService` as the internal administrative control-plane boundary for provider lifecycle mutation;
- lifecycle changes are limited to `ENABLED` / `DISABLED`;
- administrative mutation requires an already-registered provider state;
- unknown or empty provider names return `ErrProviderNotFound`;
- invalid lifecycle values return `ErrInvalidLifecycle`;
- lifecycle mutation preserves the provider's capabilities unchanged;
- administrative lifecycle mutation does not alter operational health;
- added deterministic tests for enable/disable behavior, unknown providers, invalid lifecycle values, capability preservation, and concurrent lifecycle mutation;
- no HTTP/Admin API endpoint, authentication mechanism, provider credential mutation, automatic funding, retry/failover, or ledger mutation was introduced.

### Administrative Safety Boundary

The new service is intentionally a domain/control-plane boundary rather than a public API:

`Admin Web/API -> ProviderAdminService -> ProviderStateStore`

The service does not expose provider-specific API fields and does not permit administrative lifecycle changes to masquerade as health or capability changes.

Provider lifecycle remains separate from:

- health state in the operational snapshot;
- provider capabilities;
- provider balance/liquidity;
- financial ledger state.

### Verification Gate

- implementation commits: `fdc145cb75863ec1bf7422099ae5f84a0a699c44`, `08807e4e9bbf8ed845da15ed2ca956df55368a53`;
- a fresh CI run for the new administrative boundary is required;
- `test`, `vet`, and `race` must all be **GREEN** before continuing.

### Next Milestone

1. verify CI for the administrative lifecycle boundary;
2. determine the durable persistence boundary for administrative provider lifecycle state before exposing an Admin API;
3. then add routing eligibility tests for degraded/unhealthy and stale operational snapshots;
4. only after those controls are stable, proceed toward the internal Admin API/control plane.


### 66. Milestone Update — CI Failure Fix: Provider Admin Return Contract

**Date:** 2026-09-25

CI run #380 for milestone 65 was **RED** in both `test` and `race`.

ROOT CAUSE:

- `ProviderAdminService.SetLifecycle` returned `s.states.Get(name)` directly;
- `ProviderStateStore.Get` returns `(ProviderState, bool)`, while the service method requires `(ProviderState, error)`;
- this produced a compile error and prevented downstream packages from building.

IMPACT:

- provider lifecycle admin boundary could not compile;
- `test` and `race` failed before executing the new administrative tests;
- `vet` was skipped because the test job failed.

FIX:

- retrieve the updated state explicitly;
- return the state with a nil error after successful persistence.

VERIFICATION:

- fix committed as `6ef962736d1d3fb311c74baa41e7ed9101bc2bc8`;
- a fresh CI run is required;
- development remains blocked until `test`, `vet`, and `race` are all **GREEN**.

REMAINING RISK:

- no durable provider lifecycle persistence has been added yet;
- Admin API remains intentionally deferred until lifecycle persistence and authorization boundaries are defined.

### Next Milestone

1. verify fresh CI for the compile fix;
2. only after GREEN, continue with durable lifecycle-state persistence design/implementation;
3. then proceed toward internal Admin API integration.


### 67. Milestone Update — Durable Provider Lifecycle State

**Date:** 2026-09-25

CI gate:

- CI #384 for `d28dfccd20afe15636795ac41385fb0bcb20f7d1` is **GREEN**;
- `test`, `vet`, and `race` all completed successfully;
- only after this gate was the durable lifecycle-state milestone started.

Completed:

- introduced `ProviderStatePersistence` as the persistence boundary for provider lifecycle state;
- added `JSONFileProviderStateStore` for durable local operational persistence;
- provider state is loaded during runtime startup;
- provider lifecycle changes through `ProviderStateStore.Put` are persisted before the in-memory state is committed;
- failed persistence therefore does not leave an uncommitted lifecycle mutation in memory;
- persistence uses temporary files, restrictive `0600` file permissions, sync, and atomic rename;
- runtime now uses `DESKAPROVIDER_PROVIDER_STATE_STORE_PATH`, defaulting to `data/provider-state.json`;
- existing provider lifecycle state survives runtime restart;
- runtime provider registration still does not automatically enable a provider;
- existing lifecycle state is preserved while runtime registration refreshes the explicit capability set;
- deterministic tests cover restart recovery, persistence failure atomicity, and file permissions.

Safety boundary:

- lifecycle state remains separate from health, capabilities, balance/liquidity, and ledger state;
- no automatic funding, retry/failover, credential mutation, or public API was introduced;
- Admin API remains deferred until authorization/control-plane requirements are defined.

Verification:

- implementation commits: `4f6ebb3e22d01b799f3d68ea449f0894b81c787a`, `c014029546975111945861544e32925ccc50be45`, `d33120f96cd3aeb339924d38fbb6a64433d36ead`, `059d1bb221efbdcdd17907b62118d13e8cf1d702`, `b404a36359c21486492fbfb3d9f651de57815e19`, `f6891e5431faa5abbcd1a62543b625d10dd36c5a`;
- fresh CI is required for this milestone before proceeding.

### Next Milestone

1. verify durable lifecycle-state CI;
2. add restart-focused administrative lifecycle tests through `ProviderAdminService`;
3. then add routing eligibility tests combining lifecycle, health, and catalog freshness;
4. only after those controls are stable, continue toward the internal Admin API.


### 68. CI Recovery — Durable Provider Lifecycle State

**Date:** 2026-09-25

CI #398 for `c567f7d5647e38596eb79710f0dc152971c3f9a5` was **RED** in both `test` and `race`.

**ROOT CAUSE**

- `DesKaProvider/backend/Provider/operational/provider_state_json.go` contained a malformed Go struct tag on `providerStateFile.States`;
- this caused a compile-time syntax error before the operational, routing, runtime, and command packages could build.

**IMPACT**

- no functional test execution for the new durable provider-state implementation;
- race validation also stopped at compilation;
- previous CI gate remained green, but the new milestone was not verified.

**FIX**

- corrected the JSON tag to `json:"states"`;
- no architectural or runtime behavior was changed.

**Verification**

- fix commit: `c339359c68434f730de8ebc80daca4d7aa272ec9`;
- fresh CI is mandatory before any next milestone;
- no feature work proceeds until `test`, `vet`, and `race` are green.


### 69. CI Recovery — Provider State Store Compile Fix

CI #402 remained **RED** after the previous JSON-tag fix.

**ROOT CAUSE**

- `ProviderStateStore.Put` declared `capabilities` twice in the same scope, producing `no new variables on left side of :=`;
- the same edit also left a recursive `allMemory()` helper that had no valid implementation.

**FIX**

- removed the duplicate normalization block from `Put`;
- removed the invalid recursive helper;
- retained the validated candidate-map + persistence-before-memory-commit behavior.

**Verification**

- fix commit: `434ab26fd1208da60f3363f7420f37361a60d2f`;
- CI must return green for `test`, `vet`, and `race` before the next milestone.

### 70. Milestone Update — Durable Lifecycle Restart Preservation

**Date:** 2026-09-25

CI gate:

- CI #406 for commit `8208856d4d709bc254adff70bfb12d6f3f901e06` is **GREEN**;
- `test`, `vet`, and `race` completed successfully.

Issue found before the next feature milestone:

- runtime provider registration loaded the durable provider-state store correctly, but then recreated every registered provider state with the default `DISABLED` lifecycle;
- this overwrote an administratively enabled lifecycle during process restart.

ROOT CAUSE:

- `runtime.NewFromEnvironment` unconditionally called `NewProviderState(name)` before refreshing capabilities;
- durable state recovery therefore did not actually preserve the lifecycle field.

FIX:

- runtime now first reads the persisted provider state;
- a new state is created only when the provider has no persisted state;
- runtime still refreshes the explicit provider-neutral capability set;
- lifecycle and capabilities remain separate.

Regression coverage:

- added a runtime restart test that enables `digiflazz` through `ProviderAdminService`;
- creates a second runtime using the same provider-state file;
- verifies the provider remains enabled and retains PPOB/balance/webhook capabilities.

Verification gate:

- the implementation commit below requires a fresh CI run;
- `test`, `vet`, and `race` must all be **GREEN** before continuing to routing eligibility work.

Next milestone:

1. verify fresh CI for lifecycle restart preservation;
2. if green, add routing eligibility tests for disabled/unhealthy/degraded/stale/fresh provider states;
3. keep Admin API authorization deferred until requirements are explicit.

### 71. Milestone Update — Routing Eligibility Regression Coverage

**Date:** 2026-09-25

CI gate:

- CI #408 for commit `641bd1519b9b44f463f97e34d3bca79388f6e07e` is **GREEN**;
- `test`, `vet`, and `race` completed successfully.

Implementation:

- added deterministic routing eligibility regression coverage for provider state + operational health + catalog freshness;
- enabled + unhealthy is rejected;
- enabled + degraded is rejected under the current router semantics;
- enabled + healthy + stale catalog is rejected;
- enabled + healthy + fresh catalog is selectable.

No routing behavior was broadened; the milestone locks the existing provider-neutral eligibility rules with tests.

Verification gate:

- fresh CI for the implementation commit is mandatory;
- `test`, `vet`, and `race` must all be **GREEN** before continuing.


### 72. Milestone Update — Operational Snapshot Freshness Gate

**Date:** 2026-09-25

CI gate:

- CI #424 for commit `9889b3852e4dd8d63706bbfa70d4569534647f63` is **GREEN**;
- `test`, `vet`, and `race` completed successfully.

Implementation:

- routing can now enforce an explicit operational snapshot maximum age;
- a provider with a missing or zero `LastCheckedAt` is rejected when freshness enforcement is enabled;
- stale operational snapshots are excluded before catalog/product eligibility is evaluated;
- fresh operational snapshots remain eligible when lifecycle, capability, health, balance, and catalog requirements are satisfied;
- runtime exposes `DESKAPROVIDER_OPERATIONAL_SNAPSHOT_MAX_AGE`;
- default operational snapshot freshness boundary is 2 minutes, while the balance synchronization default remains 30 seconds;
- existing router constructors retain their previous behavior unless the operational freshness boundary is explicitly enabled.

Regression coverage:

- stale operational snapshot is rejected;
- fresh operational snapshot is selectable;
- invalid operational snapshot max-age configuration is rejected.

Safety boundary:

- freshness only affects routing eligibility;
- it does not mutate provider lifecycle;
- it does not mutate health state;
- it does not trigger provider funding;
- it does not introduce retry or failover behavior.

Known limitation:

- the freshness boundary is currently an operational routing gate, not a financial ledger timestamp or source-of-truth mechanism.

Next milestone:

1. stabilize combined routing eligibility with deterministic freshness boundaries;
2. review routing behavior around missing/stale catalog and operational snapshots;
3. only after routing is stable, proceed toward transaction execution hardening and idempotency expansion.



### 73. Milestone Update — Deterministic Routing Freshness Boundaries

**Date:** 2026-09-25

CI gate:

- CI #429 for commit `c00813b7a61460d78b5c700cb6fbb3405193ac58` is **GREEN**;
- `test`, `vet`, and `race` completed successfully.

Completed:

- routing freshness evaluation now uses a router-level clock hook, defaulting to `time.Now`;
- operational snapshot and catalog freshness decisions are deterministic when tests inject a fixed time;
- a freshness timestamp is valid only when it is non-zero and not in the future;
- the configured maximum age is inclusive at the exact boundary;
- future operational snapshots are rejected instead of being treated as fresh;
- future catalog snapshots are rejected instead of being treated as fresh;
- added deterministic regression coverage for exact max-age, future operational timestamps, and future catalog timestamps.

Safety boundary:

- the change only tightens routing eligibility semantics;
- it does not mutate lifecycle, health, capability, balance, catalog state, or financial ledger state;
- no retry/failover, funding, or automatic provider activation was introduced.

Known limitation:

- freshness remains an operational routing gate and does not establish a financial source of truth;
- system clock correctness remains an infrastructure requirement for timestamp-based eligibility.

Next milestone:

1. verify and stabilize the transaction execution boundary against the routing eligibility gate;
2. review purchase idempotency persistence semantics before any retry/failover behavior;
3. preserve provider-neutral transaction correlation and do not mutate financial ledger state from provider execution results.



### 74. Milestone Update — Pre-Submission Transaction Idempotency Boundary

**Date:** 2026-09-25

CI gate:

- CI #434 for commit `342f5daf43b14c2f1e00623726261e64a0af52d7` is **GREEN**;
- `test` and `race` completed successfully;
- `vet` completed successfully as part of the test job.

Completed:

- transaction execution now selects and validates the provider before external submission;
- the selected provider plus a neutral `pending` transaction state is persisted before calling the provider;
- if pending-state persistence fails, the external provider is not called;
- if provider result persistence fails after submission, the durable pending state is retained;
- provider errors no longer erase the durable pending boundary;
- restart recovery can therefore return the persisted pending transaction instead of silently resubmitting it;
- added regression tests for pre-submit persistence failure and result-persistence failure;
- existing concurrent duplicate protection and provider no-fallback behavior remain covered.

Safety boundary:

- this milestone strengthens idempotency and crash consistency only;
- no retry/failover was introduced;
- no automatic resubmission is performed after provider errors or restart;
- financial ledger mutation remains outside provider execution;
- reconciliation remains the explicit path for resolving durable pending transactions.

Known limitation:

- the current transaction store is still an interim in-memory/JSON persistence implementation;
- atomic database uniqueness/locking semantics are still required before multi-process production deployment;
- a pending state means provider submission outcome may remain unknown until webhook/status reconciliation.

Next milestone:

1. harden restart/reconciliation behavior for durable pending transactions;
2. add explicit persistence semantics for transaction state transitions and concurrent process safety;
3. only after those boundaries are stable evaluate controlled retry/failover policies.


### 75. Milestone Update — Restart/Reconciliation Persistence Transition Hardening

**Date:** 2026-09-25

CI gate:

- CI #452 for commit `21172bdb7b43210f50599d71900f646ea304d857` is **GREEN**;
- `test` and `race` completed successfully;
- `vet` completed successfully as part of the test job.

Completed:

- transaction persistence now enforces explicit provider-neutral state transitions;
- transaction request identity and selected provider identity cannot be mutated after persistence;
- only `pending` transactions may transition to terminal `success` or `failed`;
- terminal states are immutable except for idempotent re-writes of the identical terminal result;
- the same transition validation is enforced by both in-memory and JSON transaction stores;
- restart recovery and reconciliation are covered together: a durable pending transaction can be loaded by a fresh service instance and reconciled through provider status without resubmitting the purchase;
- the restart reconciliation test now models the external provider retaining the transaction state across a DesKaProvider process restart;
- the Mock provider gained deterministic test-only helpers for modeling a provider-side transaction status transition.

Safety boundary:

- no retry/failover or automatic resubmission was introduced;
- reconciliation calls provider status only and never invokes provider purchase;
- conflicting request/provider identity or terminal overwrite is rejected;
- provider transaction state remains separate from customer financial ledger state;
- no automatic funding, ledger mutation, or lifecycle mutation is triggered by reconciliation.

Known limitations:

- the JSON transaction store remains single-process locked and is not a multi-process transaction database;
- atomic database uniqueness/locking semantics are still required before multi-process production deployment;
- provider status reconciliation still depends on the concrete provider retaining and exposing the transaction reference;
- unresolved pending transactions remain explicitly pending when provider status cannot yet resolve them.

Next milestone:

1. harden concurrent reconciliation and durable transition behavior across process boundaries;
2. define the minimum database-backed transaction-store contract before production deployment;
3. only after the persistence boundary is stable evaluate controlled retry/failover policies with explicit idempotency guarantees.


### 76. Milestone Update — Concurrent Reconciliation & Atomic Transaction-Store Contract

**Date:** 2026-09-25

CI gate:

- CI #470 for commit `0bbb5eb5c13bae8ceb5af138ffcc94a353ebd809` is **GREEN**;
- `test` and `race` completed successfully;
- `vet` completed successfully as part of the test job.

Completed:

- introduced an optional `AtomicTransactionStore` boundary with `PutIfCurrent(referenceID, previous, next)`;
- memory and JSON transaction stores enforce compare-and-transition semantics under their existing single-process locks;
- reconciliation now uses the atomic transition boundary when the configured store supports it;
- concurrent reconciliation against the same provider result is idempotent: a stale transition detects the newer terminal state and returns the same terminal result instead of overwriting it;
- added deterministic concurrent reconciliation coverage with two service instances sharing the same transaction store;
- documented the minimum database-backed transaction-store contract in the architecture document.

Safety boundary:

- atomic transition protects transaction identity/state consistency but does not authorize provider resubmission;
- no retry/failover was introduced;
- reconciliation remains status-only and never calls provider purchase;
- provider transaction state remains separate from customer ledger state;
- stale concurrent transitions are rejected rather than silently overwriting newer state.

Known limitations:

- the current JSON transaction store still provides only single-process locking and does not provide cross-process file locking;
- the database-backed implementation and PostgreSQL schema/locking strategy are not implemented yet;
- the atomic store contract is therefore an architectural boundary, not yet a production multi-process persistence guarantee.

Next milestone:

1. define the concrete PostgreSQL transaction-store schema and conditional update/locking semantics;
2. add persistence-level failure/recovery tests around atomic transitions;
3. keep retry/failover deferred until database-backed idempotency and transaction correlation are production-ready.


### 77. Milestone Update — PostgreSQL Transaction-Store Schema & Conditional Transition Contract

**Date:** 2026-09-25

Completed:

- added `DesKaProvider/backend/migrations/001_provider_transactions.sql` as the concrete PostgreSQL transaction-store schema contract;
- defined durable transaction identity, original purchase request identity, selected provider identity, provider result fields, status constraint, optimistic-concurrency `version`, and persistence timestamps;
- defined the atomic compare-and-transition SQL shape using `reference_id`, `version`, request identity, provider identity, and `status = 'pending'` as the conditional boundary;
- documented zero-row conditional updates as stale-state/concurrency conflicts that must reload state and reconcile rather than resubmit a provider purchase;
- documented PostgreSQL persistence semantics and the relationship between the schema and the existing `AtomicTransactionStore` contract;
- retained existing deterministic persistence failure/recovery coverage for pending-before-submit, result-persistence failure, restart recovery, pending reconciliation, and concurrent reconciliation.

Verification scope:

- the repository currently has no PostgreSQL driver, migration runner, or live PostgreSQL test harness in `DesKaProvider/backend/go.mod`;
- therefore this milestone intentionally delivers the schema and conditional-update contract without adding an unverified database dependency or pretending that live PostgreSQL integration has been tested;
- existing in-memory/JSON tests remain the executable contract for transition and persistence-failure behavior until the database adapter is introduced.

Safety boundary:

- no provider retry/failover/resubmission was introduced;
- no customer-ledger mutation was introduced;
- no automatic treasury/provider funding was introduced;
- a stale database transition is a conflict, not a reason to call provider purchase again;
- terminal transaction state remains immutable except for idempotent identical observations.

Known limitations:

- PostgreSQL adapter/connection management is not implemented yet;
- migration execution is not wired into runtime startup;
- live PostgreSQL concurrency, rollback, and recovery tests remain pending;
- JSON transaction storage remains single-process and is not a production cross-process substitute.

Next milestone:

1. implement the PostgreSQL `AtomicTransactionStore` adapter behind the existing provider-neutral contract;
2. add isolated PostgreSQL integration tests for conditional transition, concurrent reconciliation, rollback, and restart recovery;
3. keep retry/failover deferred until the database-backed idempotency boundary is proven.


### 78. Milestone Update — PostgreSQL AtomicTransactionStore Adapter

**Date:** 2026-09-25

Completed:

- added DesKaProvider/backend/routing/postgres_transaction_store.go implementing the existing provider-neutral AtomicTransactionStore contract;
- added durable Get/All reconstruction paths and transition-safe Put behavior;
- implemented atomic PutIfCurrent using PostgreSQL conditional UPDATE semantics;
- added deterministic adapter tests for successful transition, zero-row concurrency conflict, and request-identity mismatch;
- kept the adapter database-driver neutral through the standard library database/sql boundary;
- did not introduce retry, failover, provider resubmission, ledger mutation, or automatic provider funding.

Verification:

- CI is required to pass test, vet, and race before this milestone is closed;
- live PostgreSQL integration is intentionally not claimed because the repository does not yet select a PostgreSQL driver or provide a live database test environment;
- unit coverage verifies the adapter's atomic transition contract with a deterministic database stub.

Known limitations:

- Store operations inherit the existing context-free TransactionStore interface and therefore currently use context.Background();
- no PostgreSQL driver, connection lifecycle, migration runner, or live integration harness is wired yet;
- PostgreSQL version state is represented by the database row and conditional predicate; TransactionState itself does not expose a version field;
- JSON persistence remains single-process and is not a cross-process production substitute.

Next milestone:

1. add an isolated PostgreSQL integration harness and migration verification;
2. verify rollback and concurrent transition behavior against a real PostgreSQL instance;
3. then evaluate explicit context propagation in the persistence contract;
4. keep retry/failover deferred until database-backed idempotency is proven.


#### Milestone #78 CI closure

The initial adapter implementation exposed two contract mismatches and was corrected before milestone closure:

- CI run #490 reported the adapter Get signature and transaction-status type mismatch; corrected in commit c43ad17c70fd6625ad3a43e3c098ab246c1ec94832.
- CI run #492 reported missing request-identity validation in PutIfCurrent; corrected in commit 98eb5139e4a51e15b419853806aa18c2f90d9f88.

Final verification for milestone #78:

- CI run #496 / 36136543507: **GREEN**;
- test: PASS;
- vet: PASS;
- race: PASS.

The milestone is therefore closed at the provider-neutral adapter/unit-test boundary. Live PostgreSQL integration remains explicitly pending.


### 79. Milestone Update — PostgreSQL Integration Harness & Real Concurrency Verification

**Date:** 2026-09-25

Implementation:

- added a real PostgreSQL integration test harness behind `DESKAPROVIDER_POSTGRES_DSN`;
- added `github.com/jackc/pgx/v5` as the database/sql PostgreSQL driver used only by integration tests;
- added migration loading/verification and isolated database setup in the routing integration test package;
- verified the concrete adapter against a real PostgreSQL instance in CI scope for pending persistence, reconstruction, concurrent `PutIfCurrent`, and reconnect/recovery;
- added concurrent integration coverage requiring exactly one successful terminal transition and one stale-state conflict;
- updated DesKaProvider CI to provision an isolated PostgreSQL 18 service for both test and race jobs;
- kept the integration harness credential-free with an ephemeral CI-only DSN.

Safety boundary:

- no provider retry/failover/resubmission was introduced;
- no customer-ledger mutation was introduced;
- no automatic provider funding was introduced;
- PostgreSQL concurrency conflicts remain state conflicts and never authorize provider purchase resubmission;
- the integration harness does not expose production credentials.

Verification gate:

- fresh CI for this milestone is mandatory;
- `test`, `vet`, and `race` must all be GREEN before milestone closure;
- the integration test is expected to exercise the real PostgreSQL service in CI, while local environments without `DESKAPROVIDER_POSTGRES_DSN` skip that integration case.

Known limitations:

- the persistence contract remains context-free and the adapter uses `context.Background()`;
- migration execution is still not wired into production runtime startup;
- the integration harness validates the current schema/adapter boundary but does not establish a production migration orchestration policy;
- retry/failover remains deferred until the full database-backed transaction/idempotency boundary is reviewed.

Next milestone:

1. verify the fresh PostgreSQL-backed CI gate;
2. if green, close the integration boundary and evaluate explicit context propagation in the persistence contract;
3. continue hardening restart/reconciliation semantics before considering any retry/failover policy.


#### Milestone #79 CI closure

Final verification for milestone #79:

- CI run #515 / 36137545123: **GREEN**;
- test: PASS;
- vet: PASS;
- race: PASS;
- PostgreSQL 18 service was provisioned in both test and race jobs;
- the real PostgreSQL integration test passed for migration application, durable transaction reconstruction, concurrent `PutIfCurrent`, and reconnect/recovery.

CI recovery during the milestone:

- CI #508 / 36137207631 initially failed on an integration-test string literal and the race job also lacked a committed `go.sum` after the dependency was introduced;
- CI #512 / 36137340637 exposed a second newline-literal defect in the migration test and the race job was hardened with an explicit `go mod tidy` step;
- CI #514 / 36137424736 exposed incorrect comment stripping in the migration runner, which was executing a fragment of a SQL comment;
- these defects were corrected before final closure; no production transaction behavior was changed by the test-harness fixes.

Milestone #79 is closed at the real-PostgreSQL integration boundary. The provider-neutral adapter is now exercised against PostgreSQL concurrency and reconnect behavior in CI, while production migration orchestration and explicit context propagation remain separate concerns.


### 80. Milestone Update — Context-Aware Persistence Boundary

**Date:** 2026-09-25

Completed:

- added optional ContextTransactionStore contract extending the existing TransactionStore boundary;
- added context-aware Get/Put/All/PutIfCurrent methods to the in-memory store;
- propagated request context through PostgreSQL database/sql operations;
- updated purchase, webhook, and reconciliation persistence paths to prefer context-aware storage when available;
- retained compatibility wrappers and fallback behavior for existing context-free stores;
- documented the boundary and cancellation semantics in the architecture document.

Safety boundary:

- context cancellation only controls the persistence operation lifecycle;
- no retry/failover/resubmission was introduced;
- no customer-ledger mutation or automatic provider funding was introduced;
- a cancelled persistence operation never authorizes another provider submission.

Verification gate:

- test, vet, and race must all be GREEN;
- existing PostgreSQL integration coverage remains the real database verification boundary.

Known limitations:

- startup recovery through Service construction still uses the context-free All() compatibility path;
- legacy stores that do not implement ContextTransactionStore retain their existing behavior;
- production migration orchestration remains separate from this persistence contract.

Next milestone:

1. verify CI GREEN for the context-aware contract;
2. add explicit cancellation/deadline tests at the persistence boundary;
3. continue restart/reconciliation hardening before retry/failover consideration.


#### Milestone #80 CI closure

Final verification:

- CI run #529 / 36138375689: **GREEN**;
- test: PASS;
- vet: PASS;
- race: PASS;
- PostgreSQL integration service remained active for the test/race jobs and the existing real-database coverage passed.

Milestone #80 is closed at the context-aware persistence contract boundary. Explicit cancellation/deadline assertions remain the next hardening step, rather than being assumed from the interface alone.

Next milestone #81:

1. add deterministic cancellation/deadline tests for context-aware memory and PostgreSQL persistence paths;
2. verify cancellation does not alter durable transaction state or authorize resubmission;
3. continue restart/reconciliation hardening.


### 81. Milestone Update — Context Cancellation & Deadline Persistence Hardening

**Date:** 2026-09-25

Completed:

- added deterministic memory-store tests for canceled request contexts across GetContext, AllContext, PutContext, and PutIfCurrentContext;
- added deterministic memory-store deadline coverage for mutation paths;
- verified canceled/deadline persistence operations do not mutate the durable in-memory transaction state;
- added PostgreSQL adapter coverage proving request cancellation reaches the database/sql execution boundary;
- retained the existing rule that cancellation/deadline errors never authorize provider retry, failover, or resubmission;
- kept the provider-neutral persistence contract unchanged; this milestone hardens the behavior of the existing ContextTransactionStore boundary rather than expanding financial scope.

Safety boundary:

- cancellation and deadlines only control the persistence operation lifecycle;
- a canceled or expired persistence call cannot transition transaction state;
- no provider retry/failover/resubmission was introduced;
- no customer-ledger mutation was introduced;
- no automatic provider funding was introduced.

Verification:

- CI run #533 / 36140004549: **GREEN**;
- test: PASS;
- vet: PASS;
- race: PASS;
- PostgreSQL service remained active for the test/race jobs.

Known limitations:

- GetContext and AllContext retain their existing state-only return shape, so cancellation/database errors on those read methods can still be represented as not-found/empty results;
- startup recovery still uses the context-free All() compatibility path;
- production migration orchestration remains separate from the persistence contract.

Next milestone:

1. continue restart/reconciliation hardening with explicit read-error observability where the current context-aware interface is insufficient;
2. evaluate whether the persistence contract should expose read errors without breaking provider-neutral callers;
3. keep retry/failover deferred until transaction persistence and reconciliation semantics are fully hardened.


### 82. Milestone Update — Context-Aware Read Error Observability

**Date:** 2026-09-25

Completed:

- added optional ContextReadTransactionStore extension with error-aware GetContextE and AllContextE methods;
- preserved the existing TransactionStore and ContextTransactionStore contracts for compatibility;
- implemented error-aware reads in MemoryTransactionStore and PostgresTransactionStore;
- retained compatibility wrappers that preserve the previous state-only read behavior for legacy callers;
- updated reconciliation conflict reload to surface context/database read errors instead of silently treating them as not-found;
- added deterministic cancellation and database-read-error tests at the persistence boundary.

Safety boundary:

- no provider retry/failover/resubmission was introduced;
- a read error during reconciliation conflict handling does not authorize another provider purchase;
- no customer-ledger mutation was introduced;
- no automatic provider funding was introduced;
- the new interface is additive and does not alter provider-specific contracts.

Verification:

- CI run #547 / 36140897373: **GREEN**;
- CI run #546 / 36140894595: **GREEN**;
- CI run #545 / 36140891050: **GREEN**;
- test: PASS;
- vet: PASS;
- race: PASS;
- PostgreSQL service remained active for the test/race jobs.

Known limitations:

- GetContext and AllContext remain compatibility methods with their original state-only signatures;
- Service startup recovery still uses the context-free All() path because startup lifecycle context is not yet part of the constructor contract;
- Postgres GetContextE deterministic unit coverage is limited by the database/sql Row API; AllContextE provides direct read-error coverage.

Next milestone:

1. harden startup/restart recovery with explicit initialization context and read-error propagation;
2. evaluate constructor/API changes needed to make startup database failures distinguishable from an empty transaction set;
3. keep retry/failover deferred until startup/reconciliation recovery remains unambiguous.


### 83. Milestone Update — Context-Aware Startup Recovery Boundary

**Date:** 2026-09-25

Completed:

- added NewServiceWithStoreContext for explicit initialization context;
- startup transaction reconstruction now prefers ContextReadTransactionStore.AllContextE when available;
- startup database/read failures are surfaced as initialization errors instead of being interpreted as an empty transaction store;
- canceled initialization contexts are rejected before transaction reconstruction;
- retained compatibility behavior for existing context-free and legacy stores;
- added deterministic startup read-error and canceled-initialization tests.

Safety boundary:

- startup recovery failure never authorizes provider retry/failover/resubmission;
- no customer-ledger mutation was introduced;
- no automatic provider funding was introduced;
- existing NewServiceWithStore remains compatible and uses context.Background as its explicit legacy initialization context;
- provider-specific contracts remain unchanged.

Verification:

- CI test/vet/race must be GREEN before milestone closure;
- PostgreSQL integration remains the real database verification boundary.

Known limitations:

- legacy TransactionStore implementations still expose only context-free startup reads;
- production application startup must explicitly choose NewServiceWithStoreContext to receive initialization cancellation/error semantics;
- migration orchestration remains separate from service construction.

Next milestone:

1. wire explicit initialization context into the production-facing startup composition boundary;
2. add PostgreSQL integration coverage for startup read failure and cancellation behavior;
3. keep retry/failover deferred until startup and reconciliation recovery are unambiguous.


### 84. Milestone Update — Production Startup Context Wiring

**Date:** 2026-09-25

Completed:

- added NewFromEnvironmentContext to the runtime composition boundary;
- retained NewFromEnvironment as a compatibility wrapper using context.Background();
- changed the production command to create its signal-aware lifecycle context before runtime construction;
- passed that initialization context into NewServiceWithStoreContext so transaction-store startup reads participate in process lifecycle cancellation;
- added runtime tests for canceled and missing initialization contexts;
- corrected the historical Milestone #82 CI verification references to the actual green runs.

Safety boundary:

- startup cancellation stops initialization before transaction reconstruction and cannot authorize provider retry/failover/resubmission;
- no provider-specific contract changed;
- no customer-ledger mutation or automatic provider funding was introduced;
- the existing JSON transaction store remains the current runtime persistence selection.

Verification:

- CI test/vet/race must be GREEN before milestone closure;
- PostgreSQL integration remains green from the existing persistence harness.

Known limitations:

- runtime composition still selects the JSON transaction store; PostgreSQL startup-read integration is not claimed until a production PostgreSQL store selection/configuration boundary exists;
- migration execution remains separate from runtime startup;
- legacy callers using NewFromEnvironment retain background-context compatibility semantics.

Next milestone:

1. define the production PostgreSQL transaction-store selection boundary without leaking database details into routing;
2. add PostgreSQL-backed runtime startup failure/cancellation integration coverage through that composition boundary;
3. keep retry/failover deferred until production persistence startup and reconciliation remain unambiguous.


### 85. Milestone Update — Production PostgreSQL Transaction-Store Selection

**Date:** 2026-09-25

Completed:

- added explicit DESKAPROVIDER_TRANSACTION_STORE_DRIVER selection with json as the compatibility default;
- added DESKAPROVIDER_POSTGRES_DSN configuration required only when the PostgreSQL transaction store is selected;
- isolated transaction-store construction behind the runtime composition helper openTransactionStore;
- PostgreSQL runtime selection uses the existing provider-neutral PostgresTransactionStore adapter and database/sql;
- production startup pings the selected PostgreSQL database with the initialization context before service construction continues;
- the production Service retains the database handle and closes it during shutdown;
- added deterministic configuration/cancellation tests and a PostgreSQL runtime composition integration test using the existing migration artifact.

Safety boundary:

- selecting PostgreSQL changes persistence only; it does not alter provider routing or provider-specific contracts;
- database startup failure stops service construction and never authorizes provider retry/failover/resubmission;
- no customer-ledger mutation or automatic provider funding was introduced;
- JSON remains the default to preserve existing deployments;
- no credentials are logged or embedded in source.

Verification:

- CI test/vet/race must be GREEN before milestone closure;
- when DESKAPROVIDER_POSTGRES_DSN is present, the runtime composition integration test exercises the real PostgreSQL service.

Known limitations:

- migration execution remains a separate deployment concern; the runtime integration test applies the repository migration only inside its isolated test database;
- PostgreSQL is opt-in and is not yet the default production persistence backend;
- provider state, operational snapshots, and catalog persistence remain on their existing stores.

Next milestone:

1. harden PostgreSQL runtime restart recovery with durable pending-state reconstruction through the production composition path;
2. add integration coverage for startup recovery of pending transactions and clean reconstruction after reconnect;
3. keep retry/failover deferred until production restart/reconciliation behavior is fully verified.


#### Milestone #85 CI closure

Final verification after isolating the runtime PostgreSQL integration test schema:

- CI run #585 / 36143519374: **GREEN**;
- CI run #584 / 36143515119: **GREEN**;
- test: PASS;
- vet: PASS;
- race: PASS;
- PostgreSQL runtime composition integration passed against the CI PostgreSQL service.

CI recovery during Milestone #85:

- CI #581/#580 exposed a shared PostgreSQL test-schema collision between the existing routing integration harness and the new runtime integration test;
- CI #583/#582 exposed an invalid schema identifier containing a dot from nanosecond timestamp formatting;
- the runtime integration test was changed to create an isolated schema with a PostgreSQL-safe numeric suffix and a single connection pool for deterministic search_path usage;
- no production persistence behavior was changed by these test-harness fixes.

Milestone #85 is closed at the production PostgreSQL transaction-store selection and runtime composition boundary.


### 86. Milestone Update — PostgreSQL Restart Recovery & Reconciliation Verification

**Date:** 2026-09-25

Completed:

- added a real PostgreSQL integration scenario that creates a pending transaction through Service.Purchase using PostgresTransactionStore;
- verified the pending state is durable before process/service reconstruction;
- reconstructed a new routing service from the same PostgreSQL transaction store using NewServiceWithStoreContext;
- reconciled the recovered pending transaction through the provider status path;
- verified reconciliation does not submit the provider purchase a second time;
- verified provider identity and pending transaction state remain stable after reconstruction/reconciliation;
- isolated the integration scenario in its own PostgreSQL schema so it cannot interfere with other database tests.

Safety boundary:

- restart recovery only reads durable state and reconciles through provider status;
- recovery never calls the provider purchase operation;
- no retry/failover/resubmission policy was introduced;
- no customer-ledger mutation or automatic provider funding was introduced;
- the provider-neutral TransactionStore/Service boundary remains unchanged.

Verification:

- CI test/vet/race must be GREEN before milestone closure;
- real PostgreSQL integration verifies durable pending recovery and no-resubmission behavior.

Known limitations:

- this milestone verifies service reconstruction and reconciliation against a live PostgreSQL store, but does not simulate a physical process kill while a real network provider request is in flight;
- PostgreSQL migration execution remains a separate deployment concern;
- provider-specific crash/retry semantics remain outside the verified contract.

Next milestone:

1. verify terminal-state recovery across PostgreSQL reconnect/restart boundaries;
2. add explicit recovery tests for persisted success/failed transactions and stale transition rejection after restart;
3. keep retry/failover deferred.


### 87. Milestone Update — PostgreSQL Terminal-State Recovery & Stale Transition Hardening

**Date:** 2026-09-25

Completed:

- extended the real PostgreSQL integration coverage to verify persisted terminal success state across service reconstruction;
- reconstructed a new routing service from the same PostgreSQL transaction store and reconciled the already-terminal transaction;
- verified terminal reconciliation is idempotent and does not submit the provider purchase again;
- verified the durable terminal result remains unchanged after restart/reconciliation;
- verified an identical terminal compare-and-transition is accepted idempotently;
- verified a stale/old pending transition is rejected with ErrTransactionStateConflict after the PostgreSQL row has already become terminal;
- verified a terminal mutation to a different result is rejected with ErrReferenceConflict.

Safety boundary:

- terminal recovery never authorizes provider purchase resubmission;
- stale PostgreSQL state conflicts are treated as state conflicts, not retry/failover signals;
- no retry/failover/resubmission policy was introduced;
- no customer-ledger mutation or automatic provider funding was introduced;
- provider-neutral transaction identity and selected provider identity remain immutable.

Verification:

- the integration test uses an isolated PostgreSQL schema and the real PostgresTransactionStore;
- the terminal recovery path verifies success persistence, reconstruction, reconciliation idempotency, and provider purchase count remains exactly one;
- stale transition behavior is verified against the real PostgreSQL conditional-update boundary;
- fresh CI test, vet, race, and PostgreSQL integration must all be GREEN before milestone closure.

Known limitations:

- this milestone covers terminal success recovery directly; equivalent failed persistence remains covered by the provider-neutral transition contract but does not yet have a separate end-to-end PostgreSQL restart scenario;
- the test still does not simulate a physical process kill while an external provider request is in flight;
- PostgreSQL migration execution remains a separate deployment concern;
- provider-specific crash/retry semantics remain outside the verified contract.

Next milestone:

1. add explicit PostgreSQL restart/reconnect coverage for terminal failed recovery;
2. verify terminal-state behavior through webhook/reconciliation convergence where applicable;
3. keep retry/failover deferred until the full transaction lifecycle remains unambiguous.

### 88. Milestone Update — PostgreSQL Failed-State Recovery & Webhook Convergence

**Date:** 2026-09-25

Completed:

- added real PostgreSQL integration coverage for a persisted terminal failed transaction;
- reconstructed a new routing Service from the same PostgreSQL store and reconciled the failed terminal state;
- verified failed-terminal reconciliation is idempotent and does not submit the provider purchase again;
- verified an identical terminal failed webhook after restart converges idempotently without changing the durable result;
- verified provider purchase count remains exactly one across purchase, restart, reconciliation, and terminal webhook handling;
- verified the failed terminal result remains durable after the restart/reconciliation/webhook sequence.

Safety boundary:

- terminal failed recovery never authorizes provider resubmission;
- identical terminal webhook events are convergence signals, not retry signals;
- no retry/failover/resubmission policy was introduced;
- no customer-ledger mutation or automatic provider funding was introduced;
- provider-neutral transaction identity and selected provider identity remain immutable.

Verification:

- the integration test uses an isolated PostgreSQL schema and the real PostgresTransactionStore;
- the failed recovery path verifies durable failed persistence, reconstruction, reconciliation idempotency, and terminal webhook idempotency;
- fresh CI test, vet, race, and PostgreSQL integration must all be GREEN before milestone closure.

Known limitations:

- the integration scenario verifies an already terminal failed transaction rather than a physical process kill during an in-flight external request;
- webhook convergence is verified for an identical terminal event; conflicting terminal webhook payloads remain rejected by the existing reference-conflict boundary;
- PostgreSQL migration execution remains a separate deployment concern;
- provider-specific crash/retry semantics remain outside the verified contract.

Next milestone:

1. extend PostgreSQL integration coverage to conflicting terminal webhook/reconciliation observations;
2. verify that terminal conflicts remain non-resubmitting across restart;
3. keep retry/failover deferred.


### 89. Milestone Update — PostgreSQL Terminal Conflict Recovery After Restart

**Date:** 2026-09-25

Completed:

- added a real PostgreSQL integration scenario that creates a terminal SUCCESS transaction through Service.Purchase and reconstructs the routing Service from the same PostgreSQL transaction store;
- changed the mock provider's observed transaction status after restart to model a conflicting provider-side terminal observation;
- verified reconciliation rejects the conflicting terminal observation with ErrWebhookReferenceConflict;
- verified a conflicting terminal webhook for the same reference is also rejected with ErrWebhookReferenceConflict;
- verified both conflict paths leave the durable terminal SUCCESS result unchanged;
- verified provider PurchaseCount remains exactly one across the original purchase, service reconstruction, reconciliation conflict, and webhook conflict;
- kept the conflict boundary non-resubmitting and provider-neutral.

Safety boundary:

- conflicting terminal observations are treated as state/reference conflicts, never as retry/failover signals;
- no provider purchase resubmission was introduced;
- no customer-ledger mutation or automatic provider funding was introduced;
- terminal provider identity and transaction identity remain immutable;
- retry/failover/resubmission remain explicitly deferred.

Verification:

- CI run #612 / 36147378874: **GREEN**;
- test: PASS;
- vet: PASS;
- race: PASS;
- PostgreSQL integration service remained active and the real PostgreSQL integration coverage passed.

Known limitations:

- the scenario models a conflicting provider observation after service reconstruction using the deterministic mock provider; it does not simulate a real external provider changing a terminal transaction after an actual process crash;
- migration execution remains a separate deployment concern;
- provider-specific conflict/retry semantics remain outside the verified provider-neutral contract.

Next milestone:

1. define the next persistence/reconciliation hardening boundary from the current architecture and operational requirements;
2. preserve terminal conflict non-resubmission guarantees while expanding only where a concrete provider-neutral safety gap is identified;
3. keep retry/failover deferred until transaction lifecycle semantics are fully verified.


### 90. Milestone Update — Transaction Audit Persistence Boundary

**Date:** 2026-09-26

Completed:

- defined a provider-neutral append-only TransactionAuditStore contract separate from financial TransactionState;
- added deterministic MemoryTransactionAuditStore coverage for append-only behavior and incomplete-event rejection;
- added PostgreSQL provider_transaction_audit schema with immutable append-only intent and reference/time index;
- extended the real PostgreSQL migration verification to require the audit table and index fragments;
- kept audit persistence separate from transaction-state mutation so audit concerns cannot authorize provider retry, failover, or resubmission.

Safety boundary:

- audit records are append-only operational history, not financial ledger postings;
- audit persistence does not mutate TransactionState, customer ledger state, provider balance, or treasury state;
- no retry/failover/resubmission policy was introduced;
- no automatic provider funding was introduced;
- provider-specific credentials and protocol details are not stored by this boundary.

Verification:

- CI run #630 / 36167152247: **GREEN**;
- test: PASS;
- vet: PASS;
- race: PASS;
- PostgreSQL integration service active and migration verification passed.

Known limitations:

- this milestone defines and verifies the persistence boundary; Service lifecycle events are not yet wired to emit audit records;
- PostgreSQL audit adapter/runtime selection is deferred until the event emission contract is finalized;
- audit records are operational history and are not a substitute for the financial ledger or immutable financial postings.

Next milestone:

1. wire transaction lifecycle, webhook, reconciliation, and terminal-conflict events into the audit boundary;
2. verify audit failures cannot trigger provider resubmission and do not mutate financial state;
3. add PostgreSQL audit adapter integration only after the event semantics are stable.


### 91. Milestone Update — Transaction Lifecycle Audit Event Wiring

**Date:** 2026-09-26

Completed:

- wired the provider-neutral TransactionAuditStore into Service lifecycle paths;
- added audit observations for durable purchase pending state, provider result, provider error/pending recovery, result-persistence failure, webhook pending/terminal transitions, reconciliation transitions, and terminal conflict observations;
- added additive service constructors for explicit audit-store injection while preserving existing constructors;
- kept MemoryTransactionAuditStore as the default deterministic/local audit boundary;
- verified audit failure behavior on the purchase path: durable terminal transaction state remains unchanged and provider PurchaseCount remains exactly one;
- verified repeated Purchase after a terminal audit failure returns the existing durable result/error without resubmitting the provider;
- verified lifecycle audit events and terminal webhook conflict audit events;
- verified reconciliation transition audit event and no-resubmission behavior.

Safety boundary:

- audit records are operational history only, never financial ledger postings;
- audit persistence cannot authorize provider retry, failover, or resubmission;
- audit failure cannot revert a durable terminal transaction to pending;
- no customer-ledger mutation was introduced;
- no provider balance or treasury mutation was introduced;
- no automatic provider funding was introduced;
- provider-specific credentials and protocol details remain outside the audit boundary.

Verification:

- CI run #646 / 36170321352 initially **FAILED** because the new reconciliation test used the wrong Mock.SetTransactionStatus signature;
- CI run #648 / 36170498148 initially **FAILED** because the test referenced a non-existent provider.TransactionStatusSuccess constant;
- both failures were test-only issues; production transaction/audit behavior was not changed by those fixes;
- CI run #650 / 36170688487: **GREEN**;
- test: PASS;
- vet: PASS;
- race: PASS;
- PostgreSQL integration service active and the full suite passed.

Known limitations:

- audit persistence is still in-memory by default;
- PostgreSQL audit adapter and production runtime selection are not implemented yet;
- audit failure behavior is explicitly tested on the purchase path; webhook/reconciliation paths implement the same commit-before-audit safety ordering but require dedicated failure-injection coverage in a later hardening step;
- audit history remains separate from the financial ledger and cannot be used as financial source of truth.

Next milestone:

1. implement the PostgreSQL TransactionAuditStore adapter against provider_transaction_audit;
2. add PostgreSQL integration coverage for append-only audit durability and restart/reconnect reads;
3. add failure-injection coverage for webhook/reconciliation audit writes before selecting PostgreSQL audit persistence at runtime;
4. keep retry/failover/resubmission deferred.


### 92. Milestone Update — PostgreSQL Transaction Audit Store Adapter

**Date:** 2026-09-26

Completed:

- added provider-neutral PostgreSQL TransactionAuditStore adapter;
- added append-only INSERT boundary and reference-scoped ordered reads;
- propagated context cancellation/deadline errors through context-aware audit persistence;
- added deterministic validation/cancellation tests;
- added real PostgreSQL integration coverage for append-only durability and restart/reconnect reads;
- verified audit data remains separate from TransactionState and does not authorize provider operations;
- verified canceled audit append does not mutate an already durable terminal transaction or authorize provider resubmission.

Safety boundary:

- audit rows are append-only operational history;
- no UPDATE/DELETE audit API was introduced;
- no retry/failover/resubmission behavior was introduced;
- audit failure cannot revert durable transaction state;
- no customer-ledger, provider-balance, treasury, or automatic-funding mutation was introduced.

Verification:

- PostgreSQL integration verifies two durable audit events survive adapter reconstruction;
- ordering is deterministic by created_at, audit_id;
- unrelated references return no events;
- canceled audit append is rejected before database mutation;
- full CI verification is required before milestone closure.

Known limitations:

- PostgreSQL audit adapter is not yet selected by production runtime;
- webhook/reconciliation audit failure injection needs dedicated coverage before runtime selection;
- audit remains separate from the financial ledger.

Next milestone:

1. add dedicated webhook audit-failure injection coverage;
2. add dedicated reconciliation audit-failure injection coverage;
3. verify committed transaction state remains authoritative in both paths;
4. only then wire PostgreSQL audit-store selection into production runtime.


### 93. Milestone Update — Webhook & Reconciliation Audit Failure Injection Hardening

**Date:** 2026-09-26

Completed:

- added dedicated webhook audit-failure injection coverage after a terminal webhook transition is durably committed;
- added dedicated reconciliation audit-failure injection coverage after a terminal reconciliation transition is durably committed;
- verified the committed terminal TransactionState remains authoritative when the audit append fails;
- verified audit failure does not revert terminal state to pending;
- verified audit failure does not authorize provider resubmission;
- verified provider PurchaseCount remains exactly one through the injected webhook/reconciliation audit failure scenarios;
- verified subsequent identical webhook/reconciliation observations converge idempotently from the committed terminal state.

Safety boundary:

- transaction persistence remains the authorization boundary; audit persistence is observational;
- webhook/reconciliation audit failure cannot trigger retry, failover, or provider resubmission;
- durable terminal state is never rolled back because operational audit storage is unavailable;
- no customer-ledger mutation, provider-balance mutation, treasury mutation, or automatic funding was introduced.

Verification:

- dedicated deterministic failure-injection tests cover both webhook and reconciliation paths;
- latest CI run #676 / 36174112280: **GREEN**;
- test: PASS;
- vet: PASS;
- race: PASS;
- PostgreSQL integration service active and the full suite passed.

Known limitations:

- failure injection is deterministic at the TransactionAuditStore boundary; it does not simulate PostgreSQL process failure or network partition during an audit INSERT;
- production runtime selection of PostgreSQL audit storage remains deferred to the next milestone;
- audit history remains operational evidence and is not a financial ledger or source of truth.

Next milestone:

1. wire PostgreSQL TransactionAuditStore selection into production runtime configuration;
2. verify startup construction and shutdown lifecycle with PostgreSQL transaction and audit stores together;
3. add runtime integration coverage proving durable audit reads survive service reconstruction;
4. keep retry/failover/resubmission deferred.


### Milestone #94 — Production PostgreSQL Audit-Store Runtime Selection

**Date:** 2026-09-26

Completed:

- added runtime configuration `DESKAPROVIDER_AUDIT_STORE_DRIVER=memory|postgres`;
- retained the default audit store as in-memory for the existing local/interim runtime boundary;
- PostgreSQL audit selection requires `DESKAPROVIDER_POSTGRES_DSN` and uses the existing PostgreSQL transaction connection when the transaction store is also PostgreSQL;
- when only the audit store uses PostgreSQL, runtime opens and validates a dedicated PostgreSQL connection using the same configured DSN;
- production service construction now injects the selected `TransactionAuditStore` into the routing service;
- PostgreSQL audit connection lifecycle is closed during runtime shutdown;
- added runtime configuration tests for default, PostgreSQL, invalid, and missing-DSN audit-store settings;
- added a real PostgreSQL runtime integration test that applies the migration, appends an audit event, and reconstructs it through the selected audit adapter.

### Verification

- Latest pre-milestone CI baseline: CI #680 / run `36175136617` for HEAD `052ae776f7aa332fb7825980a7c10b89cb7c9d9b` was GREEN before Milestone #94 changes.
- Milestone #94 changes require a new CI run on the resulting HEAD; this milestone is not closed until that latest HEAD is GREEN.
- No automatic retry, failover, resubmission, ledger mutation, or provider funding was introduced.

### Safety Boundary

- Audit-store selection is operational persistence only.
- Audit failure remains non-authoritative relative to committed transaction state.
- PostgreSQL audit persistence does not authorize provider resubmission.
- A single PostgreSQL DSN may back both transaction and audit stores, but the audit table remains append-only and separate from transaction state.

### Known Limitations

- Audit-store runtime selection currently supports memory and PostgreSQL only.
- Migration deployment remains an operational prerequisite; runtime does not execute schema migrations automatically.
- The runtime integration test verifies durable audit persistence and adapter selection, not external provider crash/network failure semantics.

### Next Milestone

**#95 — Runtime Durable Audit Reconstruction & Shutdown Integration Hardening**: verify a production-style service restart with PostgreSQL transaction and audit stores, including durable audit history reconstruction and clean shutdown/error-path behavior.


### Milestone #95 — Runtime Durable Audit Reconstruction & Shutdown Integration Hardening

**Date:** 2026-09-26

Completed:

- added real PostgreSQL integration coverage that constructs the transaction and audit stores through the production service constructor;
- executed a successful purchase and verified exactly one provider submission;
- verified durable audit history contains the expected `PURCHASE_PENDING` and `PURCHASE_RESULT` events before restart;
- closed the original PostgreSQL connection and reopened a fresh connection using the configured runtime DSN;
- reconstructed both PostgreSQL stores and a new routing service instance after restart;
- verified terminal reconciliation returns the same durable purchase result without provider resubmission;
- verified audit history remains unchanged after restart and idempotent reconciliation;
- verified an identical terminal webhook after restart is idempotent and does not resubmit the provider.

### Verification

- CI #708 / run `36183435145`: **GREEN**;
- test: PASS;
- vet: PASS;
- race: PASS;
- PostgreSQL integration service active and the restart/reconstruction test passed.

### Safety Boundary

- PostgreSQL transaction state remains the authoritative transaction persistence boundary;
- audit history is durable operational evidence, not a financial ledger;
- restart reconciliation never resubmits an already terminal provider transaction;
- no retry, failover, provider resubmission, customer-ledger mutation, treasury mutation, or automatic provider funding was introduced.

### Known Limitations

- the integration models restart by closing/reopening the database connection and reconstructing service objects; it does not kill a production process during an in-flight network request;
- migration deployment remains an operational prerequisite and is not executed automatically by runtime startup;
- provider-specific crash/retry semantics remain outside the provider-neutral contract.

### Next Milestone

**#96 — PostgreSQL Audit/Transaction Lifecycle Shutdown Failure Hardening**: verify initialization and shutdown error paths when PostgreSQL transaction and audit stores are shared or separately opened, while preserving the rule that audit persistence cannot authorize financial or provider state transitions.

## Milestone #96 — PostgreSQL Audit/Transaction Lifecycle Shutdown Failure Hardening

**Date:** 2026-09-26

Milestone #96 hardens runtime ownership of PostgreSQL transaction and audit database handles during service initialization failure and shutdown.

### Completed

- runtime initialization now installs a deferred database cleanup guard immediately after transaction/audit persistence resources are opened;
- initialization failures after database acquisition therefore close resources before returning, including failures in provider-state persistence, provider-state reconstruction, routing construction, and service construction;
- shared PostgreSQL transaction/audit usage is closed once through the transaction database owner;
- audit-only PostgreSQL usage remains owned by and closed through its dedicated database handle;
- dedicated integration coverage verifies both shared and dedicated PostgreSQL handles are actually closed;
- normal shutdown continues to close runtime-owned PostgreSQL resources without changing transaction authorization semantics.

### Verification

CI #716 / run `36185635330` is GREEN: test, vet, race, and PostgreSQL integration service passed.

### Safety boundary

Database cleanup is an infrastructure lifecycle concern only. It does not introduce retry, failover, provider resubmission, ledger mutation, treasury movement, or automatic provider funding. Audit persistence remains observational and cannot authorize provider actions.

### Limitation

The integration test verifies database-handle closure rather than simulating an operating-system process kill or a network partition during shutdown. PostgreSQL migration deployment remains outside runtime startup.

### Next milestone

**Milestone #97:** runtime shutdown error propagation and ownership observability hardening, with explicit verification that resource-close failures cannot be confused with transaction/provider outcomes.

## Milestone #97 — Runtime Shutdown Error Propagation & Ownership Observability Hardening

**Date:** 2026-09-26

Milestone #97 hardens runtime shutdown error semantics so database-close failures are observable without being confused with transaction/provider outcomes.

### Completed

- runtime database ownership now uses a close-error-aware boundary;
- transaction and audit PostgreSQL close failures are surfaced from Service.Run;
- shared transaction/audit handles are still closed only once;
- when shutdown has a primary lifecycle error such as context cancellation and database close also fails, both errors remain discoverable with errors.Is;
- when no database close error exists, the original primary error identity is preserved exactly for compatibility;
- deterministic tests cover propagation of both transaction/audit close errors and the no-double-close shared-handle rule.

### Verification

CI #726 / run `36186740152` is GREEN: test, vet, race, and PostgreSQL integration service passed.

### Safety boundary

Shutdown errors are lifecycle/observability signals only. They do not authorize transaction retry, provider resubmission, failover, ledger mutation, treasury movement, or provider funding. A database close error cannot be interpreted as a provider transaction result.

### Limitation

The test boundary uses deterministic close-error injection; it does not claim to reproduce every operating-system, driver, or network failure mode during database shutdown.

### Next milestone

**Milestone #98:** runtime initialization cleanup error observability and explicit ownership diagnostics, while preserving the primary initialization failure and financial safety boundaries.


## 35. Runtime Initialization Cleanup Error Observability & Explicit Ownership Diagnostics

**Date:** 2026-09-26

Milestone #98 hardens the failure path after PostgreSQL transaction/audit resources have been acquired but before runtime service ownership is successfully transferred.

### Verified behavior

- initialization failures after database acquisition now close already-owned transaction/audit database handles;
- cleanup failures are no longer silently discarded;
- the original initialization failure remains discoverable through errors.Is;
- cleanup failures are surfaced as supplemental lifecycle infrastructure errors;
- shared transaction/audit handles remain closed exactly once;
- successful initialization transfers database ownership to the runtime Service, so the initialization cleanup guard does not close live resources;
- deterministic tests cover cleanup-error observability, primary-error preservation, and shared-handle de-duplication.

CI run #736 / 36187759170 is **GREEN**: test, vet, race, and PostgreSQL-backed integration coverage passed.

### Safety boundary

Initialization cleanup errors are infrastructure lifecycle signals only. They do not authorize provider retry, failover, resubmission, transaction-state mutation, customer-ledger mutation, treasury movement, or provider funding.

The primary initialization error remains the semantic reason startup failed; cleanup failure is supplemental evidence that must remain observable for operators.

### Known limitations

- cleanup error injection is deterministic at the databaseCloser abstraction rather than a forced real PostgreSQL Close() failure;
- the runtime still does not expose a dedicated structured ownership diagnostic object or metrics stream; current observability is through returned errors and deterministic tests;
- migration deployment remains outside runtime startup.

### Next milestone

Milestone #99: runtime ownership transfer and initialization/shutdown lifecycle contract consolidation, including explicit success-path ownership tests and regression coverage across shared versus dedicated PostgreSQL resources.

### Milestone #99 — Runtime Ownership Transfer & Initialization/Shutdown Lifecycle Contract Consolidation

**Date:** 2026-09-26

### Completed

- introduced an explicit runtime database-ownership boundary covering transaction and audit PostgreSQL handles acquired during initialization;
- made ownership transfer from the initialization guard to the runtime Service explicit on the successful construction path;
- verified the initialization cleanup guard does not close transferred resources after successful initialization;
- changed Service shutdown to close runtime-owned database resources through the ownership boundary;
- made shutdown close idempotent so a repeated shutdown path does not double-close a shared or dedicated handle and preserves the first close result;
- added deterministic success-path tests for transaction + audit resources together, shared transaction/audit handles, and dedicated PostgreSQL audit handles;
- added real PostgreSQL integration regression coverage that verifies transferred shared and dedicated handles remain usable before shutdown and are closed after ownership-owned shutdown;
- kept primary initialization error preservation and shutdown error composition unchanged;
- no provider routing, retry, failover, resubmission, financial ledger, treasury, or provider funding semantics were changed.

### Verification

- CI run #744 / run 36190106679: **GREEN** for HEAD ed456f5914124c908e0c2ac3f86d7061e1fd8784;
- `go test ./...`: PASS in CI, including the PostgreSQL integration suite;
- `go vet ./...`: PASS in CI;
- `go test -race ./...`: PASS in CI;
- PostgreSQL service active in CI and shared/dedicated ownership integration coverage passed;
- local execution was not available because the environment could not clone the GitHub repository, so verification is based on the repository's GitHub Actions run for the exact pushed HEAD.

### Safety Boundary

- transaction persistence remains the authoritative transaction state;
- audit persistence remains operational evidence only;
- database cleanup and shutdown errors remain infrastructure lifecycle signals, not provider transaction results;
- audit failure cannot authorize retry, failover, provider resubmission, ledger mutation, treasury movement, or provider funding;
- shared PostgreSQL transaction/audit usage is closed exactly once by the runtime owner;
- dedicated PostgreSQL audit usage is closed by the runtime owner exactly once;
- failed initialization still cleans up resources already acquired before ownership transfer.

### Known Limitations

- ownership observability remains internal to the runtime ownership boundary; there is still no separate metrics stream or structured external diagnostic endpoint;
- integration coverage verifies handle lifecycle and PostgreSQL usability, not process-kill or network-partition behavior during shutdown;
- PostgreSQL migration execution remains a deployment prerequisite and is not run automatically at startup;
- provider-specific crash/retry semantics remain outside this provider-neutral lifecycle contract.

### Next Milestone

**#100 — Runtime Lifecycle Integration Consolidation**: continue only with a concrete lifecycle gap identified from the current runtime architecture; preserve the established ownership, terminal-state, audit-observational, and non-resubmission boundaries.
### Milestone #100 — Runtime Lifecycle Integration Consolidation

**Date:** 2026-09-26

### Completed

- promoted Service.Close() as the explicit runtime shutdown boundary for owned database resources;
- consolidated Service.Run() shutdown handling so it delegates resource release to Service.Close() rather than duplicating database-close behavior;
- preserved idempotent ownership semantics: repeated Service.Close() calls do not double-close shared or dedicated resources;
- preserved close-error observability across repeated shutdown calls;
- added deterministic tests for Service.Close() idempotence and close-error preservation;
- added real PostgreSQL integration coverage proving Service.Close() closes shared transaction/audit ownership exactly once and closes dedicated audit ownership correctly;
- kept transaction authorization, audit observational semantics, provider routing, retry/failover/resubmission, ledger, treasury, and provider funding boundaries unchanged.

### Verification

- CI #750 / run 36190896225: **GREEN** for exact HEAD 36eb85dba859d1b5df5a5d0d2fbd34f007a2ef45;
- go test ./...: PASS;
- go vet ./...: PASS;
- go test -race ./...: PASS;
- PostgreSQL integration service active and Service.Close() shared/dedicated lifecycle coverage passed;
- CI #749 / run 36190888005 for the same HEAD also completed successfully.

### Safety Boundary

- Service.Close() is an infrastructure lifecycle operation only;
- database close errors are not provider transaction results and cannot authorize retry, failover, resubmission, ledger mutation, treasury movement, or provider funding;
- transaction persistence remains the authoritative transaction state;
- audit persistence remains operational evidence only;
- shared PostgreSQL transaction/audit ownership remains single-close;
- dedicated PostgreSQL audit ownership remains independently owned and single-close.

### Known Limitations

- the runtime does not expose a separate lifecycle metrics stream or structured external shutdown diagnostic API;
- integration coverage verifies database-handle behavior, not process-kill or network-partition behavior during shutdown;
- PostgreSQL migration deployment remains an external operational prerequisite;
- broader orchestration across future runtime workers remains outside this milestone.

### Next Milestone

**#101 — Runtime Shutdown/Startup Integration Boundary Review**: inspect remaining runtime lifecycle edges around future workers and externally initiated shutdown, and only add behavior where a concrete lifecycle gap is demonstrated.


### Milestone #101 — Runtime Balance Worker Lifecycle Integration

**Date:** 2026-09-26

### Completed

- `Service` now owns an explicit `operational.SyncWorkerLifecycle` for balance synchronization;
- runtime startup constructs the lifecycle and `Run()` starts it once through the lifecycle boundary;
- the balance worker now follows the original service context instead of being started and stopped immediately;
- normal service cancellation waits for the owned balance worker to stop before completing runtime shutdown;
- `Service.Close()` remains the database ownership shutdown boundary after worker shutdown;
- preserved the existing fallback behavior for manually constructed services that do not provide the lifecycle object;
- deterministic runtime coverage proves the balance worker performs its immediate synchronization and that runtime cancellation returns `context.Canceled` after worker shutdown;
- fixed the regression found by CI where the first lifecycle implementation returned `nil` instead of propagating service context cancellation;
- no provider retry, failover, resubmission, transaction-state, audit-authority, customer-ledger, treasury, or automatic provider-funding behavior was changed.

### Verification

- CI #763 / run `36194276130`: **GREEN** for exact HEAD `3450c8692302aedb124b27fc74daef2ac93e350d`;
- CI #764 / run `36194278985`: **GREEN** for the same exact HEAD;
- `go test ./...`: PASS;
- `go vet ./...`: PASS;
- `go test -race ./...`: PASS;
- PostgreSQL-backed integration suite remained green in the workflow.

### Safety Boundary

- transaction persistence remains the authoritative transaction state;
- audit persistence remains operational evidence only;
- worker lifecycle and database shutdown are infrastructure concerns and cannot authorize retry, failover, provider resubmission, ledger mutation, treasury movement, or provider funding;
- database ownership transfer and single-close semantics remain unchanged.

### Known Limitations

- the runtime still manages catalog synchronization directly through its ticker rather than a dedicated lifecycle object;
- lifecycle observability is internal; there is no external lifecycle metrics/diagnostic endpoint;
- process-kill and network-partition behavior remain outside this deterministic lifecycle test boundary.

### Next Milestone

**#102 — Runtime Catalog Worker Lifecycle Boundary Review**: inspect the remaining catalog synchronization lifecycle path and only introduce a dedicated lifecycle abstraction where it improves concrete startup/shutdown ownership without changing business semantics.


### Milestone #102 — Runtime Catalog Worker Lifecycle Boundary

**Date:** 2026-09-26

### Completed

- introduced an explicit runtime-owned catalog worker lifecycle boundary;
- catalog synchronization keeps its existing synchronous `SyncAll(ctx)` semantics and ticker cadence;
- the lifecycle owns the derived catalog context and provides idempotent shutdown state transitions;
- duplicate catalog lifecycle start is rejected deterministically;
- runtime catalog synchronization now runs against the lifecycle-owned context and is canceled during service shutdown;
- added deterministic tests for lifecycle start/duplicate-start/shutdown/restart behavior;
- added runtime integration coverage proving the catalog snapshot is populated and the runtime exits with `context.Canceled` after service cancellation;
- no provider retry, failover, resubmission, transaction-state, audit-authority, customer-ledger, treasury, or automatic provider-funding behavior was changed.

### Verification

- CI #771 / run `36195305696`: **GREEN** for exact HEAD `81a3e3b0a8482e1179fcff738462660f3d01fd2a`;
- CI #772 / run `36195307870`: **GREEN** for the same exact HEAD;
- `go test ./...`: PASS;
- `go vet ./...`: PASS;
- `go test -race ./...`: PASS;
- PostgreSQL-backed workflow service completed successfully.

### Safety Boundary

- catalog lifecycle is an infrastructure orchestration concern only;
- transaction persistence remains authoritative and audit persistence remains observational;
- catalog worker shutdown cannot authorize provider retry, failover, resubmission, ledger mutation, treasury movement, or provider funding;
- existing database ownership transfer and single-close semantics remain unchanged.

### Known Limitations

- catalog lifecycle currently owns context/state only; `SyncAll` remains a synchronous call driven by the runtime ticker;
- there is no external lifecycle metrics or diagnostic endpoint;
- process-kill and network-partition behavior remain outside the deterministic lifecycle test boundary.

### Next Milestone

**#103 — Runtime Startup/Shutdown Composition Review**: review the combined balance and catalog lifecycle ordering, including failure and repeated-shutdown paths, without expanding financial or provider action semantics.


### Milestone #103 — Runtime Startup/Shutdown Composition Review

**Date:** 2026-09-26

### Completed

- consolidated the runtime shutdown path for the combined balance and catalog lifecycle composition;
- explicit shutdown ordering now stops the catalog lifecycle first, then waits for the balance worker lifecycle, then closes runtime-owned database resources;
- catalog lifecycle shutdown is idempotent and remains separate from database ownership shutdown;
- when catalog lifecycle startup fails, the balance worker is shut down before database ownership is closed;
- deterministic regression coverage verifies combined balance+catalog runtime shutdown returns `context.Canceled` and database ownership is closed exactly once;
- repeated `Service.Close()` remains idempotent after the composed worker shutdown path;
- no provider retry, failover, resubmission, transaction-state, audit-authority, customer-ledger, treasury, or automatic provider-funding behavior was changed.

### Verification

- CI #777 / run `36195985232`: **GREEN** for exact HEAD `be43aac84d393824d4c99cc52c59a44f1273fb7c`;
- CI #778 / run `36195989681`: **GREEN** for the same exact HEAD;
- `go test ./...`: PASS;
- `go vet ./...`: PASS;
- `go test -race ./...`: PASS;
- PostgreSQL-backed workflow service completed successfully.

### Safety Boundary

- lifecycle composition is an infrastructure shutdown concern only;
- transaction persistence remains the authoritative transaction state and audit persistence remains operational evidence;
- worker startup/shutdown failures cannot authorize provider retry, failover, resubmission, ledger mutation, treasury movement, or provider funding;
- existing single-close database ownership semantics remain unchanged.

### Known Limitations

- lifecycle composition is internal to the runtime and has no external metrics/diagnostic endpoint;
- the catalog synchronization operation itself remains synchronous rather than a separately goroutine-owned worker;
- process-kill and network-partition failure modes remain outside the deterministic test boundary.

### Next Milestone

**#104 — Runtime Initialization Failure Composition Review**: verify startup failures across balance lifecycle construction, catalog lifecycle construction, provider-state initialization, and database ownership cleanup without changing financial or provider action semantics.

### Milestone #104 — Runtime Initialization Failure Composition Review

**Date:** 2026-09-26

### Completed

- reviewed startup failure composition across runtime configuration, provider-state initialization, store construction, routing/service construction, and lifecycle construction boundaries;
- added deterministic coverage for provider-state initialization failure after runtime database ownership has been established;
- fixed a real cleanup-boundary bug where typed-nil database handles stored in the databaseCloser interface could reach (*sql.DB).Close() and panic during initialization cleanup;
- restored the existing transaction/audit store opening helpers after the cleanup fix and revalidated the complete runtime initialization path;
- added a regression test proving typed-nil database handles are ignored safely by runtime cleanup;
- preserved the existing ownership transfer rule: cleanup runs before ownership transfer and service shutdown owns resources only after successful handoff;
- preserved primary initialization error semantics; cleanup remains an infrastructure lifecycle concern;
- no provider retry, failover, resubmission, transaction-state, audit-authority, customer-ledger, treasury, or automatic provider-funding behavior was changed.

### Verification

- CI #789 / run 36197733274: **GREEN** for exact HEAD 52b59d4dc5f2eb07e821711762c38d976e27b02e;
- CI #790 / run 36197737431: **GREEN** for the same exact HEAD;
- go test ./...: PASS;
- go vet ./...: PASS;
- go test -race ./...: PASS;
- PostgreSQL-backed workflow service completed successfully.

### Safety Boundary

- initialization and database cleanup remain infrastructure lifecycle concerns only;
- transaction persistence remains the authoritative transaction state;
- audit persistence remains operational evidence only;
- cleanup failures cannot authorize provider retry, failover, resubmission, ledger mutation, treasury movement, or provider funding;
- typed-nil cleanup protection prevents infrastructure cleanup from panicking and does not alter financial/provider action semantics.

### Known Limitations

- the deterministic failure-injection boundary does not reproduce every operating-system or driver-specific shutdown failure;
- lifecycle observability remains internal and there is no separate structured diagnostic stream;
- process-kill and network-partition behavior remain outside the deterministic runtime test boundary.

### Next Milestone

**#105 — Runtime Lifecycle Construction & Context Propagation Review**: inspect the remaining constructor-time lifecycle failure paths and context propagation guarantees, especially balance lifecycle construction, catalog lifecycle creation, and cancellation boundaries, without changing business or financial semantics.


### Milestone #106 — Runtime Start Failure & Partial Lifecycle Rollback Review

**Date:** 2026-09-26

### Completed

- reviewed the runtime startup sequence where the balance lifecycle starts before catalog lifecycle startup is attempted;
- added an explicit rollback helper for the partial-start failure path;
- when catalog lifecycle startup fails, the already-started balance lifecycle is stopped before runtime-owned database resources are closed;
- retained the existing composed shutdown path and idempotent lifecycle/database cleanup semantics;
- added deterministic failure-injection coverage for the catalog lifecycle start boundary and the partial-start rollback path;
- preserved primary startup failure semantics while treating lifecycle/database cleanup as infrastructure concerns;
- no provider retry, failover, resubmission, transaction-state, audit-authority, customer-ledger, treasury, or automatic provider-funding behavior was changed.

### Verification

- implementation HEAD: `5c32b04db8a39cfcaa7419fb3aa332b7454471ea`;
- CI #817 / run `36199199241`: **GREEN** for exact HEAD `5c32b04db8a39cfcaa7419fb3aa332b7454471ea`;
- CI #818 / run `36199202092`: **GREEN** for the same exact HEAD;
- `go test ./...`: PASS in CI;
- `go vet ./...`: PASS in CI;
- `go test -race ./...`: PASS in CI;
- PostgreSQL-backed workflow service completed successfully.

### Safety Boundary

- partial lifecycle rollback is an infrastructure startup/shutdown concern only;
- transaction persistence remains the authoritative transaction state;
- audit persistence remains operational evidence only;
- rollback and database cleanup cannot authorize provider retry, failover, resubmission, ledger mutation, treasury movement, or provider funding;
- existing single-close database ownership semantics remain unchanged.

### Known Limitations

- deterministic failure injection covers the catalog lifecycle start boundary, not every possible runtime startup instruction or external I/O failure;
- lifecycle observability remains internal and there is no separate structured diagnostic stream;
- process termination, driver-specific failure timing, and network partitions remain outside the deterministic runtime test boundary.

### Next Milestone

**#107 — Runtime Shutdown Error Composition & Close-Order Review**: review cleanup-error propagation across worker shutdown, catalog shutdown, and database ownership closure, preserving the distinction between infrastructure errors and provider/financial outcomes.


### Milestone #107 — Runtime Shutdown Error Composition & Close-Order Review

**Date:** 2026-09-26

### Completed

- reviewed the runtime cleanup composition after lifecycle shutdown and during Service.Close();
- confirmed the runtime combines primary lifecycle errors with database-close errors using errors.Join, preserving discoverability of each underlying error;
- added deterministic coverage proving a primary runtime error, worker/cleanup error, and database close error can remain independently discoverable when composed;
- added deterministic coverage proving runtime database ownership preserves the original close error across repeated shutdown calls without double-closing the resource;
- retained the existing close order: started worker lifecycles are shut down before runtime-owned database resources are closed;
- retained the existing shared-handle single-close rule for transaction/audit PostgreSQL resources;
- no provider retry, failover, resubmission, transaction-state, audit-authority, customer-ledger, treasury, or automatic provider-funding behavior was changed.

### Verification

- implementation HEAD: `819066ee2b55d22d9f99ff08cf8190951834ea59`;
- CI #821 / run `36200425590`: **GREEN** for exact HEAD `819066ee2b55d22d9f99ff08cf8190951834ea59`;
- `go test ./...`: PASS in CI;
- `go vet ./...`: PASS in CI;
- `go test -race ./...`: PASS in CI;
- PostgreSQL-backed workflow service completed successfully.

### Safety Boundary

- shutdown error composition is an infrastructure observability concern only;
- transaction persistence remains the authoritative transaction state;
- audit persistence remains operational evidence only;
- infrastructure cleanup errors cannot authorize provider retry, failover, resubmission, ledger mutation, treasury movement, or provider funding;
- existing single-close database ownership semantics remain unchanged.

### Known Limitations

- deterministic close-error injection does not reproduce every operating-system, driver, or network failure mode during shutdown;
- lifecycle observability remains internal with no separate structured diagnostic stream;
- process termination and network-partition behavior remain outside the deterministic runtime test boundary.

### Next Milestone

**#108 — Runtime Lifecycle Idempotency & Re-entry Review**: verify repeated Run/Close/lifecycle start-stop behavior and ensure runtime re-entry cannot create duplicate workers or duplicate shutdown side effects.


### Milestone #108 — Runtime Lifecycle Idempotency & Re-entry Review

**Date:** 2026-09-26

### Completed

- verified the owned balance worker lifecycle rejects duplicate Start() attempts deterministically while preserving mutex-protected lifecycle state;
- verified repeated lifecycle shutdown remains idempotent and the runtime can safely reuse the lifecycle after a completed shutdown;
- added runtime coverage for concurrent Service.Run() re-entry while the balance lifecycle is active;
- fixed a concrete re-entry ownership gap: when a second Service.Run() is rejected with operational.ErrSyncWorkerRunning, that rejected invocation no longer calls Service.Close() and therefore cannot close runtime-owned database resources belonging to the active Run() instance;
- added regression coverage proving the rejected concurrent Run() leaves the active database ownership open and the active Run() closes it exactly once after cancellation;
- preserved repeated Service.Close() idempotence and close-error preservation;
- kept provider retry, failover, resubmission, transaction-state, audit-authority, customer-ledger, treasury, and automatic provider-funding boundaries unchanged.

### Verification

- implementation HEAD: 1dff617374503c5900aea8ae148bda412ca443ca;
- CI #837 / run 36201789474: **GREEN** for exact HEAD 1dff617374503c5900aea8ae148bda412ca443ca;
- CI jobs test: success;
- CI jobs race: success;
- the prior CI #833 / run 36201434715 for f7eeb357e3ddc98b11ffe7fcc440243210c14ef0 also completed successfully before the re-entry ownership fix;
- the final #108 verification is therefore based on the newer exact HEAD and its green test/race run.

### Safety Boundary

- concurrent runtime re-entry is an infrastructure lifecycle concern only;
- rejecting a duplicate Run() does not trigger database cleanup owned by the already-running instance;
- active runtime ownership remains responsible for its own worker shutdown and database closure;
- transaction persistence remains the authoritative transaction state and audit persistence remains operational evidence;
- re-entry guards cannot authorize provider retry, failover, resubmission, ledger mutation, treasury movement, or provider funding.

### Known Limitations

- the re-entry boundary is deterministic and covers the owned balance lifecycle path; future independently owned workers may require their own coordination rules;
- lifecycle observability remains internal with no separate structured diagnostic stream;
- process termination and network-partition behavior remain outside the deterministic test boundary.

### Next Milestone

**#109 — Runtime Context Cancellation & Shutdown Boundary Review**: inspect the remaining cancellation/shutdown edges after lifecycle idempotency is established, with emphasis on context ownership and shutdown timeout behavior, without changing business or financial semantics.


### Milestone #109 — Runtime Context Cancellation & Shutdown Boundary Review

**Date:** 2026-09-26

### Completed

- reviewed the relationship between the parent service context and runtime-owned worker shutdown context;
- preserved the existing rule that shutdown cleanup must not inherit an already-canceled parent context as an immediate cancellation signal;
- added a runtime shutdown-context helper that isolates shutdown cancellation from the parent while preserving an existing parent deadline;
- updated the balance-worker shutdown path to use the derived shutdown context instead of an unbounded Background() context;
- added deterministic coverage proving a parent deadline is preserved by the shutdown context;
- added deterministic coverage proving an already-canceled parent does not cause immediate shutdown-context cancellation;
- retained the existing worker shutdown ordering and single-close database ownership semantics;
- kept provider retry, failover, resubmission, transaction-state, audit-authority, customer-ledger, treasury, and automatic provider-funding boundaries unchanged.

### Verification

- implementation HEAD: `709dfef165d61479ac2693bf292a153f444356c7`;
- CI #843 / run `36202103586`: **GREEN** for exact HEAD `709dfef165d61479ac2693bf292a153f444356c7`;
- CI jobs `test`: success;
- CI jobs `race`: success.

### Safety Boundary

- context propagation and worker shutdown remain infrastructure lifecycle concerns only;
- the shutdown context can enforce an existing runtime deadline without changing transaction or provider outcome semantics;
- an already-canceled service context does not become an immediate cleanup cancellation signal for the owned worker;
- transaction persistence remains the authoritative transaction state and audit persistence remains operational evidence;
- shutdown timeout handling cannot authorize provider retry, failover, resubmission, ledger mutation, treasury movement, or provider funding.

### Known Limitations

- the new helper preserves a parent deadline when one exists, but it does not invent a default shutdown timeout when the parent has no deadline;
- deterministic coverage exercises context cancellation boundaries but not process termination or driver-specific hangs during database close;
- independently owned future workers may require explicit deadline propagation rules of their own.

### Next Milestone

**#110 — Runtime Worker Error & Shutdown Timeout Composition Review**: inspect how worker-returned errors interact with shutdown deadlines and database-close errors, without changing business or financial semantics.


### Milestone #110 — Runtime Worker Error & Shutdown Timeout Composition Review

**Date:** 2026-09-26

### Completed

- reviewed how worker shutdown errors interact with the runtime shutdown deadline and database ownership closure;
- verified that `SyncWorkerLifecycle.Shutdown()` returns the caller-provided cancellation/deadline error when the worker has not exited yet;
- preserved lifecycle ownership while shutdown is incomplete: a canceled/expired shutdown context does not mark the worker as stopped and duplicate `Start()` remains rejected;
- added deterministic regression coverage proving lifecycle ownership is retained across an interrupted shutdown and that a later shutdown can complete cleanly;
- retained the runtime rule that worker shutdown is coordinated before runtime-owned database resources are closed;
- preserved error composition behavior for primary runtime errors, worker shutdown errors, and database close errors;
- no provider retry, failover, resubmission, transaction-state, audit-authority, customer-ledger, treasury, or automatic provider-funding behavior was changed.

### Verification

- implementation HEAD: `6dbd6711b59bdf397d537828c2b7c219432e9516`;
- CI #847 / run `36202432766`: **GREEN** for exact HEAD `6dbd6711b59bdf397d537828c2b7c219432e9516`;
- CI jobs `test`: success;
- CI jobs `race`: success.

### Safety Boundary

- shutdown timeout and worker ownership remain infrastructure lifecycle concerns only;
- an incomplete worker shutdown does not release lifecycle ownership or authorize another worker instance to start;
- database ownership remains with the runtime owner until its coordinated shutdown path reaches the database-close stage;
- transaction persistence remains the authoritative transaction state and audit persistence remains operational evidence;
- worker errors and timeout errors cannot authorize provider retry, failover, resubmission, ledger mutation, treasury movement, or provider funding.

### Known Limitations

- the regression covers lifecycle shutdown timeout/cancellation behavior but does not simulate a permanently hung worker or process termination;
- database-close timeout policy is still outside the current ownership interface because `databaseCloser.Close()` has no context-aware form;
- independently owned future workers may require their own timeout and ownership coordination rules.

### Next Milestone

**#111 — Runtime Close/Shutdown Concurrency Review**: inspect concurrent `Service.Close()` calls and interactions between explicit close and active `Run()` shutdown, preserving single-owner cleanup and avoiding premature resource release.


### Milestone #111 — Runtime Close/Shutdown Concurrency Review

**Date:** 2026-09-26

### Completed

- reviewed concurrent `Service.Close()` calls and the interaction between explicit close and an active `Service.Run()` shutdown;
- prevented explicit `Service.Close()` from releasing runtime-owned database resources while the balance worker lifecycle is still active;
- preserved the existing ownership rule that the active runtime shutdown path stops the worker before closing owned database resources;
- retained idempotent database ownership closure after the active `Run()` has completed;
- added deterministic regression coverage proving an explicit close attempt during active runtime execution does not close the database prematurely and the active `Run()` still performs the single final close;
- kept re-entry protection and shutdown error composition unchanged;
- no provider retry, failover, resubmission, transaction-state, audit-authority, customer-ledger, treasury, or automatic provider-funding behavior was changed.

### Verification

- implementation HEAD: `c9c2d562ba60ecebc594e887329658dda9f39e9f`;
- CI #853 / run `36202743252`: **GREEN** for exact HEAD `c9c2d562ba60ecebc594e887329658dda9f39e9f`;
- CI jobs `test`: success;
- CI jobs `race`: success.

### Safety Boundary

- explicit close during active runtime execution is an infrastructure lifecycle concern only;
- active worker ownership remains responsible for coordinated worker shutdown before database release;
- a rejected explicit close cannot alter transaction authorization or provider action semantics;
- transaction persistence remains the authoritative transaction state and audit persistence remains operational evidence;
- lifecycle concurrency guards cannot authorize provider retry, failover, resubmission, ledger mutation, treasury movement, or provider funding.

### Known Limitations

- the explicit `Service.Close()` guard covers the owned balance-worker lifecycle path; independently owned future workers may require additional coordination;
- `databaseCloser.Close()` remains non-context-aware, so a driver-level hang is outside this contract;
- concurrency coverage is deterministic and does not simulate process termination or arbitrary OS-level resource failures.

### Next Milestone

**#112 — Runtime Lifecycle Restart & Post-Shutdown Re-entry Review**: verify restart behavior after a completed `Run()`/shutdown cycle, including worker state reset, database ownership state, and repeated service execution without stale lifecycle state.


### Milestone #112 — Runtime Lifecycle Restart & Post-Shutdown Re-entry Review

**Date:** 2026-09-26

### Completed

- verified that the same `Service` instance can restart its owned balance worker after a completed `Run()` shutdown without leaving the worker lifecycle in a stale running state;
- added deterministic regression coverage that executes the same `Service` instance through two complete Run/shutdown cycles;
- verified the lifecycle resets to a stopped state after each completed shutdown and accepts the next worker start;
- preserved the database ownership boundary: a completed runtime shutdown closes owned database resources once, and a later worker restart does not reopen or reuse a closed database ownership handle;
- retained the explicit-close protection introduced in #111 and the duplicate-run guard introduced in #108;
- no provider retry, failover, resubmission, transaction-state, audit-authority, customer-ledger, treasury, or automatic provider-funding behavior was changed.

### Verification

- implementation HEAD: `05ae537bd94d4a755b9d417ddd516c88489ba21f`;
- CI #857 / run `36203053997`: **GREEN** for exact HEAD `05ae537bd94d4a755b9d417ddd516c88489ba21f`;
- CI jobs `test`: success;
- CI jobs `race`: success.

### Safety Boundary

- worker lifecycle restart is an infrastructure orchestration concern only;
- restarting a worker lifecycle does not imply reopening database ownership or changing transaction authority;
- closed runtime-owned database handles remain closed and are not silently recreated by `Run()` re-entry;
- transaction persistence remains the authoritative transaction state and audit persistence remains operational evidence;
- lifecycle restart cannot authorize provider retry, failover, resubmission, ledger mutation, treasury movement, or provider funding.

### Known Limitations

- restart coverage verifies the balance worker lifecycle; a complete process-level runtime restart still constructs a new `Service` and acquires fresh ownership as part of initialization;
- `databaseCloser.Close()` remains non-context-aware;
- process termination, driver-specific failure timing, and network partitions remain outside the deterministic restart test boundary.

### Next Milestone

**#113 — Runtime Construction/Ownership Freshness Review**: verify that a newly constructed runtime instance acquires fresh database ownership and lifecycle state after a prior instance has completed shutdown, without reusing closed handles or stale worker state.


### Milestone #113 — Runtime Construction/Ownership Freshness Review

**Date:** 2026-09-26

### Completed

- reviewed runtime construction after prior service shutdown to ensure each new `Service` instance receives fresh lifecycle and ownership objects;
- verified that `NewFromEnvironmentContext()` constructs a new database ownership boundary for each successfully initialized runtime instance;
- verified that each runtime instance receives fresh balance and catalog lifecycle objects rather than reusing stale worker state;
- added deterministic regression coverage comparing two independently constructed runtime instances and asserting distinct database ownership and lifecycle identities;
- preserved the existing rule that closed database ownership is not silently reopened by `Run()` re-entry on an existing service instance;
- retained provider-state persistence across runtime reconstruction while keeping worker and ownership state instance-local;
- no provider retry, failover, resubmission, transaction-state, audit-authority, customer-ledger, treasury, or automatic provider-funding behavior was changed.

### Verification

- implementation HEAD: `0ae6ed2fb7aef97b8681d1f2de50a98cccedd148`;
- CI #861 / run `36203388454`: **GREEN** for exact HEAD `0ae6ed2fb7aef97b8681d1f2de50a98cccedd148`;
- CI jobs `test`: success;
- CI jobs `race`: success.

### Safety Boundary

- fresh runtime construction is an infrastructure ownership concern only;
- a new `Service` instance acquires new lifecycle/ownership state and does not inherit closed worker ownership from a previous instance;
- persisted provider operational state may survive reconstruction, but transaction authority and runtime resource ownership remain distinct;
- transaction persistence remains the authoritative transaction state and audit persistence remains operational evidence;
- construction freshness cannot authorize provider retry, failover, resubmission, ledger mutation, treasury movement, or provider funding.

### Known Limitations

- deterministic coverage validates object-level freshness; PostgreSQL integration still depends on the workflow's available service configuration and does not simulate process-kill recovery;
- the constructor may reuse persisted operational/provider state files by design, while database handles and lifecycle objects remain instance-local;
- `databaseCloser.Close()` remains non-context-aware.

### Next Milestone

**#114 — Runtime State Persistence & Lifecycle Boundary Review**: inspect the separation between persisted provider operational state and ephemeral runtime lifecycle state, ensuring restart persistence does not accidentally restore active worker/ownership state.


### 36. Milestone Update — Runtime State Persistence & Lifecycle Boundary Review

**Date:** 2026-09-26

Completed:

- reviewed the persisted provider state model and runtime lifecycle ownership boundary;
- confirmed persisted provider state contains provider lifecycle/capability state only, not active worker ownership or database ownership handles;
- added deterministic runtime regression coverage proving an enabled provider remains enabled across restart while the new balance worker starts stopped;
- verified each restarted runtime receives fresh database ownership and fresh worker lifecycle objects;
- verified closing the first runtime instance does not close the second runtime instance's database ownership.

### Verification

- latest HEAD before this milestone: `b6e7208ff941f7c4b5e25fcd1bb6b4f70202a99e`;
- previous CI run #863 (`36203499574`): **success**;
- milestone test commit: `4988f7885309996abc36ac03872d11108dc33aa4`;
- the new commit must complete the repository CI `test` and `race` jobs successfully before this milestone is considered closed.

### Next milestone

1. continue reviewing restart/shutdown boundaries around persisted operational snapshots and provider routing state;
2. keep transaction persistence authoritative for transaction state while operational/audit persistence remains non-authoritative evidence;
3. preserve the existing no-resubmission/no-financial-authorization expansion invariants.

### 37. Milestone Update — Persisted Operational Snapshot Freshness Boundary

**Date:** 2026-09-26

Completed:

- reviewed restart recovery semantics for persisted operational balance/health snapshots and routing state;
- added deterministic routing regression coverage proving an expired persisted operational snapshot remains available as recovery data but cannot be used for a new route decision;
- verified a freshly refreshed healthy snapshot restores route eligibility without changing provider lifecycle state semantics;
- preserved the distinction between persisted operational evidence and live runtime worker state.

### Verification

- milestone implementation commit: `1af606dc5ff0b47c6bd4756910fef7fdfb9340d2`;
- prior HEAD CI #866/#867 for milestone #114: **success**;
- CI for this new HEAD must finish with both `test` and `race` jobs **success** before milestone #115 is considered closed.

### Next milestone

1. continue the restart boundary review across routing, transaction persistence, and audit evidence;
2. ensure persisted operational snapshots never authorize transaction resubmission, failover, or financial mutation by themselves.

### 38. Milestone Update — Transaction Authority Across Restart & Audit Failure

**Date:** 2026-09-26

Completed:

- reviewed restart recovery of durable transaction state together with the audit-evidence boundary;
- added deterministic regression coverage proving a restarted service reconciles a durable pending transaction without resubmitting the provider purchase;
- verified an audit failure after successful reconciliation does not revert the durable terminal transaction state;
- verified a repeated purchase request after restart returns the committed transaction result and does not trigger a second provider submission;
- preserved transaction persistence as the authoritative state and audit persistence as operational evidence only.

### Verification

- milestone test commit: `8b05e02300ddcc0428776ddf97ebb77cf62a9dc9`;
- prior HEAD CI #870 for milestone #115: **success**;
- CI #1110 / run `36221224850`: **success** for exact validation HEAD `aadf692eeb51f16e810628579e28176ab6f9ede7`;
- CI jobs `test`: success;
- CI jobs `race`: success;
- CI job `vet`: success.

### Next milestone

1. continue auditing restart/reconciliation concurrency and atomic transition boundaries;
2. preserve the invariant that audit or operational evidence cannot authorize retry, failover, resubmission, ledger mutation, treasury movement, or provider funding.


### 39. Milestone Update — Atomic Reconciliation Concurrency Boundary

**Date:** 2026-09-26

Completed:

- repaired the PostgreSQL integration-test scope so each recovery/concurrency test owns its transaction store deterministically;
- added deterministic concurrent service reconciliation coverage proving two independent services converge on the same terminal provider result without resubmitting the provider purchase;
- preserved atomic transaction-state conflict handling as the database authority when concurrent reconciliation observes the same pending transaction;
- corrected the PR CI checkout path so pull-request validation runs against the actual PR head SHA rather than a stale synthetic merge ref;
- verified the repository CI executes the current branch state without changing provider retry, failover, resubmission, ledger, treasury, funding, or audit-authority semantics.

### Verification

- latest implementation HEAD: `7e6ea2deddc493fa24f56f4b232417a630f7271e`;
- CI #1090 / run `36219715105`: **GREEN**;
- CI jobs `test`: success;
- CI jobs `race`: success.

### Safety Boundary

- transaction persistence remains the authoritative source for terminal transaction state;
- concurrent reconciliation may converge on an already-committed provider result but cannot authorize a second provider submission;
- audit and operational evidence remain non-authoritative and cannot authorize retry, failover, resubmission, ledger mutation, treasury movement, or provider funding.

### Next Milestone

**#117 — Persistent Transaction Version Consistency Review**: audit sequential and concurrent PostgreSQL transaction updates so the persisted version used by non-conditional writes remains aligned with the database version after pending-state updates and restart recovery.


### 39. Milestone Update — Atomic PostgreSQL Transition & Concurrent Reconciliation Validation
Date 2026-09-26
Completed:
- validated the PostgreSQL compare-and-transition path against concurrent callers using the persisted transaction snapshot (including version and request/provider identity);
- fixed integration-test scoping and PostgreSQL session setup so the atomic transition test remains deterministic under connection pooling;
- verified concurrent terminalization produces one committed transition and one stale-state conflict, with the terminal state remaining durable;
- verified concurrent reconciliation converges without resubmitting the provider purchase;
- corrected CI PR checkout behavior to validate the actual PR head SHA rather than a stale synthetic merge ref.
Verification:
- HEAD 5ad67a2c88150fa4b8dbeab3e526ce86c5f9ca3a
- CI #954 / run 36210990858: test = success, race = success, vet = success
Next milestone:
1. continue auditing sequential PostgreSQL transaction version transitions and restart recovery boundaries;
2. preserve the invariant that audit or operational evidence cannot authorize retry, failover, resubmission, ledger mutation, treasury movement, or provider funding.

### 47. Milestone Update — Sequential PostgreSQL Transaction Version Transition Boundary

**Date:** 2026-09-26

Completed:

- reviewed the PostgreSQL transaction store version transition path for sequential pending-state updates and terminalization;
- added deterministic integration coverage proving version progression from initial version 1 to version 2 on a pending transition and version 3 on terminal success;
- verified a stale pre-terminal snapshot cannot be reused after terminalization because the optimistic version boundary rejects the stale write;
- confirmed production `PutContext` already uses the persisted `current.Version` for conditional updates, so no production behavior change was required;
- preserved transaction persistence as the authoritative transaction state and kept operational/audit evidence non-authoritative.

### Verification

- test hardening commit: `b67780b03418d0df58f6e2512ff58cc3cf3ffd46`;
- follow-up compile-fix commit: `088b9433fed28b03c9a9a492fe8dfb7c2a161452`;
- CI #932 / run `36209808482`: **GREEN** for exact HEAD `088b9433fed28b03c9a9a492fe8dfb7c2a161452`;
- CI jobs `test`: success;
- CI jobs `race`: success;
- CI `test` job `vet`: success.

### Safety Boundary

- sequential version advancement protects transaction persistence consistency only;
- optimistic version conflicts cannot authorize provider retry, failover, resubmission, ledger mutation, treasury movement, or provider funding;
- terminal transaction state remains authoritative across restart and stale-writer attempts.

### Next Milestone

1. continue auditing restart recovery with independently constructed service instances and durable version boundaries;
2. preserve the invariant that audit or operational evidence cannot authorize retry, failover, resubmission, ledger mutation, treasury movement, or provider funding.

### 48. Milestone Update — Stale Restart Instance Terminalization Boundary

**Date:** 2026-09-26

Completed:

- reviewed restart recovery with independently constructed service instances sharing the same durable transaction store;
- added deterministic regression coverage where a fresh instance commits a terminal success and a stale instance later observes a divergent provider terminal result;
- verified the stale instance returns the existing transaction-reference conflict rather than overwriting the durable terminal success;
- verified stale reconciliation does not resubmit the provider purchase;
- preserved transaction persistence as the authoritative state across restart and concurrent/stale writers;
- kept operational and audit evidence non-authoritative.

### Verification

- regression test commit: `65e8aa84bc99bc7b285e7a5b78be1af95c58f3cb`;
- CI #936 / run `36209987655`: **GREEN** for exact HEAD `65e8aa84bc99bc7b285e7a5b78be1af95c58f3cb`;
- CI jobs `test`: success;
- CI job `test` `vet`: success;
- CI jobs `race`: success.

### Safety Boundary

- stale-instance conflict handling protects durable transaction integrity only;
- a divergent stale terminal result cannot authorize overwrite, retry, failover, resubmission, ledger mutation, treasury movement, or provider funding;
- provider submission count remains unchanged during stale-instance reconciliation.

### Next Milestone

1. continue auditing startup/restart reads with database errors and context cancellation around durable transaction recovery;
2. preserve the invariant that audit or operational evidence cannot authorize retry, failover, resubmission, ledger mutation, treasury movement, or provider funding.

### 49. Milestone Update — Restart Read Error & Deadline Propagation Boundary

**Date:** 2026-09-26

Completed:

- audited startup/restart reads for database error propagation through the context-aware transaction-store interfaces;
- added deterministic PostgreSQL integration coverage proving canceled context reads return `context.Canceled` instead of being treated as an empty store;
- added deterministic PostgreSQL integration coverage proving expired deadlines return `context.DeadlineExceeded` for both single-record and list reads;
- preserved `GetContextE`/`AllContextE` as the error-aware startup boundary while legacy store methods remain backward-compatible;
- kept transaction persistence authoritative and operational/audit evidence non-authoritative.

### Verification

- regression test commit: `542035df53d89bed5488c1852b848e2392160f97`;
- CI for this HEAD must complete `test` and `race` successfully before milestone #49 is considered closed.

### Safety Boundary

- context cancellation/deadline propagation protects startup and persistence error semantics only;
- read failures are not converted into empty transaction state, preventing accidental resubmission decisions from incomplete recovery data;
- audit or operational evidence cannot authorize retry, failover, resubmission, ledger mutation, treasury movement, or provider funding.

### Next Milestone

1. continue auditing transaction-store restart behavior under context cancellation during reconciliation and writes;
2. preserve the invariant that transaction persistence is the sole transaction-state authority.


### Next Milestone

### 50. Milestone Update — Transaction-Store Write Cancellation Boundary

**Date:** 2026-09-26

Completed:

- audited context propagation across durable transaction writes and atomic conditional transitions;
- added deterministic PostgreSQL coverage proving canceled `PutContext` returns `context.Canceled` and leaves no transaction persisted;
- added deterministic PostgreSQL coverage proving canceled `PutIfCurrentContext` returns `context.Canceled` and leaves the durable pending transaction unchanged;
- preserved database-backed conditional transitions as the concurrency authority without converting cancellation into success or conflict semantics;
- corrected the cancellation regression check to use a fresh read context after the write context is canceled, avoiding false “transaction disappeared” results;
- kept transaction persistence authoritative and operational/audit evidence non-authoritative.

### Verification

- milestone test commits: `54a29ed3f994a7fbed4ebc8a82b807712cb030d8`, follow-up corrections through `4dd0a3a557c8fd464b12036893e4c306dca7b201`;
- CI #977 / run `36212349601`: **GREEN** for exact HEAD `4dd0a3a557c8fd464b12036893e4c306dca7b201`;
- CI jobs `test`: success;
- CI job `test` `vet`: success;
- CI jobs `race`: success.

### Safety Boundary

- write cancellation is an error-propagation boundary only;
- canceled persistence cannot be treated as a committed state and cannot authorize provider retry, failover, resubmission, ledger mutation, treasury movement, or provider funding.

### Next Milestone

**#51 — PostgreSQL Write Error Classification Boundary**: verify non-context database write errors remain distinguishable from optimistic concurrency conflicts so the service cannot misclassify persistence failures as successful state transitions.

### 51. Milestone Update — PostgreSQL Write Error Classification Boundary

**Date:** 2026-09-26

Completed:

- added deterministic DBTX stub coverage proving a non-context database write error is preserved through `PutIfCurrentContext` wrapping;
- verified ordinary database failures are not classified as `ErrTransactionStateConflict`;
- preserved optimistic concurrency conflicts as the separate result of a successful SQL execution that affects zero rows;
- kept transaction persistence authoritative and operational/audit evidence non-authoritative.

### Verification

- regression test commit: `f2505b5b411380cd8239c9844897c3306a8818a4`;
- CI #979 / run `36212449794`: **GREEN** for exact HEAD `f2505b5b411380cd8239c9844897c3306a8818a4`;
- CI jobs `test`: success;
- CI job `test` `vet`: success;
- CI jobs `race`: success.

### Safety Boundary

- database write errors remain operational persistence failures and are never treated as concurrency authorization;
- `ErrTransactionStateConflict` is reserved for a conditional transition that did not affect exactly one pending row;
- neither ordinary database failure nor concurrency conflict can authorize provider retry, failover, resubmission, ledger mutation, treasury movement, or provider funding.

### Next Milestone

**#52 — PostgreSQL `PutContext` Error Classification & Version Integrity**: verify sequential transactional writes preserve exact database errors, maintain monotonic versions, and never downgrade a terminal state into a retryable persistence condition.

### 39. Milestone Update — PostgreSQL Reconciliation/CI Integrity Boundary

**Date:** 2026-09-26

Completed:

- repaired the PostgreSQL integration-test variable scope so recovery and concurrent-reconciliation tests use their intended store instances;
- updated the DesKaProvider CI checkout step to validate the pull-request head commit directly instead of relying on a stale synthetic merge ref;
- verified deterministic integration coverage for concurrent service reconciliation converging on one durable terminal result without provider resubmission;
- preserved the invariant that audit or operational evidence cannot authorize retry, failover, resubmission, ledger mutation, treasury movement, or provider funding.

### Verification

- implementation HEAD: `14908ecdf82be215ec611d509f83a5e8a49bce5a`;
- CI #985 / run `36212751435`: **GREEN** for exact HEAD `14908ecdf82be215ec611d509f83a5e8a49bce5a`;
- CI job `test`: success;
- CI job `race`: success;
- checkout validation used the exact PR head SHA rather than the stale merge ref.

### Safety Boundary

- transaction persistence remains authoritative for durable transaction state;
- reconciliation concurrency is resolved through conditional persistence rather than provider resubmission;
- CI validation must execute the exact PR head so a stale merge ref cannot mask or reintroduce build/test regressions;
- no audit, operational, or routing evidence path is granted authority over retry, failover, resubmission, ledger, treasury, or funding decisions.

### Next Milestone

### 52. Milestone Update — PostgreSQL Transaction Version Progression Review

**Date:** 2026-09-26

Completed:

- audited PostgresTransactionStore.PutContext and confirmed sequential state transitions load the current persisted version before issuing the conditional PostgreSQL update;
- verified pending-to-pending updates advance the durable version monotonically instead of reusing a stale hard-coded version;
- retained terminal-state protection and compare-and-transition semantics through the shared conditional update path;
- verified deterministic integration coverage for two sequential pending refreshes followed by a terminal transition and stale-version conflict detection;
- preserved transaction persistence as the authoritative transaction-state boundary.

### Verification

- implementation HEAD: `cc92e43d219be479af09bc4bd415539ca0b8591f`;
- CI #989 / run `36212904436`: **GREEN** for exact HEAD `cc92e43d219be479af09bc4bd415539ca0b8591f`;
- CI jobs `test`: success;
- CI jobs `test` `vet`: success;
- CI jobs `race`: success.

### Safety Boundary

- version progression is a persistence-integrity concern only;
- stale version conflicts remain persistence/concurrency failures and cannot authorize provider retry, failover, resubmission, ledger mutation, treasury movement, or provider funding.

### Next Milestone

**#118 — PostgreSQL Sequential Terminal Idempotency & Conflict Review**: verify repeated identical terminal writes remain idempotent across service/store boundaries while divergent terminal writes remain rejected after version progression.

### 39. Milestone Update — Concurrent Reconciliation / Atomic Transition Boundary

**Date:** 2026-09-26

Completed:

- verified concurrent service reconciliation against a shared PostgreSQL transaction store converges on one durable terminal result without resubmitting the provider purchase;
- repaired integration-test connection/store scoping so isolated PostgreSQL schemas are not mixed across unrelated tests;
- strengthened PR CI checkout to validate the pull-request head commit directly rather than a stale synthetic merge ref;
- verified the reconciliation path accepts an identical durable terminal observation after an atomic state race, while conflicting terminal observations remain rejected;
- preserved transaction persistence as the authoritative transaction state and kept audit/operational evidence non-authoritative.

### Verification

- implementation HEAD: `7dc332d0e412beaf4fcbd580aee777f3195dab34`;
- CI #991 / run `36213251428`: **GREEN** for exact HEAD `7dc332d0e412beaf4fcbd580aee777f3195dab34`;
- CI jobs `test`: success;
- CI job `vet`: success;
- CI job `race`: success.

### Safety Boundary

- atomic reconciliation convergence may finalize the same transaction from concurrent observers but cannot authorize a second provider purchase;
- a stale or conflicting terminal observation remains an error and cannot overwrite the committed transaction result;
- audit persistence and operational snapshots remain evidence only and cannot authorize retry, failover, resubmission, ledger mutation, treasury movement, or provider funding.

### Next Milestone

### 40. Milestone Update — Sequential PostgreSQL Version Progression

**Date:** 2026-09-26

Completed:

- audited PostgreSQL transaction persistence after the first atomic transition and verified `PutContext` uses the persisted `current.Version` for subsequent optimistic transitions;
- retained deterministic integration coverage for pending → pending → terminal progression and stale-version conflict handling;
- verified terminal results remain immutable and stale transitions cannot overwrite a committed result;
- preserved the no-resubmission boundary across restart/reconciliation paths.

### Verification

- verification HEAD: `75c6830a3307a7dfd1f19ad2166c596a2e87f7a5`;
- CI #993 / run `36213485348`: **GREEN** for exact HEAD `75c6830a3307a7dfd1f19ad2166c596a2e87f7a5`;
- CI jobs `test`: success;
- CI job `vet`: success;
- CI job `race`: success.

### Next Milestone

### 53. Milestone Update — Transaction Store Context Cancellation & Side-Effect Boundary Review

**Date:** 2026-09-26

Completed:

- verified canceled PostgreSQL PutIfCurrentContext returns context.Canceled before any durable state transition is committed;
- added deterministic coverage proving canceled PutContext leaves the persisted status, payload, and version unchanged;
- verified canceled context-aware reads return context.Canceled through the error-aware PostgreSQL read boundary;
- preserved database-error classification so infrastructure failures are not misclassified as transaction-state conflicts;
- kept transaction persistence authoritative while cancellation remains a request-lifecycle boundary only.

### Verification

- implementation HEAD: ca5cdc1087f11f65846308b54c79871cb668416d;
- CI #998 / run 36213809675: GREEN;
- CI jobs test: success;
- CI job test vet: success;
- CI job race: success.

### Safety Boundary

- cancellation prevents or interrupts persistence work but cannot authorize retry, failover, resubmission, ledger mutation, treasury movement, or provider funding;
- a canceled write leaves the prior durable transaction state intact;
- audit and operational evidence remain non-authoritative.



### 41. Milestone Update — PostgreSQL Transaction Read Consistency & Durable Result Identity

**Date:** 2026-09-26

Completed:

- added deterministic PostgreSQL integration coverage for a terminal transaction read through the error-aware `GetContextE` path;
- verified request identity, provider identity, purchase-result fields, and durable version are preserved exactly across a database read;
- preserved the invariant that a durable terminal transaction is read as one coherent state and is not reconstructed from mixed request/result fields;
- kept transaction persistence authoritative while audit/operational evidence remains non-authoritative.

### Verification

- regression test commit: `df2aa68190cbef72348267d1c8ca4d76bd993047`;
- CI for this commit must finish with both `test` and `race` jobs **success** before this milestone is considered closed.

### Safety Boundary

- read consistency is a persistence-integrity concern only;
- durable identity reads cannot authorize provider retry, failover, resubmission, ledger mutation, treasury movement, or provider funding.



### 42. Milestone Update — PostgreSQL Concurrent Read/Transition Observation

**Date:** 2026-09-26

Completed:

- added deterministic PostgreSQL integration coverage with separate reader and writer connections during an atomic pending-to-success transition;
- verified concurrent reads observe either the complete pending state or the complete terminal state, never a mixed request/provider/result/version combination;
- preserved the atomic conditional transition boundary while keeping reads observational and non-authoritative;
- kept transaction persistence authoritative while audit and operational evidence remain non-authoritative.

### Verification

- regression test commit: `3a7c04c1b43c23ad3fed2e97a2703cfa77923a6a`;
- CI for this commit must finish with both `test` and `race` jobs **success** before this milestone is considered closed.

### Safety Boundary

- concurrent read consistency is a persistence-integrity concern only;
- an observational read cannot authorize provider retry, failover, resubmission, ledger mutation, treasury movement, or provider funding.


### 43. Milestone Update — PostgreSQL Terminal Result Immutability Under Concurrent Observation

**Date:** 2026-09-26

Completed:

- added deterministic PostgreSQL regression coverage proving a committed terminal result cannot be overwritten by a stale pending observation;
- verified stale terminal transition attempts fail with the transaction-state conflict after the transaction has already advanced to a terminal version;
- verified the durable terminal request identity, provider identity, result payload, and version remain unchanged after the rejected stale observation;
- preserved transaction persistence as the sole authoritative transaction state while audit/operational evidence remains non-authoritative.

### Verification

- implementation commit: `3bc157c6bd046b5294f805af660cbeb6beba4517`;
- CI #1018 / run `36214832262`: **GREEN** for exact HEAD `3bc157c6bd046b5294f805af660cbeb6beba4517`;
- CI jobs `test`: success;
- CI jobs `vet`: success;
- CI jobs `race`: success.

### Safety Boundary

- terminal result immutability is a persistence-integrity property only;
- stale observations cannot overwrite a committed terminal transaction or trigger provider retry/resubmission;
- audit and operational evidence remain non-authoritative and cannot authorize ledger mutation, treasury movement, or provider funding.

### Next Milestone

**#122 — PostgreSQL Idempotent Terminal Read Across Restart:** verify repeated terminal reads after reconstruction continue to return the exact committed result and version without introducing a new transition or provider side effect.
 verify once a terminal result is durably committed, concurrent reads continue returning that exact terminal identity and stale observations cannot change it.

### Next Milestone

**#120 — PostgreSQL Concurrent Read/Transition Observation Review**: verify concurrent durable reads observe either the prior complete state or the committed terminal state and never a partial field combination during a conditional transition.

### Next Milestone

**#119 — PostgreSQL Transaction Read Consistency & Durable Result Identity Review**: verify restart/read paths preserve request/provider/result identity exactly and never expose mixed transaction fields during concurrent observation.

**#118 — Transaction Store Context Cancellation & Side-Effect Boundary Review:** verify canceled PostgreSQL reads/writes stop cleanly and do not leave partial durable transaction state or authorize retries/resubmission.
### 44. Milestone Update — PostgreSQL Idempotent Terminal Read Across Restart

**Date:** 2026-09-26

Completed:

- added deterministic PostgreSQL integration coverage for repeated terminal reads before and after database reconnection;
- verified the committed request identity, provider identity, terminal result payload, and version remain exactly stable across repeated reads after restart;
- verified read operations do not create a new transition or provider side effect;
- preserved transaction persistence as the authoritative transaction state while audit/operational evidence remains non-authoritative.

### Verification

- implementation commit: `66d0836b28cdef57638c840cb8dfcab67725cfdf`;
- CI for this commit must finish with both `test` and `race` jobs **success** before this milestone is considered closed.

### Safety Boundary

- terminal read idempotency is a persistence/read-integrity property only;
- repeated reads after restart cannot authorize retry, failover, resubmission, ledger mutation, treasury movement, or provider funding.

### 45. Milestone Update — PostgreSQL Restart Read + Concurrent Observation Review

**Date:** 2026-09-26

Completed:

- added deterministic PostgreSQL integration coverage that reconstructs durable terminal transaction state after reconnect and reads it concurrently from independently reconstructed store instances;
- verified concurrent restart reads return the exact same request identity, provider identity, terminal result payload, and version;
- verified stale post-restart observations cannot overwrite the committed terminal transaction;
- verified the read path remains observational and does not create a new transition or provider side effect;
- preserved transaction persistence as the authoritative transaction state while audit/operational evidence remains non-authoritative.

### Verification

- implementation commit: `8ede3ed584187fe5bd1c80496dc012a5f58233a4`;
- CI #1032 / run `36215716316`: **GREEN** for exact HEAD `8ede3ed584187fe5bd1c80496dc012a5f58233a4`;
- CI jobs `test`: success;
- CI jobs `vet`: success;
- CI jobs `race`: success.

### Safety Boundary

- restart/concurrent read consistency is a persistence-integrity property only;
- reconstructed reads cannot authorize provider retry, failover, resubmission, ledger mutation, treasury movement, or provider funding;
- stale observations remain non-authoritative and cannot replace a committed terminal result.

### 46. Milestone Update — PostgreSQL Context Cancellation During Durable Read/Transition

**Date:** 2026-09-26

Completed:

- added deterministic PostgreSQL regression coverage for canceled durable reads and canceled atomic transitions;
- verified a canceled read does not return an authoritative transaction state;
- verified a canceled write does not mutate or remove a committed terminal transaction;
- verified the committed request identity, provider identity, terminal result payload, and version remain unchanged after canceled operations;
- preserved transaction persistence as the authoritative transaction state while audit/operational evidence remains non-authoritative.

### Verification

- implementation commit: `22057310eec9c4a58f35470e20b0f2d3af25ea35`;
- CI #1036 / run `36215868824`: **GREEN** for exact HEAD `22057310eec9c4a58f35470e20b0f2d3af25ea35`;
- CI jobs `test`: success;
- CI jobs `vet`: success;
- CI jobs `race`: success.

### Safety Boundary

- context cancellation is a persistence-operation boundary only;
- canceled reads and writes cannot authorize provider retry, failover, resubmission, ledger mutation, treasury movement, or provider funding;
- cancellation never downgrades or replaces an already committed terminal transaction.

### 47. Milestone Update — PostgreSQL Sequential Pending Transition Version Integrity

**Date:** 2026-09-26

Completed:

- audited `PostgresTransactionStore.PutContext` sequential transition logic against the PostgreSQL versioned transition predicate;
- verified each mutable transition uses the currently persisted `Version` in the atomic update condition;
- verified sequential pending -> pending transition advances version from 1 to 2 and preserves the updated pending payload;
- verified subsequent pending -> terminal transition advances version from 2 to 3;
- verified a stale pre-transition version is rejected after the terminal transition;
- preserved transaction persistence as the authoritative transaction state while audit/operational evidence remains non-authoritative.

### Verification

- implementation HEAD: `da7b6015b8089dc83b277ffe6bef07b54a9eeb2c`;
- CI #1046 / run `36216669655`: **GREEN** for exact HEAD `da7b6015b8089dc83b277ffe6bef07b54a9eeb2c`;
- CI jobs `test`: success;
- CI jobs `vet`: success;
- CI jobs `race`: success;
- existing deterministic integration test `TestPostgresTransactionStoreSequentialVersionTransitionIsPreserved` covers the version progression and stale-version rejection.

### Safety Boundary

- version correctness is a persistence-integrity property only;
- stale durable observations remain non-authoritative;
- versioned transitions cannot authorize provider retry, failover, resubmission, ledger mutation, treasury movement, or provider funding.

### 48. Milestone Update — Restart/Reconciliation Concurrency Boundary

**Date:** 2026-09-26

Completed:

- verified deterministic PostgreSQL transaction reconciliation across independently constructed service instances;
- verified concurrent reconciliation converges to the same terminal provider result without resubmitting the provider purchase;
- repaired the PostgreSQL integration-test scope so isolated schema/session setup stays local to each test;
- updated the CI workflow so pull-request validation checks the exact PR head commit instead of a stale synthetic merge checkout;
- preserved transaction persistence as the authoritative transaction state while operational/audit evidence remains non-authoritative.

### Verification

- implementation HEAD: `818b2d3449cab5b83592d4d81ca1499495dfc878`;
- CI #1051 / run `36217131301`: **GREEN**;
- CI jobs `test`: success;
- CI jobs `race`: success;
- CI job `test` also completed `vet`: success.

### Safety Boundary

- reconciliation concurrency is a transaction-state consistency concern only;
- concurrent reconciliation cannot authorize a second provider purchase submission for the same reference;
- atomic transition convergence cannot mutate customer ledger, treasury, or provider funding state;
- audit or operational evidence cannot authorize retry, failover, resubmission, ledger mutation, treasury movement, or provider funding.

### Next Milestone

**#126 — PostgreSQL Concurrent Read/Transition Observation Atomicity:** verify concurrent durable reads observe only complete transaction states while an atomic transition commits, without exposing mixed fields or authorizing side effects.

### 49. Milestone Update — PostgreSQL Reconciliation Test Isolation & PR-Head CI Validation

**Date:** 2026-09-26

Completed:

- repaired PostgreSQL integration-test session/schema scoping so unrelated tests no longer inherit the dedicated concurrent-transition connection;
- preserved isolated schema usage only where the concurrency test requires deterministic session-local routing;
- changed pull-request CI checkout to validate the exact PR head SHA rather than a stale synthetic merge ref;
- verified the exact branch HEAD now executes the repository's standard and race suites successfully;
- preserved transaction persistence as the authoritative transaction state while operational/audit evidence remains non-authoritative.

### Verification

- implementation HEAD: `2f71c4548c03a97d47b51bd6fb69e6908a169f8f`;
- CI #1059 / run `36217437808`: **GREEN**;
- CI jobs `test`: success;
- CI jobs `race`: success;
- CI job `test` completed `vet`: success.

### Safety Boundary

- test-session isolation and CI checkout correctness are validation/infrastructure concerns only;
- concurrent reconciliation still converges on the durable transaction result without resubmitting the provider purchase;
- no retry, failover, resubmission, ledger mutation, treasury movement, or provider funding authorization is introduced.

### 50. Milestone Update — PostgreSQL Concurrent Read/Transition Observation Atomicity

**Date:** 2026-09-26

Completed:

- reviewed PostgreSQL durable read behavior during an atomic pending -> terminal transition;
- verified the store reads a complete transaction row and commits state changes through a single conditional SQL update;
- added deterministic regression coverage performing repeated durable reads while an atomic transition runs concurrently;
- verified every observed state is either the complete pre-transition pending payload or the complete terminal payload, never a field-level mixture;
- preserved transaction persistence as the authoritative transaction state while operational/audit evidence remains non-authoritative.

### Verification

- implementation HEAD: `47e6e1a51bd0d6ff8ca88c78cc0f42753ce47dbd`;
- CI #1092 / run `36219817809`: **GREEN** for exact HEAD `47e6e1a51bd0d6ff8ca88c78cc0f42753ce47dbd`;
- CI jobs `test`: success;
- CI jobs `race`: success;
- CI job `test` completed `vet`: success.

### Safety Boundary

- read/transition atomicity is a persistence-integrity property only;
- no read observation can authorize provider retry, failover, resubmission, customer-ledger mutation, treasury movement, or provider funding.

### 51. Milestone Update — PostgreSQL Restart Read Consistency Boundary

**Date:** 2026-09-26

Completed:

- verified durable PostgreSQL transaction reads remain complete after reconnecting a fresh database session;
- verified transaction identity, provider identity, terminal status, and version metadata survive reconnect without field-level drift;
- added deterministic restart/read regression coverage for a terminal transaction recovered after reconnect;
- preserved transaction persistence as the authoritative transaction state while operational/audit evidence remains non-authoritative.

### Verification

- implementation HEAD: `cba1bbb5fc458c85573e56c75bbcdd379c55cab1`;
- CI #1094 / run `36220200153`: **GREEN** for exact HEAD `cba1bbb5fc458c85573e56c75bbcdd379c55cab1`;
- CI jobs `test`: success;
- CI jobs `race`: success;
- CI job `test` completed `vet`: success.

### Safety Boundary

- restart read consistency is a persistence/recovery property only;
- reconnecting a transaction store does not authorize retry, failover, resubmission, customer-ledger mutation, treasury movement, or provider funding;
- operational and audit evidence remain non-authoritative for transaction state.

### Next Milestone

**#128 — PostgreSQL Restart Read Consistency Boundary:** verify durable reads remain complete and authoritative after reconnect/restart while preserving transaction identity and version metadata.

### 2026-09-26 — CI Validation Follow-up

- final validation HEAD: `d228e81519afee2f8d17bccb2d700aeda0244f9f`;
- CI #1100 / run `36220472799`: **GREEN**;
- CI jobs `test`: success;
- CI jobs `race`: success;
- corrected PR workflow checkout to validate the explicit `pull_request.head.sha` instead of a stale synthetic merge ref;
- repaired PostgreSQL integration-test variable scoping so recovery and concurrent reconciliation coverage execute against the intended store.

### 52. Milestone Update — PR-Head CI Validation & PostgreSQL Integration Scope Closure

**Date:** 2026-09-26

Completed:

- verified the PR workflow now checks out the explicit pull-request head SHA, eliminating validation against the stale synthetic merge ref;
- repaired PostgreSQL integration-test variable scoping so isolated stores remain local to their intended tests;
- verified the current branch head `93a61d0acfcf8534f6928af7a59a91846cb54626` completes both standard and race test suites successfully;
- verified the `test` job and `race` job are both successful, with `test` also completing `vet` successfully;
- retained the existing persistence boundary: durable transaction state is authoritative, operational/audit evidence remains non-authoritative.

### Verification

- implementation HEAD: `93a61d0acfcf8534f6928af7a59a91846cb54626`;
- CI #1102 / run `36220559972`: **GREEN** for exact HEAD `93a61d0acfcf8534f6928af7a59a91846cb54626`;
- CI jobs `test`: success;
- CI jobs `race`: success;
- CI job `test` completed `vet`: success.

### Safety Boundary

- CI checkout correctness and integration-test isolation are validation/infrastructure concerns only;
- no retry, failover, resubmission, customer-ledger mutation, treasury movement, or provider-funding authorization is introduced;
- atomic transaction transitions remain persistence-integrity mechanisms only.

### Next Milestone

**#129 — PostgreSQL Atomic Transition Version Discipline:** review every durable write path for monotonic version handling and ensure sequential pending updates and concurrent compare-and-transition operations use the persisted row version consistently.

### 53. Milestone Update — PostgreSQL Atomic Transition Version Discipline

**Date:** 2026-09-26

Completed:

- reviewed every PostgreSQL durable write path for persisted transaction-version handling;
- retained and verified deterministic regression coverage that sequential pending `PutContext` calls advance the persisted version monotonically;
- verified compare-and-transition paths continue to use the persisted `current.Version` for conditional updates;
- verified cancellation during atomic writes leaves durable transaction state and version unchanged;
- preserved transaction persistence as the authoritative transaction state while operational/audit evidence remains non-authoritative.

### Verification

- implementation HEAD: `0ba10fb158c1e21d6ae837b91aacab416c8c7d70`;
- CI #1104 / run `36220935799`: **GREEN** for exact HEAD `0ba10fb158c1e21d6ae837b91aacab416c8c7d70`;
- CI jobs `test`: success;
- CI jobs `race`: success;
- CI job `test` completed `vet`: success.

### Safety Boundary

- version discipline is a persistence-integrity property only;
- version increments cannot authorize provider retry, failover, resubmission, customer-ledger mutation, treasury movement, or provider funding;
- operational and audit evidence remain non-authoritative for transaction state.

### Next Milestone

**#130 — PostgreSQL Reconciliation Conflict Recovery:** verify stale service instances recover cleanly from `ErrTransactionStateConflict` by reloading the durable transaction and returning the committed terminal result when observations are identical.

### 54. Milestone Update — PostgreSQL Reconciliation Conflict Recovery

**Date:** 2026-09-26

Completed:

- added deterministic service-level regression coverage for a stale instance whose reconciliation loses the atomic compare-and-transition race;
- verified an identical committed terminal result is reloaded from durable state after `ErrTransactionStateConflict` and returned to the stale caller;
- verified divergent terminal observations remain rejected and do not mutate the durable transaction;
- verified conflict recovery never triggers a second provider purchase submission;
- preserved transaction persistence as the authoritative state while operational/audit evidence remains non-authoritative.

### Verification

- implementation HEAD: `abfa361f8f9f6e9a81bba8fe2c22a0385d986aab`;
- CI #1114 / run `36221384173`: **GREEN** for exact HEAD `abfa361f8f9f6e9a81bba8fe2c22a0385d986aab`;
- CI jobs `test`: success;
- CI jobs `race`: success;
- CI job `test` completed `vet`: success.

### Safety Boundary

- reconciliation conflict recovery is a persistence-integrity property only;
- reloading an identical terminal result cannot authorize provider retry, failover, resubmission, customer-ledger mutation, treasury movement, or provider funding;
- divergent durable results remain conflicts and do not get overwritten;
- operational and audit evidence remain non-authoritative for transaction state.

### Next Milestone

1. continue auditing PostgreSQL reconciliation behavior for cancellation/error boundaries after durable conflict recovery;
2. preserve the no-resubmission and no-financial-authorization invariants across all reconciliation outcomes.


### 39. Milestone Update — PostgreSQL Atomic Reconciliation Boundary

**Date:** 2026-09-26

Completed:

- repaired PostgreSQL integration-test scope/ownership around isolated schemas and reopened database handles;
- corrected the PR CI checkout boundary so pull-request validation executes the actual head commit instead of a stale synthetic merge ref;
- added deterministic coverage for concurrent service reconciliation converging on one durable terminal transaction without provider resubmission;
- preserved compare-and-transition semantics through the PostgreSQL conditional update and transaction identity checks.

### Verification

- implementation HEAD: `f325259d4c0e1438927ec065e100dd9249628015`;
- CI #1120 / run `36221718448`: **GREEN**;
- CI jobs `test`: success;
- CI jobs `race`: success.

### Safety Boundary

- PostgreSQL remains the transaction-state authority for persisted purchase state;
- concurrent reconciliation may observe the same provider terminal result but cannot create a second provider submission;
- audit and operational evidence remain non-authoritative for retry, failover, resubmission, ledger mutation, treasury movement, or provider funding.

### Next milestone

**#117 — Reconciliation Conflict Matrix Review**: expand deterministic coverage across identical terminal convergence, conflicting terminal observations, stale pending versions, and restart boundaries without widening financial authority.

### 39. Milestone Update — Atomic Reconciliation Transition Boundary

**Date:** 2026-09-26

Completed:

- audited the PostgreSQL service-reconciliation integration boundary after repeated CI regressions;
- repaired integration-test store scoping so concurrent reconciliation tests use deterministic local transaction-store ownership;
- updated the CI workflow so pull-request runs explicitly checkout the PR head SHA rather than relying on a potentially stale synthetic merge ref;
- verified concurrent service reconciliation converges on the same durable terminal result without resubmitting the provider purchase;
- verified the repository CI for the exact implementation HEAD is green in both `test` and `race` jobs;
- preserved transaction persistence as the authoritative transaction state and kept operational/audit evidence non-authoritative.

### Verification

- implementation HEAD: `741cf959ec51af1c4ec842d0b95ca1e9cf31521d`;
- CI #1127 / run `36222194850`: **GREEN** for exact HEAD;
- CI jobs `test`: success;
- CI jobs `race`: success.

### Safety Boundary

- the reconciliation path may converge durable transaction state across concurrent service instances, but it never authorizes a second provider purchase;
- atomic transition conflict remains a coordination signal, not permission to retry, fail over, resubmit, mutate customer ledger, move treasury funds, or fund a provider;
- audit and operational persistence remain evidence/state inputs only and cannot override durable transaction authority.

### Known Limitations

- the integration test validates concurrent service convergence against PostgreSQL but does not simulate process termination between provider observation and durable transition;
- schema isolation currently relies on the test connection's session search path and remains test-fixture-specific rather than application configuration;
- provider-specific eventual-consistency and network-partition behavior remain outside the deterministic mock boundary.



### 41. Milestone Update — Concurrent Reconciliation Verification & CI Head Integrity

**Date:** 2026-09-26

Completed:

- verified the PostgreSQL concurrent service reconciliation regression after repairing integration-test connection/store scope;
- verified the PR workflow now checks out the actual pull-request head SHA, preventing stale synthetic merge refs from masking branch state;
- verified concurrent independent service instances converge to the same durable terminal result without resubmitting the provider purchase;
- retained strict durable-state assertions and the no-financial-authorization boundary.

### Verification

- final implementation HEAD: `13c355f7f00c4b95cfb5ce96becc8cc5a2626c82`;
- CI #1137 / run `36222719669`: **GREEN** for the exact validated HEAD;
- CI jobs `test`: success;
- CI jobs `race`: success.

### Safety Boundary

- reconciliation only coordinates durable transaction state;
- atomic transition conflicts are coordination outcomes and never authorize retry, failover, resubmission, ledger mutation, treasury movement, or provider funding;
- operational and audit evidence remain non-authoritative.

### Known Limitations

- process termination between external provider observation and durable transition is outside this deterministic integration boundary;
- schema/session isolation remains a test-fixture concern rather than application runtime configuration.

### 42. Milestone Update — Cancellation & Durable Transition Propagation

**Date:** 2026-09-26

Completed:

- added deterministic PostgreSQL regression coverage proving a canceled context does not create or transition a durable transaction;
- verified canceled persistence operations return `context.Canceled` before mutating durable state;
- preserved provider non-resubmission behavior while validating cancellation at the persistence boundary;
- verified the new regression suite under both normal and race-enabled CI.

### Verification

- implementation HEAD: `62cca37d9d67258fc4d43205ddab7408304ade73`;
- CI #1149 / run `36223780137`: **GREEN**;
- CI job `test`: success;
- CI job `race`: success.

### Safety Boundary

- cancellation is a control-flow/persistence boundary and never authorizes provider retry, failover, resubmission, ledger mutation, treasury movement, or provider funding;
- a canceled persistence operation leaves the last durable transaction state unchanged;
- audit and operational evidence remain non-authoritative.

### Next milestone

1. audit database-error propagation and partial-write behavior across `PutContext` and `Reconcile`;
2. verify non-cancellation database failures cannot be mistaken for empty/not-found transaction state;
3. preserve no-resubmission and no-financial-authorization invariants.


### 40. Milestone Update — PostgreSQL Transaction Version Progression

**Date:** 2026-09-26

Completed:

- audited the PostgreSQL `PutContext` transition path and confirmed it uses the current durable row version as the compare-and-transition precondition;
- added deterministic regression coverage for repeated non-terminal persistence followed by terminal transition, validating version progression `1 → 2 → 3`;
- verified the regression passes under both normal and race-enabled repository CI;
- preserved the invariant that version advancement is persistence coordination only and cannot authorize provider resubmission or financial mutation.

### Verification

- implementation HEAD: `6e948be1acb0223a321fe8d2fe0469ca9de5be51`;
- CI #1131 / run `36222360321`: **GREEN** for exact HEAD;
- CI jobs `test`: success;
- CI jobs `race`: success;
- CI job `test` completed `vet`: success.

### Safety Boundary

- transaction version progression protects durable consistency only;
- stale version conflicts remain coordination failures and never become permission to retry, fail over, resubmit, mutate customer ledger, move treasury funds, or fund a provider;
- operational and audit evidence remain non-authoritative for transaction state.

### Next milestone

1. audit cancellation and database-error propagation across `PutContext` and `Reconcile` boundaries;
2. verify a canceled context cannot produce a partial durable transaction transition or accidental provider resubmission;
3. preserve the no-resubmission/no-financial-authorization invariants.

### 39. Milestone Update — Atomic Reconciliation Transition Verification

**Date:** 2026-09-26

Completed:

- repaired the PostgreSQL integration-test variable scoping that caused CI compilation failures;
- verified the PR workflow checks out the actual pull-request head SHA rather than a stale synthetic merge ref;
- verified concurrent service reconciliation converges to one durable terminal result without a second provider purchase submission;
- verified the exact current implementation HEAD completes both repository CI jobs successfully;
- preserved transaction persistence as the authoritative state boundary and kept operational/audit evidence non-authoritative.

### Verification

- final verified implementation HEAD: `bc2d5c7ae2a80b3e8367e15915be218da5b47d6b`;
- CI #1159 / run `36224845284`: **GREEN** for exact HEAD;
- CI job `test`: success;
- CI job `race`: success;
- CI job `test` completed `vet`: success.

### Safety Boundary

- reconciliation only coordinates durable transaction state and provider observation;
- atomic or stale-state conflicts never authorize retry, failover, resubmission, customer-ledger mutation, treasury movement, or provider funding;
- audit and operational evidence remain non-authoritative.

### Known Limitations

- PostgreSQL schema/session isolation in integration tests remains a fixture concern rather than application runtime configuration;
- the deterministic concurrency boundary does not simulate process termination between provider observation and durable commit.

### Next milestone

### 40. Milestone Update — Sequential/Repeated Transaction Transition Review

**Date:** 2026-09-26

Completed:

- audited repeated PostgreSQL pending-state persistence and terminal transition behavior;
- verified the integration suite already covers sequential durable version progression and stale-version rejection;
- verified repeated pending transitions advance the persisted version monotonically while preserving the latest pending result;
- verified terminal transition advances the version once and stale writers cannot mutate the committed terminal result;
- confirmed terminal idempotency remains read/replay-safe without authorizing provider resubmission or financial mutation;
- kept transaction persistence authoritative and operational/audit evidence non-authoritative.

### Verification

- validated implementation HEAD: `3764d7f24e9fbc96527bb4fb1aa45eb69beb92f3`;
- CI #1163 / run `36225513104`: **GREEN**;
- CI jobs `test`: success;
- CI job `race`: success;
- CI job `test` completed `vet`: success.

### Safety Boundary

- PostgreSQL version increments are coordination metadata only;
- stale version conflicts remain persistence conflicts and never authorize retry, failover, resubmission, customer-ledger mutation, treasury movement, or provider funding;
- terminal idempotency does not create a second provider submission path.

### Next milestone

**#118 — Database-Error Propagation Boundary Review**: verify non-cancellation PostgreSQL errors remain distinguishable from not-found/empty state across `GetContextE`, `PutContext`, and `Reconcile`, with no accidental retry authorization.

**#117 — Sequential/Repeated Transaction Transition Review**: continue auditing repeated pending-to-pending persistence and terminal-idempotent behavior, especially PostgreSQL version progression, without widening transaction authority.

### 39. Milestone Update — PostgreSQL Reconciliation Concurrency & CI Restoration

**Date:** 2026-09-26

Completed:

- repaired PostgreSQL integration-test store scoping so isolated-schema fixtures remain local to each test and no cross-test variable leakage remains;
- hardened the DesKaProvider CI checkout path so pull-request validation checks the PR head commit directly instead of a stale synthetic merge ref;
- added deterministic coverage for concurrent service reconciliation converging on one durable terminal result without provider resubmission;
- verified the routing package builds cleanly after the scope corrections;
- preserved transaction persistence as the authoritative transaction state and kept audit/operational evidence non-authoritative.

### Verification

- latest verified HEAD: `1af1a0a67c08c0171fdded1b437fc027777f6b86`;
- CI #1169 / run `36226066545`: **GREEN**;
- CI jobs `test`: success;
- CI jobs `race`: success.

### Safety Boundary

- CI checkout changes only affect validation provenance, not runtime authorization behavior;
- concurrent reconciliation may converge on an already-committed durable transaction result, but cannot trigger a second provider purchase;
- atomic transaction transition conflicts remain conflict signals and do not authorize retry, failover, ledger mutation, treasury movement, or provider funding.

### Next milestone

**#117 — Atomic Transition Version Semantics Review**: audit sequential and concurrent PostgreSQL transaction version transitions so every durable state mutation uses the current version and preserves idempotent terminal behavior across repeated writes and restart recovery.\n

### 39. Milestone Update — Atomic Reconciliation Convergence Verification

**Date:** 2026-09-26

Completed:

- repaired the PostgreSQL integration-test scope so restart/reconciliation coverage uses correctly scoped transaction stores;
- made the PR workflow explicitly checkout `github.event.pull_request.head.sha` for pull-request validation, preventing stale synthetic merge refs from masking branch state;
- preserved the concurrent reconciliation assertion that both independently constructed services converge on the same terminal provider result without resubmitting the provider purchase;
- verified the corrected integration suite and race detector after the scope/workflow fixes;
- kept transaction persistence authoritative while audit and operational evidence remain non-authoritative.

### Verification

- implementation HEAD: `eaca268230fab82f6e5800400ce24a9bb66d2406`;
- CI #1179 / run `36226960719`: **GREEN** for exact HEAD `eaca268230fab82f6e5800400ce24a9bb66d2406`;
- CI jobs `test`: success;
- CI step `vet`: success;
- CI job `race`: success.

### Safety Boundary

- reconciliation convergence only resolves already-authorized provider transaction state;
- reconciliation cannot create a second provider purchase attempt;
- an audit/operational failure does not become transaction authority;
- PostgreSQL atomic transition conflicts remain part of the durable convergence boundary;
- no ledger mutation, treasury movement, automatic provider funding, failover authorization, or retry authorization is introduced.

### Next milestone

**#117 — PostgreSQL Transaction Versioning Review**: audit sequential `PutContext` updates against the durable version column so legitimate pending-state transitions remain correct after prior updates while stale writers still conflict atomically.

### 40. Milestone Update — PostgreSQL Transaction Version Progression

**Date:** 2026-09-26

Completed:

- audited `PostgresTransactionStore.PutContext` against the durable transaction `version` column;
- added deterministic integration coverage for sequential pending → pending → success transitions;
- verified the first pending update advances the durable version from 1 to 2;
- verified the subsequent terminal transition uses the current version and advances it to 3;
- preserved atomic stale-writer protection through version matching;
- kept transaction persistence authoritative and unchanged as the source of transaction state.

### Verification

- implementation HEAD: `e3082a75f02480ebfc76b19158f7ae1feaa6f322`;
- CI #1183 / run `36227055909`: **GREEN** for exact HEAD `e3082a75f02480ebfc76b19158f7ae1feaa6f322`;
- CI jobs `test`: success;
- CI step `vet`: success;
- CI job `race`: success.

### Safety Boundary

- transaction version progression only protects durable transaction-state transitions;
- version changes do not authorize provider retries, failover, resubmission, or financial mutation;
- stale writers remain rejected by the atomic version predicate;
- audit and operational evidence remain non-authoritative;
- no customer ledger, treasury, or automatic provider-funding behavior is introduced.

### Next milestone

**#118 — PostgreSQL Sequential Writer Conflict Review**: add deterministic coverage proving a stale pending writer cannot overwrite a newer pending version, while an up-to-date writer can continue the legitimate transition.
### 41. Milestone Update — PostgreSQL Sequential Writer Conflict Review

**Date:** 2026-09-26

Completed:

- added deterministic PostgreSQL coverage for a stale pending writer attempting to overwrite a newer pending version;
- verified the stale writer is rejected with `ErrTransactionStateConflict` and cannot change the durable pending record;
- verified an up-to-date writer can continue the legitimate pending → success transition;
- verified the durable version advances monotonically from 1 → 2 → 3 across the accepted writes;
- preserved transaction persistence as the authoritative transaction state.

### Verification

- implementation commit: `df8d369f8142c7d40cf4f91321e3ac5eb425e61f`;
- CI for this HEAD must complete with both `test` and `race` jobs **success** before this milestone is considered closed.

### Safety Boundary

- stale-writer rejection is a transaction-state integrity boundary only;
- version advancement does not authorize retries, failover, resubmission, ledger mutation, treasury movement, or provider funding;
- audit and operational evidence remain non-authoritative.

### 42. Milestone Update — PostgreSQL Sequential Writer Conflict Correction + Terminal Idempotency Coverage

**Date:** 2026-09-26

Completed:

- corrected PostgreSQL `PutContext` to reject non-idempotent writes whose caller version does not equal the current durable version;
- changed PostgreSQL `PutContext` to use `GetContextE`, preserving database read errors instead of collapsing them into not-found/empty state;
- added deterministic PostgreSQL coverage for repeated identical terminal writes with no version churn;
- verified identical terminal replay remains idempotent even when the replay carries a stale version, because identical terminal observations do not mutate state;
- added deterministic coverage rejecting `SUCCESS → FAILED` terminal rewrites and both current/stale `FAILED → SUCCESS` rewrites;
- verified conflicting terminal rewrites preserve the committed durable terminal state and version;
- retained restart/reconciliation terminal recovery coverage and exactly-one provider submission assertions already present in the suite;
- kept PostgreSQL transaction persistence authoritative and audit/operational evidence non-authoritative.

### Verification

- sequential writer correction implementation: `4d16122bb72cb9593fb0372bde89f378a013b185`;
- terminal idempotency test baseline: `63e6eb3e153b59c158368beaa7d7b546bd1508d7`;
- stale `FAILED → SUCCESS` regression: `a80134990c425929a86b5f4936756075df83c8de`;
- PostgreSQL error-aware `PutContext` correction: `bf60f275b1b2fbe0d693bed26069247685708727`;
- invalid error-propagation fixture removed: `f371cf85f1fada8f9a4330dd48d616ba507310d0`;
- CI #1218 / run `36229227605`: **GREEN** for exact HEAD `f371cf85f1fada8f9a4330dd48d616ba507310d0`;
- CI #1219 / run `36229229590`: **GREEN** for exact HEAD `f371cf85f1fada8f9a4330dd48d616ba507310d0`;
- CI jobs `test` and `race` both completed successfully on the exact HEAD;
- the repository CI also covers `go vet` in the `test` job.

### Safety Boundary

- stale writer conflicts remain persistence-integrity outcomes and never authorize retry, failover, resubmission, ledger mutation, treasury movement, or provider funding;
- identical terminal replay performs no durable mutation and no provider action;
- conflicting terminal rewrites remain rejected;
- database read failures remain observable and cannot be interpreted as absence of durable transaction state;
- no transaction model, ledger, routing policy, or financial authorization boundary was broadened.

### Known Limitations

- the local runtime still cannot execute the repository's full Go/PostgreSQL verification because it has no external Git/network access;
- live external-provider credential validation remains environment-gated.

### Next milestone

**#120 — PostgreSQL Database-Error Propagation Boundary Review**: extend the error-aware persistence matrix across startup reconstruction, reconciliation reads, and atomic transition writes so non-cancellation database failures remain distinguishable from not-found/conflict outcomes without authorizing retry, failover, resubmission, or financial mutation.



### 43. Milestone Update — PostgreSQL Database-Error Propagation Boundary Review

**Date:** 2026-09-26

Completed:

- audited error-aware PostgreSQL read paths across startup reconstruction and reconciliation;
- confirmed GetContextE distinguishes sql.ErrNoRows from non-cancellation database failures;
- confirmed AllContextE propagates query, scan, and iteration failures to service initialization;
- confirmed reconciliation reloads use getTransactionContextE and preserve database read errors rather than treating them as missing transaction state;
- corrected the PutContext insert-race fallback so a sql.ErrNoRows from INSERT ... RETURNING is followed by GetContextE instead of legacy error-swallowing GetContext;
- preserved propagation of atomic update execution failures and RowsAffected failures without collapsing them into ErrTransactionStateConflict;
- verified the resulting implementation does not introduce retry, failover, resubmission, ledger mutation, treasury movement, or provider-funding authorization.

### Verification

- implementation commit: 826dd330daa64f60eb4a559bff0d5e7bdc422144;
- CI #1224 / run 36230674952: **GREEN** for exact implementation HEAD;
- CI jobs test and race: success;
- CI test job includes successful go vet execution.

### Safety Boundary

- database errors remain observable control/persistence failures;
- not-found remains distinct from database failure;
- atomic state conflicts remain coordination outcomes only;
- no error condition can authorize a second provider submission or financial mutation;
- transaction persistence remains authoritative, while audit and operational evidence remain non-authoritative.

### Known Limitations

- local full Go/PostgreSQL verification remains unavailable in this runtime because external Git/network access is unavailable;
- live external-provider credential validation remains environment-gated.

### Next milestone

**#121 — PostgreSQL Persistence Error-Matrix Expansion**: extend deterministic coverage to additional startup/reconciliation persistence failure cases, especially cancellation versus non-cancellation errors, while retaining the same no-resubmission and no-financial-authorization invariants.

### 44. Milestone Update — PostgreSQL Persistence Error-Matrix Expansion

**Date:** 2026-09-26

Completed:

- added an error-aware transaction-store fixture for deterministic routing-layer persistence failure tests;
- verified service startup propagates non-cancellation persistence read failures from AllContextE instead of treating them as empty state;
- verified canceled initialization preserves context.Canceled rather than replacing it with an injected persistence error;
- added reconciliation coverage proving a non-cancellation GetContextE database failure remains observable after provider status lookup;
- verified the reconciliation read failure preserves the durable pending transaction and does not resubmit the provider purchase;
- retained transaction persistence as the authoritative transaction state and kept audit/operational evidence non-authoritative.

### Verification

- implementation HEAD: 70c3687dc24668dc93f75a468f50b71edab6b18a;
- CI #1232 / run 36235523288: **GREEN** for exact HEAD;
- CI jobs test and race: success;
- the test job includes successful go vet execution.

### Safety Boundary

- persistence read failures remain observable control/persistence errors and are not converted into missing transaction state;
- cancellation remains a control-flow boundary and does not authorize retry, failover, resubmission, ledger mutation, treasury movement, or provider funding;
- reconciliation database errors leave the durable pending state unchanged and cannot create a second provider submission;
- no transaction model, routing policy, or financial authorization boundary was broadened.

### Known Limitations

- local full Go/PostgreSQL verification remains unavailable in this runtime because external Git/network access is unavailable;
- live external-provider credential validation remains environment-gated.

### Next milestone

**#122 — PostgreSQL Persistence Mutation Error-Matrix Expansion**: extend deterministic coverage to persistence mutation failures, including atomic PutIfCurrentContext error propagation and insert/update failure boundaries, while preserving no-resubmission and no-financial-authorization invariants.

### 45. Milestone Update — PostgreSQL Persistence Mutation Error-Matrix Expansion

**Date:** 2026-09-26

Completed:

- added deterministic coverage for PutIfCurrent database execution failures;
- verified database execution errors are propagated with their original cause intact;
- verified database execution errors are not collapsed into ErrTransactionStateConflict;
- retained existing coverage for RowsAffected failures and zero-row optimistic conflicts;
- confirmed the atomic transition SQL path remains the persistence gate for pending-to-terminal state changes;
- no retry, failover, resubmission, ledger mutation, treasury movement, or provider-funding authorization was introduced.

### Verification

- implementation HEAD: 5460f570893305825a99f759a26646fc5621c8f4;
- CI #1236 / run 36236039141: GREEN for exact HEAD;
- CI jobs test and race: success;
- test job Vet: success.

### Safety Boundary

- PostgreSQL mutation failures remain observable persistence errors and are not interpreted as state conflicts;
- optimistic zero-row conflicts remain coordination outcomes only;
- transaction persistence remains authoritative, while audit and operational evidence remain non-authoritative;
- no error path authorizes a second provider submission or financial mutation.

### Known Limitations

- local full Go/PostgreSQL verification remains unavailable in this runtime because external Git/network access is unavailable;
- live external-provider credential validation remains environment-gated.

### Next milestone

**#123 — PostgreSQL Mutation Failure Coverage Extension**: evaluate the remaining PutContext mutation boundaries using a repository-compatible deterministic strategy, without weakening the database abstraction or introducing production behavior changes.


### 46. Milestone Update — PostgreSQL Mutation Failure Coverage Extension

**Date:** 2026-09-26

Completed:

- added an error-aware ContextReadTransactionStore test fixture with an injectable PutContext persistence error;
- verified service purchase propagates the underlying persistence error when pending-state persistence fails;
- verified the persistence failure blocks the external provider submission;
- retained the existing durable-pending safety gate and single-submission behavior;
- no production routing, provider failover, retry, resubmission, ledger mutation, treasury movement, or provider-funding authorization was introduced.

### Verification

- test implementation commit: 35efe39e48f8a575b3b89dccef5864403c2ff7e0;
- CI #1242 / run 36236813319: GREEN for exact HEAD;
- CI #1243 / run 36236816686: GREEN for exact HEAD;
- CI jobs test and race: success;
- test job Vet: success.

### Safety Boundary

- persistence mutation failures remain observable persistence errors;
- pending-state write failure prevents provider submission;
- no persistence error path authorizes a second provider submission or financial mutation;
- transaction persistence remains authoritative, while audit and operational evidence remain non-authoritative.

### Known Limitations

- local full Go/PostgreSQL verification remains unavailable in this runtime because external Git/network access is unavailable;
- live external-provider credential validation remains environment-gated.

### Next milestone

**#124 — PostgreSQL Mutation Error Coverage Completion**: cover remaining concrete PutContext PostgreSQL mutation branches only where the repository's DB abstraction permits deterministic verification, preserving the same safety boundaries.
### 47. Milestone Update — PostgreSQL Mutation Error Coverage Completion

**Date:** 2026-09-26

Completed:

- added integration coverage for `PutContext` INSERT failures caused by the PostgreSQL schema's positive-amount constraint;
- verified the database constraint failure is classified as an insert persistence error rather than `ErrTransactionStateConflict`;
- verified a failed initial INSERT does not create a durable transaction row;
- retained the existing deterministic coverage for atomic update execution failures and `RowsAffected` failures;
- retained the repository-compatible boundary: no artificial `*sql.Row` stubbing, no production behavior change, and no new retry/failover/resubmission path;
- kept transaction persistence authoritative and audit/operational evidence non-authoritative.

### Verification

- implementation commit: `8658c6c1457585653b101eff855891ecd2e01389`;
- CI #1246 / run `36237363703`: **GREEN** for exact HEAD;
- CI #1247 / run `36237365650`: **GREEN** for exact HEAD;
- CI jobs `test` and `race`: success;
- test job `Vet`: success.

### Safety Boundary

- PostgreSQL INSERT constraint failures remain observable persistence errors;
- failed initial persistence does not authorize provider submission;
- persistence errors are not converted into state conflicts or retry authorization;
- no provider failover, resubmission, ledger mutation, treasury movement, or provider funding behavior was introduced.

### Known Limitations

- the current `DBTX` contract returns concrete `*sql.Row`, so an isolated unit-test stub cannot deterministically inject arbitrary INSERT `Scan` failures without changing the production database abstraction;
- the remaining INSERT `Scan` error path is therefore left to the real PostgreSQL integration layer rather than introducing a test-only seam.

### Next milestone

**#125 — PostgreSQL Read/Write Boundary Closure Review**: audit the completed mutation-error matrix against startup reconstruction, reconciliation, and atomic transition paths, then identify the next concrete reliability gap without broadening transaction authority.


### 126. Milestone Update — PostgreSQL Audit Append Error Observability

**Date:** 2026-09-26

Completed:

- closed the concrete audit-store mutation coverage gap identified during the PostgreSQL read/write boundary review;
- added deterministic `DBTX` coverage proving `PostgresTransactionAuditStore.AppendContext` preserves a non-cancellation database execution error;
- verified the wrapped error remains discoverable with `errors.Is`;
- verified an ordinary database append failure is not collapsed into `context.Canceled`;
- verified the append path reaches the intended append-only audit SQL boundary;
- confirmed no production audit-store behavior required changing because the existing adapter already returns the underlying database error correctly;
- kept the existing real-PostgreSQL durability, restart-read, cancellation, webhook, and reconciliation audit-safety coverage unchanged.

### Safety Boundary

- audit append failures remain operational persistence errors only;
- audit persistence cannot authorize provider retry, failover, resubmission, ledger mutation, treasury movement, or provider funding;
- transaction persistence remains the authoritative transaction-state boundary;
- no provider-specific contract or financial authorization boundary was broadened.

### Verification

- implementation commit: `50993770d4eea6476c9252bd1739151c0f367f96`;
- status documentation updated after the implementation commit;
- the resulting branch HEAD must be verified with fresh `test`, `vet`, and `race` CI before this milestone is closed.

### Known Limitations

- arbitrary PostgreSQL `*sql.Row.Scan` failures for the INSERT-returning path remain unsuitable for isolated unit stubbing under the current `DBTX` abstraction;
- real PostgreSQL integration remains the verification boundary for database/driver-specific scan behavior;
- audit remains operational evidence and is not a financial source of truth.

### Next Milestone

**#127 — PostgreSQL Audit Read Error Matrix Review**: verify audit `QueryContext`, row-scan, and iteration failures remain observable and cannot be mistaken for an empty audit history, without broadening the audit or transaction authority boundaries.


### 127. Milestone Update — PostgreSQL Audit Read Error Matrix Review

**Date:** 2026-09-26

Completed:

- extended deterministic PostgreSQL audit-read coverage across the three concrete `AllContext` database error boundaries;
- added a repository-local `database/sql/driver` fixture so `QueryContext` failures can be injected without changing the production `DBTX` abstraction;
- verified query execution failures remain observable through the audit adapter and are not converted into an empty history;
- verified row-read/scan-path failures remain observable through the audit adapter and are not converted into partial audit history;
- verified iteration failures returned from the underlying rows stream remain observable and are not converted into a successful empty/partial result;
- retained the existing real-PostgreSQL durability, ordering, restart/reconnect, cancellation, and append-failure coverage;
- no production audit-store behavior or database abstraction was broadened.

### Safety Boundary

- audit read failures are operational persistence errors only;
- audit history remains observational evidence and cannot authorize provider retry, failover, resubmission, ledger mutation, treasury movement, or provider funding;
- transaction persistence remains the authoritative transaction-state boundary;
- no provider-specific or financial authorization boundary was broadened.

### Verification

- test implementation commit: `f42679bae2cafb0b76e5f78a3e8d987b28287e5f`;
- this commit corrects the deterministic row fixture so its injected row error is actually returned through the `database/sql` read path;
- status documentation update follows after implementation verification;
- latest branch HEAD requires fresh `test`, `vet`, and `race` CI before milestone closure.

### Known Limitations

- the deterministic fixture exercises database/sql driver behavior rather than reproducing every PostgreSQL-driver-specific wire failure;
- real PostgreSQL integration remains the boundary for driver/schema-specific read behavior;
- audit remains operational evidence and is not a financial source of truth.

### Next Milestone

**#128 — PostgreSQL Audit Read Cancellation & Partial-Result Safety:** verify canceled/deadline audit reads stop without exposing a partial authoritative history and preserve the existing observational-only audit boundary.


### 128. Milestone Update — PostgreSQL Audit Read Cancellation & Partial-Result Safety

**Date:** 2026-09-26

Completed:

- added deterministic coverage for PostgresTransactionAuditStore.AllContext with an already-canceled context;
- verified cancellation is returned as context.Canceled before any audit history is exposed;
- verified a canceled audit read returns a nil result rather than partial or stale history;
- preserved the existing query, row-read/scan-path, and rows-iteration failure coverage from milestone #127;
- retained the existing production implementation because its pre-query context check plus nil,error returns on read failures already enforce the required no-partial-history behavior;
- no production audit-store behavior, database abstraction, retry/failover logic, transaction authority, or financial authorization boundary was broadened.

### Verification

- test implementation commit: `00e596cd34df56ca2d52959acd53d88a88a3e8ea`;
- exact branch HEAD after the test change must pass both push and PR CI with `test`, `vet`, and `race` successful before this milestone is considered closed.

### Safety Boundary

- audit cancellation is an operational read-control outcome only;
- cancellation cannot authorize provider retry, failover, resubmission, ledger mutation, treasury movement, or provider funding;
- transaction persistence remains the authoritative transaction-state boundary;
- audit history remains observational evidence and cannot be used as a financial source of truth.

### Known Limitations

- this milestone deterministically verifies an already-canceled read; it does not attempt to emulate every PostgreSQL wire-level mid-stream cancellation timing scenario;
- the database/sql driver fixture remains a repository-local test boundary for deterministic failure injection;
- real PostgreSQL integration remains the boundary for driver-specific cancellation and wire behavior.

### Next Milestone

**#129 — PostgreSQL Audit Mid-Stream Cancellation Safety:** add deterministic coverage for cancellation occurring after audit rows have begun streaming, proving the adapter does not expose the rows accumulated before cancellation.


### 129. Milestone Update — PostgreSQL Audit Mid-Stream Cancellation Safety

**Date:** 2026-09-26

Completed:

- extended the repository-local `database/sql/driver` audit fixture with a deterministic cancellation hook after a row has been delivered;
- added coverage proving `PostgresTransactionAuditStore.AllContext` does not expose the row accumulated before cancellation when cancellation occurs during iteration;
- verified the adapter returns `context.Canceled` and a `nil` result for the mid-stream cancellation case;
- preserved the existing production implementation because its iteration error path already discards accumulated results by returning `nil,error`;
- no production audit-store behavior, database abstraction, retry/failover logic, transaction authority, or financial authorization boundary was broadened.

### Verification

- test implementation commit: `8ca9df67cf67a297d3333a633df246e3d90f5565`;
- exact branch HEAD after the test change must pass both push and PR CI with `test`, `vet`, and `race` successful before this milestone is considered closed.

### Safety Boundary

- mid-stream audit cancellation is an operational read-control outcome only;
- rows accumulated before cancellation are not returned as authoritative history;
- cancellation cannot authorize provider retry, failover, resubmission, ledger mutation, treasury movement, or provider funding;
- transaction persistence remains the authoritative transaction-state boundary and audit remains observational evidence.

### Known Limitations

- the deterministic hook models cancellation after a row is delivered but does not reproduce all PostgreSQL driver/network timing variants;
- the fixture remains a repository-local deterministic boundary for cancellation injection;
- real PostgreSQL integration remains the boundary for driver-specific wire-level behavior.

### Next Milestone

**#130 — PostgreSQL Audit Read Deadline/Error Classification:** verify deadline-driven audit reads preserve `context.DeadlineExceeded` distinctly from ordinary database/iteration failures and do not expose partial history.


### 130. Milestone Update — PostgreSQL Audit Read Deadline/Error Classification

**Date:** 2026-09-26

Completed:

- corrected the canceled-read fixture to use scan-safe string values so cancellation coverage is isolated from unrelated `database/sql` NULL conversion errors;
- added deterministic coverage for `PostgresTransactionAuditStore.AllContext` with an expired deadline;
- verified deadline-driven audit reads preserve `context.DeadlineExceeded` and do not return any audit history;
- retained the existing production implementation because its context pre-check and error-aware `nil,error` read paths already preserve cancellation/deadline classification and prevent partial history exposure;
- no production audit-store behavior, database abstraction, retry/failover logic, transaction authority, or financial authorization boundary was broadened.

### Verification

- test implementation commit: `a62eacbe4bb9ea24241fc90a28d9bd8786934301`;
- exact branch HEAD after this change must pass both push and PR CI with `test`, `vet`, and `race` successful before this milestone is considered closed.

### Safety Boundary

- deadline expiration is an operational read-control outcome only;
- `context.DeadlineExceeded` remains distinguishable from ordinary database/iteration failures;
- deadline/error paths cannot authorize provider retry, failover, resubmission, ledger mutation, treasury movement, or provider funding;
- transaction persistence remains the authoritative transaction-state boundary and audit remains observational evidence.

### Known Limitations

- this milestone uses an already-expired deadline for deterministic classification rather than reproducing every wire-level timeout timing pattern;
- mid-stream cancellation behavior remains covered separately by milestone #129;
- real PostgreSQL integration remains the boundary for driver-specific deadline and timeout behavior.

### Next Milestone

**#131 — PostgreSQL Audit Read Service Boundary Review:** trace audit-read error handling through service-layer callers to ensure database cancellation/deadline/error outcomes are not flattened into empty history or used to trigger provider actions.

### 131. Milestone Update — PostgreSQL Audit Read Service Boundary Review

**Date:** 2026-09-26

Completed:

- traced the current service-layer audit usage and confirmed Service records audit events but does not consume AuditStore history for startup reconstruction, purchase authorization, provider selection, or reconciliation;
- confirmed startup recovery and reconciliation use the transaction store as the authoritative state boundary rather than audit history;
- added deterministic service-layer guardrail coverage proving an audit read that returns context.DeadlineExceeded remains observable and cannot authorize a provider resubmission;
- verified the pending durable transaction remains the result returned for an idempotent repeat and the mock provider submission count remains exactly one;
- no production service behavior required changing because audit history is already observational-only at the service boundary.

### Safety Boundary

- audit read errors and deadline/cancellation outcomes are operational evidence failures only;
- audit history cannot authorize provider retry, failover, resubmission, ledger mutation, treasury movement, or provider funding;
- transaction persistence remains the authoritative transaction-state boundary;
- provider submission authorization remains gated by durable transaction state, not audit history.

### Verification

- implementation commit: 7dc05027c5661add8f6eeeaa818e622c63a73a58;
- exact branch HEAD after this documentation update must pass both push and PR CI with test, vet, and race successful before this milestone is considered closed.

### Known Limitations

- the service-layer guardrail verifies the current architecture where audit reads are not consumed by service actions; it does not introduce an audit history consumer solely for testing;
- the deterministic audit-read fixture returns a controlled error rather than emulating every PostgreSQL/network failure timing variant;
- real PostgreSQL integration remains the boundary for driver-specific audit-read behavior.

### Next Milestone

**#132 — PostgreSQL Audit Read API Boundary:** review whether the audit-store interface should expose an explicit error-aware read contract for callers, without making audit history authoritative or widening provider transaction authority.

### 132. Milestone Update — PostgreSQL Audit Read API Boundary

**Date:** 2026-09-26

Completed:

- formalized an optional error-aware audit read capability as ContextReadTransactionAuditStore;
- added AllContextE(ctx, referenceID) to the capability so callers can distinguish cancellation, deadlines, database failures, and successful reads without removing the legacy All() compatibility method;
- aligned the in-memory audit store with the new capability and added cancellation coverage returning context.Canceled with a nil result;
- aligned the PostgreSQL audit store with the formal AllContextE API while retaining All() as a compatibility wrapper that returns nil on read error;
- migrated PostgreSQL audit-read tests to the formal error-aware API;
- kept audit observational-only: no audit read can authorize provider retry, failover, resubmission, ledger mutation, treasury movement, or provider funding;
- no transaction authority or provider-specific behavior was broadened.

### Safety Boundary

- transaction persistence remains the authoritative transaction-state boundary;
- audit history remains operational evidence only;
- legacy All() intentionally keeps compatibility semantics and must not be used when the caller needs error classification;
- error-aware callers should use ContextReadTransactionAuditStore.AllContextE to preserve cancellation, deadline, and database-error visibility.

### Verification

- implementation commits: 124e297886e67ba68445bacac42a26b979123fa7, 39d72f3fb8c73d653da72dc70442e0a5dac5e4b7, fdc7ccb2c379ab1e050ee62f0685c453704b5148, a50fd58737bf5892a9620d069abfd54897ac9e2e, 8e28b83456fdd5032e7554fb790f4f15ea9d4864;
- status documentation update follows after the final implementation commit;
- exact branch HEAD after this documentation update must pass both push and PR CI with test, vet, and race successful before this milestone is considered closed.

### Known Limitations

- current service code does not consume audit history, so the new capability is an API boundary for future error-aware callers rather than a new service decision input;
- All() remains a compatibility convenience that intentionally flattens read errors to nil;
- real PostgreSQL driver/network behavior remains the integration verification boundary.

### Next Milestone

**#133 — PostgreSQL Audit Read Compatibility & Adoption Review:** identify any remaining internal callers that still use compatibility All() and determine whether they should migrate to AllContextE where error classification materially matters, without making audit authoritative.

### 133. Milestone Update — PostgreSQL Audit Read Compatibility & Adoption Review

**Date:** 2026-09-26

Completed:

- audited the current DesKaProvider routing/service source tree for audit-history consumers;
- confirmed no production/service caller currently uses compatibility TransactionAuditStore.All() to make provider, retry, failover, reconciliation, or transaction-state decisions;
- kept All() as a compatibility/observational API rather than forcing a needless production migration;
- retained ContextReadTransactionAuditStore.AllContextE as the explicit error-aware capability for future callers that materially need cancellation, deadline, or database-error classification;
- added a test-level compile-time capability assertion for the audit read failure fixture and a compatibility test confirming the memory store still exposes the error-aware capability while All() remains observational;
- no production provider authority, transaction authority, retry/failover behavior, ledger mutation, treasury movement, or provider funding behavior was broadened.

### Safety Boundary

- compatibility All() is not an authorization source and remains unsuitable when the caller needs error classification;
- AllContextE is the supported error-aware audit-read boundary;
- transaction persistence remains authoritative for transaction state and idempotency;
- audit remains operational evidence only.

### Verification

- implementation/test commit: 37f4abb639189c9abf8a001ec7929027e326e40c;
- exact branch HEAD after this documentation update must pass both push and PR CI with test, vet, and race successful before this milestone is considered closed.

### Known Limitations

- there are currently no production audit-history consumers to migrate, so adoption review is based on the current source tree rather than a production caller migration;
- compatibility All() intentionally flattens read errors to nil;
- real PostgreSQL driver/network behavior remains the integration boundary for driver-specific read failures.

### Next Milestone

**#134 — PostgreSQL Audit Store Error Taxonomy Review:** verify wrapped audit read/append errors preserve errors.Is matching and distinguish cancellation/deadline from ordinary database failures without changing transaction authority.


### 134. Milestone Update — PostgreSQL Audit Store Error Taxonomy Review

**Date:** 2026-09-26

Completed:

- locked the PostgreSQL audit append/read error taxonomy with deterministic `errors.Is` coverage;
- verified ordinary append database failures preserve their sentinel identity through the adapter's wrapped error;
- added explicit append deadline coverage and verified `context.DeadlineExceeded` is preserved distinctly from `context.Canceled`;
- verified query, scan-path, and rows-iteration failures preserve their sentinel identity through the adapter's wrapped errors;
- verified ordinary query/scan/iteration failures are not misclassified as `context.Canceled` or `context.DeadlineExceeded`;
- retained the existing nil-result behavior for canceled/deadline/read-error paths so partial audit history is never exposed after an error;
- no production audit-store implementation change was required because existing `%w` wrapping already preserves the underlying error taxonomy;
- no transaction authority, provider submission authority, retry/failover behavior, ledger mutation, treasury movement, or provider funding behavior was broadened.

### Safety Boundary

- `context.Canceled` and `context.DeadlineExceeded` remain operational request-control outcomes only;
- ordinary database/query/scan/iteration errors remain distinguishable operational persistence failures;
- error classification cannot authorize provider retry, failover, resubmission, or financial state mutation;
- transaction persistence remains the authoritative transaction-state boundary and audit remains observational evidence only.

### Verification

- test implementation commit: `47a7bfdb8278476978150ed6f6b5cdcf6198a98d`;
- exact branch HEAD after this status documentation update must pass both push and PR CI with `test`, `vet`, and `race` successful before this milestone is considered closed.

### Known Limitations

- the deterministic fixture validates adapter error taxonomy and `errors.Is` semantics without reproducing every PostgreSQL wire/network failure timing variant;
- real PostgreSQL integration remains the boundary for driver-specific error behavior;
- compatibility `All()` intentionally flattens read errors to nil and remains unsuitable when callers require error classification.

### Next Milestone

**#135 — PostgreSQL Audit Integration Recovery Observability:** verify audit append/read failures remain diagnosable across real PostgreSQL integration/restart scenarios without turning audit state into transaction authority.

### 135. Milestone Update — PostgreSQL Audit Integration Recovery Observability

**Date:** 2026-09-26

Completed:

- added real-PostgreSQL integration coverage for an audit read after the underlying database connection has been closed;
- verified the failed read returns an error and does not expose partial audit history;
- verified a closed-database failure is not misclassified as `context.Canceled` or `context.DeadlineExceeded`;
- reopened PostgreSQL, restored the isolated schema context, and verified the same durable audit event becomes readable again after recovery;
- confirmed recovery observability is limited to audit diagnostics and does not mutate transaction state or authorize a provider resubmission;
- no production audit-store or service behavior required changing for this recovery scenario;
- no transaction authority, provider submission authority, retry/failover behavior, ledger mutation, treasury movement, or provider funding behavior was broadened.

### Safety Boundary

- audit connection/read failures remain operational persistence failures only;
- recovery of the audit read path does not reconstruct or overwrite transaction state;
- audit history remains observational evidence and cannot authorize provider retry, failover, resubmission, or financial state mutation;
- transaction persistence remains the authoritative transaction-state boundary.

### Verification

- test implementation commit: `d38e712c1b116dd951dc0e12e21163bcb7eff38a`;
- exact branch HEAD after this status documentation update must pass both push and PR CI with `test`, `vet`, and `race` successful before this milestone is considered closed.

### Known Limitations

- the recovery scenario uses the repository's existing real PostgreSQL integration harness and validates connection-close/reopen behavior, not every network partition or server failover timing variant;
- the deterministic driver fixture remains the unit boundary for injected query/scan/iteration failures;
- audit remains operational evidence and is not a financial source of truth.

### Next Milestone

**#136 — PostgreSQL Audit Append Recovery Observability:** verify audit append failures after connection loss remain diagnosable and that a recovered connection does not duplicate or authorize transaction side effects.


### 136. Milestone Update — PostgreSQL Audit Append Recovery Observability

**Date:** 2026-09-26

Completed:

- added real-PostgreSQL integration coverage for an audit append after the underlying database connection has been closed;
- verified the failed append returns an ordinary database error and is not misclassified as `context.Canceled` or `context.DeadlineExceeded`;
- reopened PostgreSQL, restored the isolated schema context, and verified the same intended audit event can be appended successfully after recovery;
- verified the recovered append produces exactly one durable audit row for the reference, proving the failed closed-connection attempt did not create a hidden duplicate;
- preserved the existing append-only audit model: recovery writes only the explicitly retried audit event and does not reconstruct or mutate transaction state;
- no production audit-store implementation change was required;
- no transaction authority, provider submission authority, retry/failover behavior, ledger mutation, treasury movement, or provider funding behavior was broadened.

### Safety Boundary

- a closed-database audit append failure is an operational persistence failure only;
- a successful retry after database recovery must be an explicit append operation, not an inferred replay of transaction authority;
- audit recovery cannot authorize provider retry, failover, resubmission, ledger mutation, treasury movement, or provider funding;
- transaction persistence remains the authoritative transaction-state boundary.

### Verification

- test implementation commit: `574c22d237f7806473eb9e80638d2cd1ef40f357`;
- exact branch HEAD after this status documentation update must pass both push and PR CI with `test`, `vet`, and `race` successful before this milestone is considered closed.

### Known Limitations

- the recovery scenario validates database connection close/reopen behavior with the existing real PostgreSQL harness, not every network partition, transaction timeout, or server failover timing variant;
- the test proves no duplicate row is created by the observed closed-connection failure path, but does not establish global idempotency for arbitrary repeated audit append calls with identical payloads;
- audit remains operational evidence and is not a financial source of truth.

### Next Milestone

**#137 — PostgreSQL Audit Append Idempotency Boundary:** determine and lock the behavior of repeated identical audit append attempts, especially around recovery/retry, without turning the audit table into a transaction-authority or deduplication mechanism.


### 137. Milestone Update — PostgreSQL Audit Append Idempotency Boundary

**Date:** 2026-09-26

Completed:

- added real-PostgreSQL integration coverage for repeated identical audit append attempts;
- verified two explicit identical `AppendContext` calls produce two durable audit rows with the same evidence payload, preserving the repository's append-only audit semantics;
- verified repeated identical audit appends do not overwrite prior evidence and do not create transaction-state rows;
- confirmed the audit table is not being used as an implicit deduplication or transaction-authority mechanism;
- no production audit-store implementation change was required because the current SQL is append-only by design;
- no transaction authority, provider submission authority, retry/failover behavior, ledger mutation, treasury movement, or provider funding behavior was broadened.

### Safety Boundary

- identical audit events are evidence records, not idempotency keys for transaction execution;
- audit append repetition cannot authorize provider retry, failover, resubmission, ledger mutation, treasury movement, or provider funding;
- transaction persistence remains the authoritative transaction-state and idempotency boundary;
- any future audit deduplication must remain separate from transaction execution authority and must not silently discard operational evidence.

### Verification

- test implementation commit: `ce861e6b79292e02e059f4fff9733b629617f590`;
- exact branch HEAD after this status documentation update must pass both push and PR CI with `test`, `vet`, and `race` successful before this milestone is considered closed.

### Known Limitations

- the test locks the current append-only behavior for repeated identical payloads; it does not define idempotency for semantically equivalent but differently encoded or timestamped events;
- transaction execution idempotency continues to be governed by durable transaction state, not by the audit table;
- audit remains operational evidence and is not a financial source of truth.

### Next Milestone

**#138 — PostgreSQL Audit Ordering & Timestamp Collision Boundary:** verify deterministic ordering when multiple audit events share the same `created_at` timestamp and ensure ordering remains observational without influencing transaction authority.


### 138. Milestone Update — PostgreSQL Audit Ordering & Timestamp Collision Boundary

**Date:** 2026-09-26

Completed:

- added real-PostgreSQL integration coverage for multiple audit events sharing the exact same `created_at` timestamp;
- verified audit reads return those events in deterministic insertion order through the existing `ORDER BY created_at, audit_id` query;
- verified the timestamp collision tie-breaker is the audit row identity and remains an observational storage concern;
- confirmed audit ordering does not participate in provider selection, retry authorization, reconciliation authority, transaction-state mutation, or financial state transitions;
- no production audit-store implementation change was required because the existing SQL already includes the deterministic `audit_id` tie-breaker;
- no transaction authority, provider submission authority, retry/failover behavior, ledger mutation, treasury movement, or provider funding behavior was broadened.

### Safety Boundary

- audit event ordering is for deterministic observation and diagnostics only;
- equal timestamps do not require using audit history as transaction authority;
- the `audit_id` tie-breaker does not become a transaction idempotency or execution key;
- transaction persistence remains the authoritative transaction-state and idempotency boundary.

### Verification

- test implementation commit: `a18362cd00f88dfb65d9efff003971edf89eea56`;
- exact branch HEAD after this status documentation update must pass both push and PR CI with `test`, `vet`, and `race` successful before this milestone is considered closed.

### Known Limitations

- the test covers exact timestamp equality in the repository's real PostgreSQL environment, not every clock precision or cross-node timestamp-generation scenario;
- deterministic ordering relies on the existing `audit_id` monotonic identity within the database table;
- audit remains operational evidence and is not a financial source of truth.

### Next Milestone

**#139 — PostgreSQL Audit Ordering Across Restart:** verify that persisted audit ordering remains unchanged after database connection close/reopen and service reconstruction, without turning audit ordering into execution authority.


### 139. Milestone Update — PostgreSQL Audit Ordering Across Restart

**Date:** 2026-09-26

Completed:

- added real-PostgreSQL integration coverage for persisted audit ordering before and after database connection close/reopen;
- verified audit history retains the same event sequence after reopening PostgreSQL and reconstructing the audit-store adapter;
- verified timestamp-collision ordering remains stable across restart because persisted `audit_id` ordering is retained;
- confirmed restart/read behavior does not use audit ordering to authorize provider retry, reconciliation, transaction-state mutation, or financial state transitions;
- no production audit-store implementation change was required because the existing ordering query is already based on durable `created_at, audit_id` values;
- no transaction authority, provider submission authority, retry/failover behavior, ledger mutation, treasury movement, or provider funding behavior was broadened.

### Safety Boundary

- persisted audit ordering is deterministic observational evidence only;
- restart reconstruction must not infer transaction execution authority from audit sequence;
- `audit_id` remains an audit-row ordering key, not a transaction execution or idempotency key;
- transaction persistence remains the authoritative transaction-state and idempotency boundary.

### Verification

- test implementation commit: `fd941c1a0f9403debb276e8af9fed2185c6f6490`;
- exact branch HEAD after this status documentation update must pass both push and PR CI with `test`, `vet`, and `race` successful before this milestone is considered closed.

### Known Limitations

- the restart scenario uses connection close/reopen against the existing real PostgreSQL harness and does not cover every crash-recovery or multi-primary replication timing;
- ordering stability assumes the audit table's durable `audit_id` identity remains intact across restart;
- audit remains operational evidence and is not a financial source of truth.

### Next Milestone

**#140 — PostgreSQL Audit Ordering Under Concurrent Appends:** verify deterministic audit ordering when concurrent append operations share the same `created_at` timestamp, without allowing audit ordering to influence transaction authority.


### 140. Milestone Update — PostgreSQL Audit Ordering Under Concurrent Appends

**Date:** 2026-09-26

Completed:

- added real-PostgreSQL integration coverage for concurrent audit appends sharing the exact same `created_at` timestamp;
- corrected the integration harness after CI exposed a PostgreSQL session-scoping race: each concurrent worker now pins its own connection and sets the isolated `search_path` before appending, while verification queries use the schema-qualified table;
- verified all concurrent append operations persist successfully without collapsing distinct audit evidence;
- verified persisted rows are read using the existing `created_at, audit_id` ordering and every concurrently written event remains uniquely represented;
- confirmed concurrent audit ordering is deterministic at the persisted-row level without making audit order an input to transaction execution authority;
- no production audit-store implementation change was required because the existing schema identity and ordering query already provide the required deterministic tie-breaker;
- no transaction authority, provider submission authority, retry/failover behavior, ledger mutation, treasury movement, or provider funding behavior was broadened.

### Safety Boundary

- concurrent audit append order is observational evidence only;
- `audit_id` remains a storage-order identity and is not a transaction execution or idempotency key;
- audit concurrency cannot authorize provider retry, failover, resubmission, ledger mutation, treasury movement, or provider funding;
- transaction persistence remains the authoritative transaction-state and idempotency boundary.

### Verification

- test implementation commits: `d4e03a00443654b66f71b954276d07031199f0d5`, `9a2da42182b5dc8ae51b55a8dd915f9386670fce`;
- exact branch HEAD after this status documentation update must pass both push and PR CI with `test`, `vet`, and `race` successful before this milestone is considered closed.

### Known Limitations

- concurrent PostgreSQL scheduling is nondeterministic, so the test validates uniqueness and deterministic persisted ordering rather than assuming goroutine completion order;
- the scenario covers same-timestamp concurrent inserts on one PostgreSQL deployment, not cross-region replication ordering or clock-skew semantics;
- audit remains operational evidence and is not a financial source of truth.

### Next Milestone

**#141 — PostgreSQL Audit Concurrent Read/Append Visibility:** verify readers never observe partially inserted audit rows while concurrent append operations are in flight, while preserving the observational-only audit boundary.


### 141. Milestone Update — PostgreSQL Audit Concurrent Read/Append Visibility

**Date:** 2026-09-26

Completed:

- added real-PostgreSQL integration coverage for a reader observing audit history while another connection holds a new audit row uncommitted;
- verified the reader sees only previously committed audit evidence and does not observe the uncommitted row or any partially written payload;
- committed the staged audit append and verified the same reader then observes the complete committed audit history in deterministic `created_at, audit_id` order;
- confirmed PostgreSQL transaction isolation provides the visibility boundary while the audit adapter remains append-only and observational;
- no production audit-store implementation change was required;
- no transaction authority, provider submission authority, retry/failover behavior, ledger mutation, treasury movement, or provider funding behavior was broadened.

### Safety Boundary

- uncommitted audit rows are not exposed to concurrent readers;
- audit read visibility is a persistence/isolation property, not a transaction authorization signal;
- audit visibility cannot authorize provider retry, failover, resubmission, ledger mutation, treasury movement, or provider funding;
- transaction persistence remains the authoritative transaction-state and idempotency boundary.

### Verification

- test implementation commit: `fb7576b3710101c9aaadbd8953c5eac97e79db1e`;
- exact branch HEAD after this status documentation update must pass both push and PR CI with `test`, `vet`, and `race` successful before this milestone is considered closed.

### Known Limitations

- the scenario validates visibility against one uncommitted PostgreSQL transaction and does not model every isolation level, replica lag, or failover topology;
- the test proves no uncommitted row is exposed but does not generalize this result to arbitrary application-level caching layers outside the audit adapter;
- audit remains operational evidence and is not a financial source of truth.

### Next Milestone

**#142 — PostgreSQL Audit Read Visibility After Concurrent Commit:** verify readers converge to the same committed audit history after concurrent writers commit, including same-timestamp events, without introducing ordering or state ambiguity.


### 142. Milestone Update — PostgreSQL Audit Read Visibility After Concurrent Commit

**Date:** 2026-09-26

Completed:

- added real-PostgreSQL integration coverage for an audit reader before and after a concurrent writer commits a staged audit row;
- verified the reader sees only the previously committed audit event before the concurrent transaction commits;
- verified the same reader converges to the complete committed audit history after commit, with both same-timestamp events preserved in deterministic `created_at, audit_id` order;
- confirmed PostgreSQL transaction visibility prevents partial/uncommitted audit evidence from appearing while allowing committed evidence to become visible without rebuilding or mutating transaction state;
- no production audit-store implementation change was required;
- no transaction authority, provider submission authority, retry/failover behavior, ledger mutation, treasury movement, or provider funding behavior was broadened.

### Safety Boundary

- audit visibility before and after concurrent commit remains a persistence/isolation property only;
- a newly committed audit event does not grant or alter transaction execution authority;
- audit ordering and visibility cannot authorize provider retry, failover, resubmission, ledger mutation, treasury movement, or provider funding;
- transaction persistence remains the authoritative transaction-state and idempotency boundary.

### Verification

- test implementation commit: `63cff012a21ac8bd70b6e1a310dac8a70eecbae7`;
- exact branch HEAD after this status documentation update must pass both push and PR CI with `test`, `vet`, and `race` successful before this milestone is considered closed.

### Known Limitations

- the scenario validates one concurrent writer commit against a real PostgreSQL deployment and does not cover every isolation level, replica lag, or distributed database topology;
- same-timestamp ordering relies on the existing durable `audit_id` tie-breaker;
- audit remains operational evidence and is not a financial source of truth.

### Next Milestone

**#143 — PostgreSQL Audit Read After Repeated Concurrent Commits:** verify readers converge monotonically as multiple committed audit writers finish, preserving every durable event exactly once in deterministic order without affecting transaction authority.


### 142. Milestone Update — PostgreSQL Audit Read Visibility After Concurrent Commit

**Date:** 2026-09-26

Completed:

- added real-PostgreSQL integration coverage for an audit reader before and after a concurrent writer commits a staged audit row;
- verified the reader sees only the previously committed audit event before the concurrent transaction commits;
- verified the same reader converges to the complete committed audit history after commit, with both same-timestamp events preserved in deterministic `created_at, audit_id` order;
- corrected the integration test to pin the reader connection and its isolated `search_path`, avoiding `database/sql` connection-pool session scoping from making the test depend on an arbitrary connection;
- confirmed PostgreSQL transaction visibility prevents partial/uncommitted audit evidence from appearing while allowing committed evidence to become visible without rebuilding or mutating transaction state;
- no production audit-store implementation change was required;
- no transaction authority, provider submission authority, retry/failover behavior, ledger mutation, treasury movement, or provider funding behavior was broadened.

### Safety Boundary

- audit visibility before and after concurrent commit remains a persistence/isolation property only;
- a newly committed audit event does not grant or alter transaction execution authority;
- audit ordering and visibility cannot authorize provider retry, failover, resubmission, ledger mutation, treasury movement, or provider funding;
- transaction persistence remains the authoritative transaction-state and idempotency boundary.

### Verification

- initial test implementation commit: `63cff012a21ac8bd70b6e1a310dac8a70eecbae7`;
- CI exposed a test-harness session-scoping defect; the correction commit is `1406fca46d439447c36e12f6326780922277b2f2`;
- exact branch HEAD after this status documentation update must pass both push and PR CI with `test`, `vet`, and `race` successful before this milestone is considered closed.

### Known Limitations

- the scenario validates one concurrent writer commit against a real PostgreSQL deployment and does not cover every isolation level, replica lag, or distributed database topology;
- same-timestamp ordering relies on the existing durable `audit_id` tie-breaker;
- audit remains operational evidence and is not a financial source of truth.

### Next Milestone

**#143 — PostgreSQL Audit Read After Repeated Concurrent Commits:** verify readers converge monotonically as multiple committed audit writers finish, preserving every durable event exactly once in deterministic order without affecting transaction authority.


### 142. Milestone Update — PostgreSQL Audit Read Visibility After Concurrent Commit

**Date:** 2026-09-26

Completed:

- added real-PostgreSQL integration coverage for audit history read before and after a concurrent writer commits;
- verified the reader sees only the previously committed audit event while the second event remains uncommitted;
- committed the second event and verified the reader then converges to the complete committed audit history without partial payload exposure;
- verified same-timestamp committed events retain deterministic `created_at, audit_id` ordering after the concurrent commit;
- confirmed read visibility/convergence is a PostgreSQL persistence and isolation property only and does not feed transaction execution authority;
- no production audit-store implementation change was required;
- no transaction authority, provider submission authority, retry/failover behavior, ledger mutation, treasury movement, or provider funding behavior was broadened.

### Safety Boundary

- audit readers observe only committed rows;
- commit visibility does not turn audit history into a transaction-state source;
- audit ordering remains observational and deterministic, not an execution or idempotency key;
- transaction persistence remains the authoritative transaction-state and idempotency boundary.

### Verification

- test implementation commit: `c55d4a1d`;
- exact branch HEAD after this status documentation update must pass both push and PR CI with `test`, `vet`, and `race` successful before this milestone is considered closed.

### Known Limitations

- the scenario validates one concurrent commit boundary on the repository's PostgreSQL integration harness and does not cover every isolation level, replica lag, or distributed PostgreSQL topology;
- deterministic ordering remains tied to the persisted `audit_id` identity for equal timestamps;
- audit remains operational evidence and is not a financial source of truth.

### Next Milestone

**#143 — PostgreSQL Audit Read Consistency Across Repeated Concurrent Snapshots:** verify repeated audit reads converge on a stable committed sequence while concurrent appends occur, without making read sequence part of transaction authority.


### 143. Milestone Update — PostgreSQL Audit Read Consistency Across Repeated Concurrent Snapshots

**Date:** 2026-09-26

Completed:

- added real-PostgreSQL integration coverage for repeated audit snapshots while committed audit writers finish sequentially;
- verified each snapshot contains only already-committed audit evidence and grows monotonically as new committed rows become visible;
- verified every committed event remains present exactly once in the final snapshots and preserves the existing deterministic `created_at, audit_id` ordering;
- verified repeated reads after all writers finish converge to the same complete durable audit sequence;
- confirmed read convergence is a persistence/isolation property only and does not participate in transaction execution authority or provider submission authorization;
- no production audit-store implementation change was required;
- no transaction authority, provider submission authority, retry/failover behavior, ledger mutation, treasury movement, or provider funding behavior was broadened.

### Safety Boundary

- audit snapshots contain committed evidence only;
- snapshot growth cannot authorize provider retry, failover, resubmission, ledger mutation, treasury movement, or provider funding;
- audit sequence remains observational evidence, not a transaction execution or idempotency key;
- transaction persistence remains the authoritative transaction-state and idempotency boundary.

### Verification

- test implementation commit: `28023ecbc288e89bd5c9acc10b47124b7d01f2f2`;
- exact branch HEAD after this status documentation update must pass both push and PR CI with `test`, `vet`, and `race` successful before this milestone is considered closed.

### Known Limitations

- the test models repeated snapshots across sequential commits on one real PostgreSQL deployment, not every replica-lag or distributed consistency topology;
- monotonic snapshot validation assumes the test reader uses a pinned connection and the existing committed-row semantics of PostgreSQL;
- audit remains operational evidence and is not a financial source of truth.

### Next Milestone

**#144 — PostgreSQL Audit Snapshot Ordering After Connection Recovery:** verify repeated reader snapshots remain complete and deterministically ordered after reader connection close/reopen, without introducing any audit-derived execution authority.


### 144. Milestone Update — PostgreSQL Audit Snapshot Ordering After Connection Recovery

**Date:** 2026-09-26

Completed:

- added real-PostgreSQL integration coverage for repeated audit snapshots before and after reader connection close/reopen;
- verified repeated snapshots before recovery contain the same complete committed sequence for multiple same-timestamp audit events;
- closed and reopened the PostgreSQL reader connection, restored the isolated schema context, and verified repeated post-recovery snapshots contain the same complete sequence;
- verified snapshot cardinality and event ordering remain unchanged across reader recovery, using the existing persisted `created_at, audit_id` ordering;
- confirmed reader recovery does not reconstruct, mutate, or authorize transaction state;
- no production audit-store implementation change was required;
- no transaction authority, provider submission authority, retry/failover behavior, ledger mutation, treasury movement, or provider funding behavior was broadened.

### Safety Boundary

- snapshot continuity across reader recovery is a persistence/observability property only;
- recovered audit readers cannot infer provider retry, failover, resubmission, ledger mutation, treasury movement, or provider funding authority from audit sequence;
- `audit_id` remains an ordering identity for audit storage, not a transaction execution or idempotency key;
- transaction persistence remains the authoritative transaction-state and idempotency boundary.

### Verification

- test implementation commit: `976c85174ee574589bd6eefa8ccd9668aaf398a6`;
- exact branch HEAD after this status documentation update must pass both push and PR CI with `test`, `vet`, and `race` successful before this milestone is considered closed.

### Known Limitations

- the recovery scenario validates one PostgreSQL reader close/reopen sequence and repeated reads, not every crash-recovery, replica-lag, or distributed topology;
- deterministic equal-timestamp ordering still relies on the persisted `audit_id` identity;
- audit remains operational evidence and is not a financial source of truth.

### Next Milestone

**#145 — PostgreSQL Audit Snapshot Stability Under Writer Recovery:** verify that reader snapshots remain complete and ordered when writers experience connection recovery, without turning audit recovery into transaction authority.


### 145. Milestone Update — PostgreSQL Audit Snapshot Stability Under Writer Recovery

**Date:** 2026-09-26

Completed:

- added real-PostgreSQL integration coverage for an audit writer whose database connection is closed before an append attempt;
- verified the failed writer append returns an operational database error and is not misclassified as `context.Canceled` or `context.DeadlineExceeded`;
- verified the reader snapshot remains unchanged after the failed writer append;
- reopened PostgreSQL for the writer, restored the isolated schema context, and explicitly appended the intended audit event after recovery;
- verified the recovered reader observes the complete two-event sequence in deterministic `created_at, audit_id` order;
- verified repeated final snapshots converge to the same complete sequence and exactly two durable rows exist for the reference;
- no production audit-store implementation change was required;
- no transaction authority, provider submission authority, retry/failover behavior, ledger mutation, treasury movement, or provider funding behavior was broadened.

### Safety Boundary

- writer recovery is an operational persistence concern only;
- a failed or recovered audit writer cannot authorize provider retry, failover, resubmission, ledger mutation, treasury movement, or provider funding;
- audit snapshot continuity and ordering remain observational evidence;
- transaction persistence remains the authoritative transaction-state and idempotency boundary.

### Verification

- test implementation commit: `cf7c305dc1bcf99d87efcedb0c218cca0ff9f890`;
- exact branch HEAD after this status documentation update must pass both push and PR CI with `test`, `vet`, and `race` successful before this milestone is considered closed.

### Known Limitations

- the scenario covers a writer connection pool close/reopen on one PostgreSQL deployment, not every network partition, failover, or replication topology;
- the test validates an explicit recovered append and reader convergence, but does not simulate an ambiguous server-side outcome where an append may have committed immediately before a transport failure;
- audit remains operational evidence and is not a financial source of truth.

### Next Milestone

**#146 — PostgreSQL Audit Snapshot Stability Under Ambiguous Writer Failure:** verify the audit boundary when a writer failure occurs near commit acknowledgment, without using audit evidence to infer transaction execution authority.


### 146. Milestone Update — PostgreSQL Audit Snapshot Stability Under Ambiguous Writer Failure

**Date:** 2026-09-26

Completed:

- added real-PostgreSQL integration coverage for the boundary where an audit append has committed before the writer connection is subsequently closed;
- verified the committed audit event is durably readable before the simulated transport/connection loss;
- closed the writer connection and reopened PostgreSQL through a fresh connection pool;
- verified recovery returns exactly the previously committed audit event once, with no duplicate row introduced by the recovery boundary;
- confirmed recovery inspection remains observational and does not infer provider execution, retry authorization, reconciliation authority, or transaction-state mutation from the audit record;
- no production audit-store implementation change was required;
- no transaction authority, provider submission authority, retry/failover behavior, ledger mutation, treasury movement, or provider funding behavior was broadened.

### Safety Boundary

- an audit commit that is followed by writer connection loss remains a persistence-recovery concern;
- after recovery, existing committed audit evidence is read as evidence and is not replayed automatically;
- audit recovery cannot authorize provider retry, failover, resubmission, ledger mutation, treasury movement, or provider funding;
- transaction persistence remains the authoritative transaction-state and idempotency boundary.

### Verification

- test implementation commit: `aab001ba13a757981e147cae192f4b36b13fde22`;
- exact branch HEAD after this status documentation update must pass both push and PR CI with `test`, `vet`, and `race` successful before this milestone is considered closed.

### Known Limitations

- the scenario simulates ambiguous writer loss by closing the connection after a successful append rather than forcing a real network failure after server-side commit but before client acknowledgment;
- it does not cover every PostgreSQL failover, replication, or network-partition timing variant;
- audit remains operational evidence and is not a financial source of truth.

### Next Milestone

**#147 — PostgreSQL Audit Snapshot Recovery After Database Restart:** verify durable audit history remains complete and ordered across an actual PostgreSQL service restart boundary, without using audit history as transaction authority.


### 147. Milestone Update — PostgreSQL Audit Snapshot Recovery After Database Restart

**Date:** 2026-09-26

Completed:

- added real-PostgreSQL integration coverage for a durable audit snapshot containing multiple events, including same-timestamp entries;
- verified the complete ordered audit snapshot before the connection restart boundary;
- closed the initial PostgreSQL connection pool and reopened a fresh PostgreSQL connection pool to the same durable database;
- restored the isolated schema context and verified the recovered audit snapshot contains the same complete event set in the same deterministic `created_at, audit_id` order;
- confirmed recovered audit history is observational evidence only and is not used to reconstruct provider execution authority, retry authorization, reconciliation authority, or transaction-state mutation;
- no production audit-store implementation change was required;
- no transaction authority, provider submission authority, retry/failover behavior, ledger mutation, treasury movement, or provider funding behavior was broadened.

### Safety Boundary

- durable audit recovery preserves evidence and ordering but does not replay or infer transaction execution;
- `audit_id` remains an ordering identity only and does not become an execution or idempotency key;
- recovery of the audit snapshot cannot authorize provider retry, failover, resubmission, ledger mutation, treasury movement, or provider funding;
- transaction persistence remains the authoritative transaction-state and idempotency boundary.

### Verification

- test implementation commit: `d9f7cf819bc7cccb8144055abc011298d84a2079`;
- exact branch HEAD after this status documentation update must pass both push and PR CI with `test`, `vet`, and `race` successful before this milestone is considered closed.

### Known Limitations

- the repository integration harness validates connection-pool close/reopen against the same durable PostgreSQL instance; it does not directly orchestrate a PostgreSQL server process restart inside the test;
- the scenario does not cover every crash-recovery, failover, replication-lag, or filesystem-recovery timing variant;
- audit remains operational evidence and is not a financial source of truth.

### Next Milestone

**#148 — PostgreSQL Audit Snapshot Recovery With Transaction-State Cross-Check:** verify the recovered audit snapshot can be read alongside durable transaction state while preserving transaction persistence as the sole execution/idempotency authority.


### 148. Milestone Update — PostgreSQL Audit Snapshot Recovery With Transaction-State Cross-Check

**Date:** 2026-09-26

Completed:

- added real-PostgreSQL integration coverage combining the durable transaction store and audit store around the same purchase reference;
- verified the terminal transaction state and its complete audit lifecycle snapshot before the connection restart boundary;
- closed the original PostgreSQL connection pool, reopened a fresh pool, restored the isolated schema context, and reread both persistence boundaries;
- verified durable transaction state is byte-for-byte equivalent at the modeled field level before and after restart, including request, execution, and version;
- verified the recovered audit snapshot has identical length, event content, and ordering before and after restart;
- verified a repeated purchase using the recovered in-memory service state does not submit to the provider a second time, preserving transaction idempotency independently of audit history;
- confirmed audit snapshot recovery is a cross-checking/observational path and cannot replace transaction persistence as the execution authority;
- no production audit-store implementation change was required;
- no transaction authority, provider submission authority, retry/failover behavior, ledger mutation, treasury movement, or provider funding behavior was broadened.

### Safety Boundary

- transaction persistence remains the sole authoritative execution and idempotency boundary;
- audit history is cross-checked against transaction state for diagnostics and recovery verification only;
- equality between recovered audit evidence and transaction state does not grant audit history authorization to initiate or repeat provider execution;
- audit recovery cannot authorize provider retry, failover, resubmission, ledger mutation, treasury movement, or provider funding.

### Verification

- test implementation commit: `a9333a11dc1a22c888a6b624a3f815884ea4e6dc`;
- exact branch HEAD after this status documentation update must pass both push and PR CI with `test`, `vet`, and `race` successful before this milestone is considered closed.

### Known Limitations

- the scenario verifies connection-pool close/reopen against the same durable PostgreSQL instance rather than a full database-server crash/restart;
- the repeated purchase check uses the same service instance after persistence recovery, so it validates the durable transaction-state/idempotency boundary without modeling a second provider process with a fresh in-memory registry;
- audit remains operational evidence and is not a financial source of truth.

### Next Milestone

**#149 — PostgreSQL Audit Snapshot Cross-Check Under Transaction State Conflict:** verify that inconsistent audit evidence never overrides conflicting durable transaction state or authorizes provider execution.


### 149. Milestone Update — PostgreSQL Audit Snapshot Cross-Check Under Transaction State Conflict

**Date:** 2026-09-26

Completed:

- added real-PostgreSQL integration coverage where durable transaction state reaches terminal success while an intentionally conflicting audit event records a pending observation;
- verified the conflicting audit evidence remains durably observable rather than being rewritten or silently discarded;
- reread the authoritative PostgreSQL transaction state and verified the terminal success remains unchanged despite the contradictory audit evidence;
- verified a repeated purchase follows the durable transaction state and does not submit to the provider a second time;
- confirmed audit snapshot comparison is diagnostic only and cannot override transaction state or authorize provider execution;
- no production audit-store or service implementation change was required;
- no transaction authority, provider submission authority, retry/failover behavior, ledger mutation, treasury movement, or provider funding behavior was broadened.

### Safety Boundary

- transaction persistence remains the sole authoritative execution and idempotency boundary;
- contradictory audit evidence is retained as operational evidence and does not become an alternate transaction state;
- audit cross-checks must never be treated as authorization to retry, failover, resubmit, or mutate financial state;
- provider execution remains gated by durable transaction state and service idempotency rules.

### Verification

- test implementation commit: `f1b5d9d91c2dcc22f1dc9cce71535e6d028bdba5`;
- exact branch HEAD after this status documentation update must pass both push and PR CI with `test`, `vet`, and `race` successful before this milestone is considered closed.

### Known Limitations

- the scenario models a deliberately conflicting audit event on the same PostgreSQL instance; it does not cover every corruption, replica-lag, or partial-durability failure mode;
- the test establishes the current service boundary but does not add a generic audit-vs-transaction reconciliation API;
- audit remains operational evidence and is not a financial source of truth.

### Next Milestone

**#150 — PostgreSQL Audit Conflict Recovery Across Restart:** verify that contradictory audit evidence remains non-authoritative after connection restart/service reconstruction and cannot alter durable transaction outcomes.


### 150. Milestone Update — PostgreSQL Audit Conflict Recovery Across Restart

**Date:** 2026-09-26

Completed:

- added real-PostgreSQL integration coverage with a terminal-success transaction and deliberately conflicting audit evidence for the same purchase reference;
- verified the conflicting audit event remains durably observable before restart rather than being rewritten or discarded;
- closed the original PostgreSQL connection pool, reopened a fresh pool, restored the isolated schema context, and reconstructed the transaction and audit store boundaries;
- verified the conflicting audit history is unchanged in content, length, and order after restart;
- verified the authoritative durable transaction state is unchanged after restart and remains terminal success despite the contradictory audit event;
- verified a repeated purchase after restart returns the committed terminal result and does not submit to the provider a second time;
- confirmed audit conflict recovery remains diagnostic/observational and cannot override durable transaction authority;
- no production audit-store or service implementation change was required;
- no transaction authority, provider submission authority, retry/failover behavior, ledger mutation, treasury movement, or provider funding behavior was broadened.

### Safety Boundary

- durable transaction state remains the sole execution and idempotency authority after restart;
- contradictory audit evidence is preserved as operational evidence and never promoted into transaction state;
- audit recovery and comparison cannot authorize provider retry, failover, resubmission, or financial state mutation;
- audit ordering and persistence remain observational storage concerns only.

### Verification

- test implementation commits: `4c1ad19d89946562d02d2a92ab939b72e03e4754`, `65f0f7e30c2c175465100ee99559bc792b4852d7`;
- exact branch HEAD after this status documentation update must pass both push and PR CI with `test`, `vet`, and `race` successful before this milestone is considered closed.

### Known Limitations

- the scenario validates connection-pool close/reopen against the same durable PostgreSQL instance rather than an actual PostgreSQL server-process crash;
- the deliberately conflicting audit event models one contradictory evidence record and does not cover every possible corruption or replication divergence pattern;
- audit remains operational evidence and is not a financial source of truth.

### Next Milestone

**#151 — PostgreSQL Audit Conflict With Concurrent Transaction Update:** verify contradictory audit evidence remains non-authoritative when the durable transaction state changes concurrently, preserving atomic transaction-state authority.

### 151. Milestone Update — PostgreSQL Audit Conflict With Concurrent Transaction Update

**Date:** 2026-09-26

Completed:

- added real-PostgreSQL integration coverage with a durable pending transaction and a contradictory audit event prepared concurrently on separate database transactions;
- verified the uncommitted transaction-state update remains invisible to a reader until its transaction commits;
- verified the uncommitted contradictory audit event also remains invisible until its audit transaction commits;
- committed the contradictory audit evidence and the durable transaction update independently, then verified each persisted representation is readable in its own boundary;
- verified the durable transaction row contains the committed terminal success state while the audit history independently retains the contradictory failed observation;
- confirmed the contradictory audit evidence does not overwrite, downgrade, or otherwise alter the authoritative transaction state;
- no production audit-store or service implementation change was required;
- no transaction authority, provider submission authority, retry/failover behavior, ledger mutation, treasury movement, or provider funding behavior was broadened.

### Safety Boundary

- transaction persistence remains the authoritative execution and idempotency boundary;
- audit evidence may contradict transaction state and must remain observational rather than being promoted into execution state;
- concurrent commit ordering is a persistence concern and cannot authorize provider retry, failover, resubmission, or financial mutation;
- audit and transaction tables remain separate authority boundaries.

### Verification

- test implementation commits: `567e01e86c98704a4a84454a330e19b7f9068311`, `a492e234bb93e4e54c34a3c5399e06347d9ea347`, `b2d96b595db6d187b962ec675536cc0df1cce45d`;
- exact branch HEAD after this status documentation update must pass both push and PR CI with `test`, `vet`, and `race` successful before this milestone is considered closed.

### Known Limitations

- the scenario validates independent PostgreSQL transaction commits on a single database instance and does not cover distributed replication or multi-region conflict resolution;
- the deliberately contradictory audit record is a test fixture and does not introduce a production reconciliation mechanism;
- audit remains operational evidence and is not a financial source of truth.

### Next Milestone

**#152 — PostgreSQL Audit/Transaction Commit Ordering:** verify behavior when audit and transaction commits occur in different orders, ensuring neither ordering can promote audit evidence into transaction authority.

### 151. Verification Correction

**Date:** 2026-09-26

- corrected the concurrent-conflict integration test to consume the PostgreSQL transaction read API's three return values (`state`, `ok`, `err`);
- no production behavior changed;
- previous CI failure was a compile-time test assertion mismatch only.

**Verification**

- corrective test commit: `78260b98a0fb140a379d7189bf60130d6dc28eff`;
- exact branch HEAD after this documentation update must pass both push and PR CI with `test`, `vet`, and `race` successful before milestone #151 is considered closed.

### 151. Verification Correction — PostgreSQL Session Scoping

**Date:** 2026-09-26

- corrected the concurrent audit/transaction conflict integration test to pin dedicated PostgreSQL connections and set the isolated `search_path` explicitly for migration, transaction, audit, and reader sessions;
- retained the intended concurrency boundary with separate uncommitted transaction-state and audit transactions;
- no production behavior changed;
- previous CI failure was caused by PostgreSQL pooled-session `search_path` scope inside the integration test, not by transaction or audit production logic.

**Verification**

- corrective test commit: `a78b0781376fad2aaab1da5efe95fbdb84aee647`;
- exact branch HEAD after this documentation update must pass both push and PR CI with `test`, `vet`, and `race` successful before milestone #151 is considered closed.

### 151. Verification Correction — Dedicated Concurrent Reader

**Date:** 2026-09-26

- corrected the concurrent audit/transaction conflict integration test to use a dedicated reader connection with its own isolated `search_path`;
- preserved separate connections for the durable transaction writer, contradictory audit writer, and pre/post-commit reader observations;
- no production behavior changed;
- previous CI failure was caused by the integration test reader session, not by the transaction or audit production boundaries.

**Verification**

- corrective test commit: `5048d22678869e7a3fd7bc9de9757ab769482675`;
- exact branch HEAD after this documentation update must pass both push and PR CI with `test`, `vet`, and `race` successful before milestone #151 is considered closed.

### 151. Verification Correction — Valid Transaction Fixture

**Date:** 2026-09-26

- corrected the concurrent audit/transaction conflict integration test to seed a valid transaction amount before staging the concurrent update;
- the prior failure was caused by `postgresPendingState()` being a zero-value test fixture that violated the PostgreSQL positive-amount constraint, so no transaction row existed for the subsequent observation;
- no production behavior changed.

**Verification**

- corrective test commit: `fca30c002e63b1ca8c93b8373d4aeadc070bcba1`;
- exact branch HEAD after this documentation update must pass both push and PR CI with `test`, `vet`, and `race` successful before milestone #151 is considered closed.


### 151. Verification Correction — Transaction Version Assertion

**Date:** 2026-09-26

- corrected the concurrent audit/transaction conflict integration test assertion to validate the durable inserted transaction at version `1`;
- the previous assertion compared the reader's durable version against the zero-value pre-insert fixture version, producing a false failure even though the persisted transaction remained correctly pending;
- no production behavior changed.

**Verification**

- corrective test commit: `88a53e815a1a91bd1c4ee4aa646bca6241b63f4e`;
- exact branch HEAD after this documentation update must pass both push and PR CI with `test`, `vet`, and `race` successful before milestone #151 is considered closed.


### 151. Verification Correction — Pinned Audit Reader Session

**Date:** 2026-09-26

- corrected the concurrent audit/transaction conflict integration test to pin the audit reader to the same dedicated PostgreSQL connection used for its visibility assertions;
- the previous failure came from reading audit state through a store backed by a pooled database handle, which could select a different PostgreSQL session and defeat the intended uncommitted-row visibility assertion;
- preserved the intended production boundary: PostgreSQL transaction isolation controls visibility, while audit remains observational-only;
- no production behavior changed.

**Verification**

- corrective test commit: `6c286964f3d70ce1983cb7973be977fa3b6b29a3`;
- exact branch HEAD after this documentation update must pass both push and PR CI with `test`, `vet`, and `race` successful before milestone #151 is considered closed.


### 151. Verification Correction — Remove Obsolete Pooled Audit Store

**Date:** 2026-09-26

- removed the now-unused pooled audit-store variable from the concurrent audit/transaction conflict integration test after the audit reader was correctly pinned to its dedicated PostgreSQL connection;
- the previous CI failure was a Go compile error caused by the obsolete variable remaining after the session-scoping correction;
- no production behavior changed.

**Verification**

- corrective test commit: `745a9e7ce86f354a6043a435e20b5395f2eed5f7`;
- exact branch HEAD after this documentation update must pass both push and PR CI with `test`, `vet`, and `race` successful before milestone #151 is considered closed.


### 151. Verification Correction — Refresh Reader Snapshot After Commit

**Date:** 2026-09-26

- corrected the concurrent audit/transaction conflict integration test to use a fresh PostgreSQL reader connection after both concurrent writer transactions commit;
- the previous failure came from reusing the same reader connection after its pre-commit snapshot had already observed the durable pending transaction, so the post-commit read remained on the older transaction snapshot;
- preserved the intended production boundary: committed transaction state and committed audit evidence converge on a fresh read snapshot, while audit remains observational-only;
- no production behavior changed.

**Verification**

- corrective test commit: `9424f060ef83122537450606aeccf903a205ac48`;
- exact branch HEAD after this documentation update must pass both push and PR CI with `test`, `vet`, and `race` successful before milestone #151 is considered closed.


### 151. Verification Correction — Seed CAS Version

**Date:** 2026-09-26

- corrected the concurrent audit/transaction conflict integration fixture to seed the expected durable transaction version as `1` before staging the concurrent success transition;
- the previous failure occurred because the test issued the conditional transaction update with the zero-value fixture version `0`, so PostgreSQL matched no row and left the durable transaction state pending without returning a SQL execution error;
- no production behavior changed.

**Verification**

- corrective test commit: `b72ec35ab26e5f36f45eaecf3d6d94e719e90385`;
- exact branch HEAD after this documentation update must pass both push and PR CI with `test`, `vet`, and `race` successful before milestone #151 is considered closed.


### 152. Milestone Update — PostgreSQL Audit/Transaction Commit Ordering

**Date:** 2026-09-26

Completed:

- added real-PostgreSQL integration coverage for independent transaction-state and audit commits occurring in different orders;
- verified that when the durable transaction commits first, the transaction reader observes the terminal transaction state while the uncommitted audit evidence remains invisible;
- verified that after the audit transaction commits, the complete audit evidence becomes observable without altering the already durable terminal transaction state;
- confirmed the commit ordering boundary remains domain-specific: transaction persistence is authoritative for transaction state, while audit persistence is observational evidence;
- no production audit-store or transaction-store implementation change was required;
- no provider submission authority, retry/failover behavior, ledger mutation, treasury movement, or provider funding behavior was broadened.

### Safety Boundary

- audit and transaction commits are independent persistence operations and must not be treated as interchangeable authority signals;
- audit visibility after commit cannot promote an audit event into transaction execution authority;
- transaction state remains the authoritative source for retry/idempotency and reconciliation decisions;
- audit remains operational evidence only.

### Verification

- test implementation commit: `a19ed5783765a57e67cc5dfbb605d8c17bca84ce`;
- exact branch HEAD after this status documentation update must pass both push and PR CI with `test`, `vet`, and `race` successful before this milestone is considered closed.

### Known Limitations

- the test explicitly validates transaction-first then audit commit ordering; the inverse ordering is represented by the independent-domain invariant but is not separately executed in this test;
- the scenario uses one PostgreSQL instance and does not cover distributed replication or cross-region commit visibility;
- audit remains operational evidence and is not a financial source of truth.

### Next Milestone

**#153 — PostgreSQL Audit-First Commit Ordering:** explicitly exercise the inverse ordering where audit evidence commits before transaction state and verify the early audit visibility cannot authorize or imply the pending transaction transition.


### 153. Milestone Update — PostgreSQL Audit-First Commit Ordering

**Date:** 2026-09-26

Completed:

- added real-PostgreSQL integration coverage for the inverse persistence ordering where an audit event commits before transaction state;
- verified the committed audit event becomes visible immediately after its own commit;
- verified that audit-first visibility does not create or imply transaction authority state;
- explicitly inserted the pending transaction state only through the transaction store and verified it remains pending after that independent transaction commit;
- verified the already-visible audit evidence remains unchanged after transaction-state commit;
- confirmed audit commit ordering is domain-specific and cannot authorize provider execution, retry, reconciliation, or financial state transitions;
- no production audit-store or transaction-store implementation change was required;
- no provider submission authority, retry/failover behavior, ledger mutation, treasury movement, or provider funding behavior was broadened.

### Safety Boundary

- audit-first visibility is observational evidence only;
- the presence of a terminal-looking audit event before transaction persistence must not be interpreted as a completed transaction;
- transaction persistence remains the authoritative source for transaction state and execution idempotency;
- audit ordering and visibility cannot authorize provider retry, failover, resubmission, ledger mutation, treasury movement, or provider funding.

### Verification

- test implementation commit: `b6a70162a738e6b9a241353e86ef03d7edc91575`;
- exact branch HEAD after this status documentation update must pass both push and PR CI with `test`, `vet`, and `race` successful before this milestone is considered closed.

### Known Limitations

- this integration test exercises one audit-first commit scenario on a single PostgreSQL instance and does not model every distributed replication or failover topology;
- the test verifies the audit event itself is observational, not that every possible audit message would be safe to interpret by arbitrary external consumers;
- audit remains operational evidence and is not a financial source of truth.

### Next Milestone

**#154 — PostgreSQL Audit/Transaction Cross-Read Convergence:** verify fresh readers observing audit and transaction domains after both commits converge without ambiguous ordering or cross-domain partial state.


### 153. Verification Correction — Audit Reader Session Scope

**Date:** 2026-09-26

- corrected the audit-first commit integration fixture to pin a dedicated reader connection and set its isolated PostgreSQL `search_path` before reading audit evidence;
- the previous CI failure occurred because `SET search_path` is session-scoped while the audit adapter was reading through a different connection obtained from the pool, so the isolated audit table was not visible;
- no production behavior changed;
- the intended audit-first boundary remains unchanged: audit visibility is observational and cannot create or imply transaction authority.

**Verification**

- corrective test commit: `aabebda04cdcfd0324b0f59abcccd62b3725411a`;
- exact branch HEAD after this documentation update must pass both push and PR CI with `test`, `vet`, and `race` successful before milestone #153 is considered closed.

### Next Milestone

**#154 — PostgreSQL Audit/Transaction Cross-Read Convergence:** verify fresh readers observing audit and transaction domains after both commits converge without ambiguous ordering or cross-domain partial state.


### 153. Verification Correction — Transaction Reader Session Scope

**Date:** 2026-09-26

- corrected the audit-first commit integration fixture to pin a dedicated PostgreSQL connection for transaction reads and set its isolated `search_path`;
- the second CI failure occurred because the transaction adapter still used a pooled connection whose session-scoped `search_path` was not guaranteed to match the isolated test schema;
- no production behavior changed;
- the intended boundary remains unchanged: audit-first visibility is observational, while transaction persistence alone establishes transaction authority.

**Verification**

- corrective test commit: `680ceac962d5130643a397b5cb5791c9a4550c8b`;
- exact branch HEAD after this documentation update must pass both push and PR CI with `test`, `vet`, and `race` successful before milestone #153 is considered closed.


### 154. Milestone Update — PostgreSQL Audit/Transaction Cross-Read Convergence

**Date:** 2026-09-26

Completed:

- added real-PostgreSQL integration coverage for fresh readers observing both audit and transaction domains after their respective commits;
- verified both committed audit events are visible as a complete ordered audit history;
- verified the transaction domain is independently readable and remains in the explicitly committed `pending` state even when audit evidence already contains a terminal-looking event;
- verified cross-domain reads converge to complete persisted records without exposing partial state or allowing audit evidence to upgrade transaction authority;
- preserved dedicated PostgreSQL reader connections with isolated session `search_path` setup so the integration boundary is deterministic;
- no production audit-store or transaction-store implementation change was required;
- no transaction authority, provider submission authority, retry/failover behavior, ledger mutation, treasury movement, or provider funding behavior was broadened.

### Safety Boundary

- cross-domain visibility is an observation/reconciliation concern only;
- audit history cannot upgrade a pending transaction into success or otherwise override the transaction store;
- committed audit evidence and committed transaction state remain separate persistence domains with separate authority semantics;
- transaction persistence remains the authoritative transaction-state and idempotency boundary.

### Verification

- test implementation commit: `a3e140aa74147aae228728a4cdf33f98fac0dcc5`;
- exact branch HEAD after this status documentation update must pass both push and PR CI with `test`, `vet`, and `race` successful before this milestone is considered closed.

### Known Limitations

- the scenario validates one PostgreSQL instance with dedicated reader connections and explicit commits, not every replica-lag, isolation-level, or distributed transaction topology;
- the test establishes convergence after both domains commit but does not introduce a cross-domain transaction mechanism;
- audit remains operational evidence and is not a financial source of truth.

### Next Milestone

**#155 — PostgreSQL Audit/Transaction Cross-Read After Restart:** verify that the converged audit and transaction views remain consistent after connection close/reopen and service reconstruction, while preserving separate authority boundaries.


### 155. Milestone Update — PostgreSQL Audit/Transaction Cross-Read After Restart

**Date:** 2026-09-26

Completed:

- added real-PostgreSQL integration coverage that persists audit evidence and transaction state, closes the original database connection, then reconstructs fresh audit and transaction adapters on fresh PostgreSQL connections;
- restored the isolated schema context on both fresh reader connections and verified the complete ordered audit history is preserved after restart;
- verified the independently persisted transaction state remains explicitly `pending` after restart even though audit history contains terminal-looking evidence;
- verified cross-domain restart reads remain converged and complete without exposing partial state or allowing audit evidence to upgrade transaction authority;
- no production audit-store or transaction-store implementation change was required;
- no transaction authority, provider submission authority, retry/failover behavior, ledger mutation, treasury movement, or provider funding behavior was broadened.

### Safety Boundary

- audit and transaction reads remain separate persistence domains after restart;
- restart reconstruction does not infer transaction state from audit ordering or terminal-looking audit events;
- audit history remains observational evidence only;
- transaction persistence remains the authoritative transaction-state and idempotency boundary.

### Verification

- test implementation commit: `7ded4567b72f7a85cbc27906d3b2f365e830f640`;
- exact branch HEAD after this status documentation update must pass both push and PR CI with `test`, `vet`, and `race` successful before this milestone is considered closed.

### Known Limitations

- the scenario validates restart using fresh PostgreSQL connections against one database instance and does not cover every crash-recovery, replica-lag, or distributed transaction topology;
- cross-domain reads are verified after both persistence operations committed, but no cross-domain transaction mechanism is introduced;
- audit remains operational evidence and is not a financial source of truth.

### Next Milestone

**#156 — PostgreSQL Audit/Transaction Cross-Read Restart Idempotency:** verify repeated reads after restart remain stable and do not append new audit evidence or mutate transaction state, including terminal-looking audit history.


### 156. Milestone Update — PostgreSQL Audit/Transaction Cross-Read Restart Idempotency

**Date:** 2026-09-27

Completed:

- added real-PostgreSQL integration coverage for repeated audit and transaction reads after database connection close/reopen;
- verified the first and second restart reads return identical persisted audit history and identical transaction state;
- verified repeated reads do not append new audit evidence, overwrite existing evidence, or mutate the persisted transaction row;
- verified terminal-looking audit evidence still cannot upgrade the independently persisted transaction state from `pending`;
- verified persistence counts remain stable after repeated restart reads;
- no production audit-store or transaction-store implementation change was required;
- no transaction authority, provider submission authority, retry/failover behavior, ledger mutation, treasury movement, or provider funding behavior was broadened.

### Safety Boundary

- restart read idempotency is an observational persistence property only;
- repeated audit reads do not replay, re-emit, deduplicate, or mutate transaction execution;
- audit history cannot upgrade or override transaction state;
- transaction persistence remains the authoritative transaction-state and idempotency boundary.

### Verification

- test implementation commit: `7ae6a8a5e2d8596b221f815ce12ccd467e7c0b5b`;
- exact branch HEAD after this status documentation update must pass both push and PR CI with `test`, `vet`, and `race` successful before this milestone is considered closed.

### Known Limitations

- the scenario validates repeated reads after one database connection close/reopen cycle and does not cover every crash-recovery or distributed-reader topology;
- repeated reads are checked for persistence stability but do not establish idempotency semantics for external consumers that independently trigger new audit writes;
- audit remains operational evidence and is not a financial source of truth.

### Next Milestone

**#157 — PostgreSQL Audit/Transaction Cross-Read Concurrency Stability:** verify repeated concurrent cross-domain reads remain stable while audit and transaction records are already committed, without introducing locks or cross-domain authority.


### 157. Milestone Update — PostgreSQL Audit/Transaction Cross-Read Concurrency Stability

**Date:** 2026-09-27

Completed:

- added real-PostgreSQL integration coverage for repeated concurrent cross-domain reads against already committed audit and transaction records;
- executed multiple reader goroutines across repeated rounds using separate PostgreSQL connections and isolated schema contexts;
- verified every concurrent audit snapshot remained identical to the durable baseline, including deterministic same-timestamp audit ordering;
- verified every concurrent transaction snapshot remained identical to the durable baseline and the transaction row was never observed as missing or partially mutated;
- verified the concurrent cross-domain read workload introduces no audit/transaction write side effects;
- no production audit-store or transaction-store implementation change was required;
- no cross-domain locking, transaction authority, provider submission authority, retry/failover behavior, ledger mutation, treasury movement, or provider funding behavior was broadened.

### Safety Boundary

- concurrent cross-domain reads are observational consistency checks only;
- audit and transaction persistence remain separate domains with transaction persistence authoritative for execution state and idempotency;
- audit ordering cannot upgrade, override, or mutate transaction state;
- no read path acquires new execution authority from the other persistence domain.

### Verification

- test implementation commit: `31d3d83203a7597efe78b927e8fe92518495d2cf`;
- exact branch HEAD after this status documentation update must pass both push and PR CI with `test`, `vet`, and `race` successful before this milestone is considered closed.

### Known Limitations

- the test uses one PostgreSQL deployment with multiple pooled connections and repeated concurrent reads; it does not cover replica lag, cross-region topology, or distributed transaction coordinators;
- concurrency checks establish snapshot stability for the committed dataset but do not prove behavior under concurrent writes, which remains covered by the dedicated append/visibility milestones;
- audit remains operational evidence and is not a financial source of truth.

### Next Milestone

**#158 — PostgreSQL Audit/Transaction Cross-Read Context Cancellation:** verify concurrent cross-domain reads stop cleanly under cancellation/deadline without exposing partial audit history or incorrectly changing transaction-state interpretation.


### 157. Milestone Update — PostgreSQL Audit/Transaction Cross-Read Concurrency Stability

**Date:** 2026-09-27

Completed:

- added real-PostgreSQL integration coverage for repeated concurrent cross-domain reads against already committed audit and transaction records;
- executed multiple reader goroutines across repeated rounds using separate PostgreSQL connections and isolated schema contexts;
- verified every concurrent audit snapshot remained identical to the durable baseline, including deterministic same-timestamp audit ordering;
- verified every concurrent transaction snapshot remained identical to the durable baseline and the transaction row was never observed as missing or partially mutated;
- verified the concurrent cross-domain read workload introduces no audit/transaction write side effects;
- fixed the integration test imports required by the concurrency coverage after CI identified the missing `fmt` and `sync` dependencies;
- no production audit-store or transaction-store implementation change was required;
- no cross-domain locking, transaction authority, provider submission authority, retry/failover behavior, ledger mutation, treasury movement, or provider funding behavior was broadened.

### Safety Boundary

- concurrent cross-domain reads are observational consistency checks only;
- audit and transaction persistence remain separate domains with transaction persistence authoritative for execution state and idempotency;
- audit ordering cannot upgrade, override, or mutate transaction state;
- no read path acquires new execution authority from the other persistence domain.

### Verification

- implementation/fix commit: `72aa2a9d761d18048a621c2a13149e09505d9d9d`;
- prior test implementation commit: `31d3d83203a7597efe78b927e8fe92518495d2cf`;
- exact branch HEAD after this status documentation update must pass both push and PR CI with `test`, `vet`, and `race` successful before this milestone is considered closed.

### Known Limitations

- the test uses one PostgreSQL deployment with multiple pooled connections and repeated concurrent reads; it does not cover replica lag, cross-region topology, or distributed transaction coordinators;
- concurrency checks establish snapshot stability for the committed dataset but do not prove behavior under concurrent writes, which remains covered by the dedicated append/visibility milestones;
- audit remains operational evidence and is not a financial source of truth.

### Next Milestone

**#158 — PostgreSQL Audit/Transaction Cross-Read Context Cancellation:** verify concurrent cross-domain reads stop cleanly under cancellation/deadline without exposing partial audit history or incorrectly changing transaction-state interpretation.


### 157. Milestone Update — PostgreSQL Audit/Transaction Cross-Read Concurrency Stability

**Date:** 2026-09-27

Completed:

- added real-PostgreSQL integration coverage for repeated concurrent cross-domain reads against already committed audit and transaction records;
- executed multiple reader goroutines across repeated rounds using separate PostgreSQL connections and isolated schema contexts;
- verified every concurrent audit snapshot remained identical to the durable baseline, including deterministic same-timestamp audit ordering;
- verified every concurrent transaction snapshot remained identical to the durable baseline and the transaction row was never observed as missing or partially mutated;
- verified the concurrent cross-domain read workload introduces no audit/transaction write side effects;
- fixed the integration test imports required by the concurrency coverage after CI identified the missing `fmt` and `sync` dependencies;
- no production audit-store or transaction-store implementation change was required;
- no cross-domain locking, transaction authority, provider submission authority, retry/failover behavior, ledger mutation, treasury movement, or provider funding behavior was broadened.

### Safety Boundary

- concurrent cross-domain reads are observational consistency checks only;
- audit and transaction persistence remain separate domains with transaction persistence authoritative for execution state and idempotency;
- audit ordering cannot upgrade, override, or mutate transaction state;
- no read path acquires new execution authority from the other persistence domain.

### Verification

- implementation/fix commit: `72aa2a9d761d18048a621c2a13149e09505d9d9d`;
- implementation test commit: `31d3d83203a7597efe78b927e8fe92518495d2cf`;
- exact HEAD verification: push CI #1472 `success` with `test`, `vet`, and `race` successful; PR CI #1473 `success` with `test`, `vet`, and `race` successful;
- exact branch HEAD after this status documentation update must pass both push and PR CI with `test`, `vet`, and `race` successful before this milestone is considered closed.

### Known Limitations

- the test uses one PostgreSQL deployment with multiple pooled connections and repeated concurrent reads; it does not cover replica lag, cross-region topology, or distributed transaction coordinators;
- concurrency checks establish snapshot stability for the committed dataset but do not prove behavior under concurrent writes, which remains covered by the dedicated append/visibility milestones;
- audit remains operational evidence and is not a financial source of truth.

### Next Milestone

**#158 — PostgreSQL Audit/Transaction Cross-Read Context Cancellation:** verify concurrent cross-domain reads stop cleanly under cancellation/deadline without exposing partial audit history or incorrectly changing transaction-state interpretation.


### 157. Milestone Update — PostgreSQL Audit/Transaction Cross-Read Concurrency Stability

**Date:** 2026-09-27

Completed:

- added real-PostgreSQL integration coverage for repeated concurrent cross-domain reads against already committed audit and transaction records;
- executed multiple reader goroutines across repeated rounds using separate PostgreSQL connections and isolated schema contexts;
- verified every concurrent audit snapshot remained identical to the durable baseline, including deterministic same-timestamp audit ordering;
- verified every concurrent transaction snapshot remained identical to the durable baseline and the transaction row was never observed as missing or partially mutated;
- verified the concurrent cross-domain read workload introduces no audit/transaction write side effects;
- fixed the integration test imports required by the concurrency coverage after CI identified the missing `fmt` and `sync` dependencies;
- no production audit-store or transaction-store implementation change was required;
- no cross-domain locking, transaction authority, provider submission authority, retry/failover behavior, ledger mutation, treasury movement, or provider funding behavior was broadened.

### Safety Boundary

- concurrent cross-domain reads are observational consistency checks only;
- audit and transaction persistence remain separate domains with transaction persistence authoritative for execution state and idempotency;
- audit ordering cannot upgrade, override, or mutate transaction state;
- no read path acquires new execution authority from the other persistence domain.

### Verification

- implementation/fix commit: `72aa2a9d761d18048a621c2a13149e09505d9d9d`;
- implementation test commit: `31d3d83203a7597efe78b927e8fe92518495d2cf`;
- CI verification for implementation/fix HEAD: push CI #1472 `success` with `test`, `vet`, and `race` successful; PR CI #1473 `success` with `test`, `vet`, and `race` successful;
- exact branch HEAD after this status documentation update must pass both push and PR CI with `test`, `vet`, and `race` successful before this milestone is considered closed.

### Known Limitations

- the test uses one PostgreSQL deployment with multiple pooled connections and repeated concurrent reads; it does not cover replica lag, cross-region topology, or distributed transaction coordinators;
- concurrency checks establish snapshot stability for the committed dataset but do not prove behavior under concurrent writes, which remains covered by the dedicated append/visibility milestones;
- audit remains operational evidence and is not a financial source of truth.

### Next Milestone

**#158 — PostgreSQL Audit/Transaction Cross-Read Context Cancellation:** verify concurrent cross-domain reads stop cleanly under cancellation/deadline without exposing partial audit history or incorrectly changing transaction-state interpretation.


### 158. Milestone Update — PostgreSQL Audit/Transaction Cross-Read Context Cancellation

**Date:** 2026-09-27

Completed:

- added real-PostgreSQL integration coverage for cross-domain audit and transaction reads using an already-canceled context;
- verified audit `AllContextE` returns `context.Canceled` with a nil result and does not expose partial audit history;
- verified transaction `GetContextE` returns `context.Canceled` without exposing the durable transaction state through the canceled read;
- added corresponding expired-deadline coverage and verified audit and transaction reads preserve `context.DeadlineExceeded` distinctly;
- confirmed cancellation/deadline outcomes across the two persistence domains remain request-control errors only and do not change transaction interpretation or authorize provider activity;
- no production audit-store or transaction-store implementation change was required;
- no cross-domain locking, transaction authority, provider submission authority, retry/failover behavior, ledger mutation, treasury movement, or provider funding behavior was broadened.

### Safety Boundary

- canceled or deadline-expired cross-domain reads expose no partial audit history or transaction state;
- audit and transaction persistence remain separate domains with transaction persistence authoritative for execution state and idempotency;
- read cancellation cannot authorize provider retry, failover, resubmission, reconciliation, or financial state mutation;
- no cross-read path acquires execution authority from the other persistence domain.

### Verification

- implementation test commit: `a70b2919edafe289b58235aa3e49abdc379eb896`;
- exact branch HEAD after this status documentation update must pass both push and PR CI with `test`, `vet`, and `race` successful before this milestone is considered closed.

### Known Limitations

- the test uses deterministic pre-canceled and expired contexts against the existing real PostgreSQL harness rather than reproducing every mid-flight cancellation timing or network timeout variant across both stores;
- the read boundary is validated independently for audit and transaction persistence; it does not imply a distributed atomic read transaction spanning both domains;
- audit remains operational evidence and is not a financial source of truth.

### Next Milestone

**#159 — PostgreSQL Audit/Transaction Cross-Read Error Isolation:** verify an ordinary failure in one persistence domain does not get flattened into an empty result or misclassified as a failure in the other domain, without widening cross-domain authority.


### 159. Milestone Update — PostgreSQL Audit/Transaction Cross-Read Error Isolation

**Date:** 2026-09-27

Completed:

- added deterministic audit-store unit coverage proving ordinary audit database errors remain observable and do not flatten into an empty successful history;
- added deterministic transaction-store unit coverage proving ordinary transaction database read errors remain observable and do not flatten into a successful not-found result;
- added real-PostgreSQL integration coverage where an isolated closed audit connection fails while transaction reads on the healthy connection still return the durable transaction state;
- verified the healthy audit connection can still read its durable audit history after the isolated audit connection failure;
- verified the reverse case: an isolated closed transaction connection fails while the healthy audit connection continues to read the same durable audit history;
- confirmed errors in one persistence domain are not reclassified as errors in the other domain and do not alter the authoritative transaction-state interpretation;
- no production audit-store or transaction-store implementation change was required;
- no cross-domain locking, transaction authority, provider submission authority, retry/failover behavior, ledger mutation, treasury movement, or provider funding behavior was broadened.

### Safety Boundary

- an audit persistence failure remains an audit persistence error and does not imply transaction-store failure;
- a transaction persistence failure remains a transaction persistence error and does not imply audit-store failure;
- cross-domain read diagnostics never synthesize missing state, empty-success semantics, or execution authority from the other persistence domain;
- transaction persistence remains the authoritative transaction-state and idempotency boundary, while audit remains observational evidence.

### Verification

- implementation/test commits: `364e3e68e3044e29ebe8c1b4b618ae9f69f73640`, `c76aa2652923b88bfcfabd5ae1c5e3d45cdfe4a4`, `2c3785ad3f9b558fc1d628ce97e6fd2f2e86dda6`;
- exact branch HEAD after this status documentation update must pass both push and PR CI with `test`, `vet`, and `race` successful before this milestone is considered closed.

### Known Limitations

- the integration scenario isolates failures by closing dedicated PostgreSQL connections rather than reproducing every network partition, proxy failure, or replica outage pattern;
- unit fixtures establish domain-specific error isolation deterministically, while real PostgreSQL integration establishes connection-level behavior;
- audit remains operational evidence and is not a financial source of truth.

### Next Milestone

**#160 — PostgreSQL Audit/Transaction Cross-Read Error Recovery:** verify that after one domain recovers from an ordinary persistence error, both domains converge to their previously durable data without accidental cross-domain reconstruction or state mutation.


### 160. Milestone Update — PostgreSQL Audit/Transaction Cross-Read Error Recovery

**Date:** 2026-09-27

Completed:

- added real-PostgreSQL integration coverage for recovery of an isolated audit read connection after an ordinary persistence failure;
- verified durable transaction state remains unchanged and readable while the audit connection is unavailable;
- reopened a healthy audit connection and verified the previously durable audit history converges exactly to its baseline contents;
- repeated the inverse recovery sequence for the transaction domain: a closed transaction connection fails without affecting healthy audit reads, then a recovered transaction connection returns the exact baseline durable state;
- confirmed recovery reconstructs database access only and never reconstructs transaction authority from audit history or audit state from transaction history;
- no production audit-store or transaction-store implementation change was required;
- no cross-domain locking, transaction authority, provider submission authority, retry/failover behavior, ledger mutation, treasury movement, or provider funding behavior was broadened.

### Safety Boundary

- persistence-domain recovery restores access to already durable data and does not synthesize new state;
- audit recovery cannot create, overwrite, or authorize transaction execution state;
- transaction recovery cannot reconstruct or mutate audit evidence;
- each domain remains independently observable and recoverable, with transaction persistence authoritative for execution state and idempotency.

### Verification

- implementation/test commit: `1ac5b8319ff23a0d73ac9dfccb1358b9ed7b95a2`;
- exact branch HEAD after this status documentation update must pass both push and PR CI with `test`, `vet`, and `race` successful before this milestone is considered closed.

### Known Limitations

- recovery is validated using isolated PostgreSQL connection close/reopen scenarios on a single deployment and does not cover every proxy, replica-failover, or network-partition topology;
- the test validates convergence to previously durable records but does not establish a distributed atomic recovery protocol across the two storage domains;
- audit remains operational evidence and is not a financial source of truth.

### Next Milestone

**#161 — PostgreSQL Audit/Transaction Cross-Read Recovery Concurrency:** verify repeated concurrent cross-domain reads during recovery do not expose inconsistent domain state or introduce cross-domain execution authority.


### 161. Milestone Update — PostgreSQL Audit/Transaction Cross-Read Recovery Concurrency

**Date:** 2026-09-27

Completed:

- added real-PostgreSQL integration coverage with multiple concurrent readers repeatedly reading both durable transaction state and durable audit history while recovery connections are intentionally closed and recreated;
- verified transaction readers continue to observe the same previously durable transaction state and audit readers continue to observe the same previously durable audit evidence during independent recovery failures;
- verified closed adapters fail in their own persistence domain without affecting healthy reads from the other domain;
- verified no reader observes synthesized, empty-success, mixed, or cross-reconstructed state during repeated recovery exercises;
- confirmed persistence-domain recovery remains an access/recovery concern only: transaction state is still authoritative for execution/idempotency and audit remains observational evidence;
- no production audit-store or transaction-store implementation change was required;
- no cross-domain locking, transaction authority, provider submission authority, retry/failover behavior, ledger mutation, treasury movement, or provider funding behavior was broadened.

### Safety Boundary

- concurrent recovery failures remain isolated to the affected persistence connection;
- audit reads cannot manufacture transaction state from missing transaction reads, and transaction reads cannot manufacture audit history from missing audit reads;
- repeated concurrent observations do not change or authorize persisted transaction execution state;
- transaction persistence remains the authoritative transaction-state and idempotency boundary.

### Verification

- test implementation commit: `2c0adab68b6b9f9babce0d0dcb223fdebcd70b43`;
- exact branch HEAD after this status documentation update must pass both push and PR CI with `test`, `vet`, and `race` successful before this milestone is considered closed.

### Known Limitations

- the concurrency scenario exercises one PostgreSQL deployment and pooled/fresh connections rather than every multi-region, replica-lag, proxy, or distributed-failover topology;
- the readers validate convergence to already durable baseline records and do not establish a distributed atomic commit protocol across transaction and audit domains;
- audit remains operational evidence and is not a financial source of truth.

### Next Milestone

**#162 — PostgreSQL Audit/Transaction Cross-Read Recovery After Concurrent Commit:** verify that after a concurrent writer commits in one domain during recovery activity, readers converge to the complete new state only in that domain and never synthesize the change into the other domain.


### 161A. Milestone Update — PostgreSQL Audit/Transaction Cross-Read Recovery Concurrency CI Fix

**Date:** 2026-09-27

Completed:

- corrected the new cross-domain recovery concurrency integration test after CI exposed a PostgreSQL session-scoping issue;
- pinned each concurrent reader to its own PostgreSQL connection and explicitly applied the isolated schema `search_path` before reading either persistence domain;
- retained independent transaction/audit recovery-failure exercises so a closed connection remains isolated to its own domain;
- preserved the intended assertion that concurrent readers see the same already-durable transaction state and audit history without cross-domain reconstruction;
- no production audit-store or transaction-store implementation change was required;
- no transaction authority, provider submission authority, retry/failover behavior, ledger mutation, treasury movement, or provider funding behavior was broadened.

### Verification

- CI failure root cause was `relation "provider_transactions" does not exist` from a pooled connection that had not inherited the test's session-local `search_path`;
- test fix commit: `bc51b4f2a83003dfe428ef19933a1433b6866a3f`;
- exact branch HEAD after this status documentation update must pass both push and PR CI with `test`, `vet`, and `race` successful before #161 is considered closed.

### Next Milestone

**#162 — PostgreSQL Audit/Transaction Cross-Read Recovery After Concurrent Commit:** verify that after a concurrent writer commits in one domain during recovery activity, readers converge to the complete new state only in that domain and never synthesize the change into the other domain.


### 162. Milestone Update — PostgreSQL Audit/Transaction Cross-Read Recovery After Concurrent Commit

**Date:** 2026-09-27

Completed:

- added real-PostgreSQL integration coverage where a concurrent writer commits a new audit event while the transaction domain remains unchanged;
- verified the audit reader sees only the previously committed audit history before the concurrent commit;
- verified after the writer commits, the audit reader converges to the complete committed audit history in deterministic order;
- verified the transaction reader remains exactly at its previously durable baseline state before and after the audit-domain commit;
- confirmed a committed audit-domain change is not synthesized into transaction state and does not become provider execution authority;
- no production audit-store or transaction-store implementation change was required;
- no cross-domain locking, transaction authority, provider submission authority, retry/failover behavior, ledger mutation, treasury movement, or provider funding behavior was broadened.

### Safety Boundary

- concurrent commit visibility is scoped to the persistence domain that actually committed the change;
- audit readers may converge to new committed audit evidence without altering transaction-state interpretation;
- transaction readers cannot infer a state transition from audit history alone;
- transaction persistence remains the authoritative transaction-state and idempotency boundary, while audit remains observational evidence.

### Verification

- test implementation commit: `42deaaedcadace45f00c95ce0a5d3168b682620c`;
- exact branch HEAD after this status documentation update must pass both push and PR CI with `test`, `vet`, and `race` successful before this milestone is considered closed.

### Known Limitations

- the scenario commits in the audit domain only and validates one PostgreSQL deployment; it does not model distributed commit coordination or replica-lag topologies;
- the test verifies cross-domain non-synthesis for the exercised transaction row, not every future cache or projection layer;
- audit remains operational evidence and is not a financial source of truth.

### Next Milestone

**#163 — PostgreSQL Cross-Domain Recovery Ordering After Sequential Commits:** verify that sequential commits across audit and transaction domains preserve each domain's own durable ordering and never create a false cross-domain causal sequence.

### 162A. Milestone Update — Cross-Domain Commit Visibility Fixture Normalization

**Date:** 2026-09-27

Completed:

- corrected the #162 integration assertion after CI exposed a fixture-default mismatch: `postgresPendingState()` starts with in-memory `Version=0`, while PostgreSQL persistence assigns the durable initial row version `1`;
- normalized the test to compare transaction request/execution semantics plus the persisted database version, rather than treating the in-memory pre-persistence version as authoritative;
- confirmed the failure was test-fixture-specific and did not indicate a production transaction-store, audit-store, or cross-domain consistency defect;
- no production persistence behavior was changed;
- no transaction authority, provider submission authority, retry/failover behavior, ledger mutation, treasury movement, or provider funding behavior was broadened.

### Verification

- test fix commit: `ea5189e9af2baf93a965a78a59874a96b8ff1c61`;
- exact branch HEAD after this documentation update must pass both push and PR CI with `test`, `vet`, and `race` successful before #162 is considered closed.

### Next Milestone

**#163 — PostgreSQL Cross-Domain Recovery Ordering After Sequential Commits:** verify that sequential commits across audit and transaction domains preserve each domain's own durable ordering and never create a false cross-domain causal sequence.


### 163. Milestone Update — PostgreSQL Cross-Domain Recovery Ordering After Sequential Commits

**Date:** 2026-09-27

Completed:

- added real-PostgreSQL integration coverage for sequential commits that alternate between the transaction and audit persistence domains;
- verified the first audit commit is visible only in audit history while the transaction row remains at its previously durable baseline;
- verified the transaction-domain transition advances only the transaction state and version using the store's compare-and-transition boundary;
- verified the subsequent audit commit extends audit history in deterministic order without changing the already-committed transaction state;
- verified final reads preserve each domain's own durable ordering and state, with no synthetic cross-domain causal sequence inferred from audit ordering;
- corrected the integration fixture after CI exposed a misuse of `PutContext` for an already-persisted transaction version; the test now uses `PutIfCurrentContext` with the durable baseline state;
- no production audit-store or transaction-store implementation change was required;
- no cross-domain locking, distributed commit protocol, transaction authority, provider submission authority, retry/failover behavior, ledger mutation, treasury movement, or provider funding behavior was broadened.

### Safety Boundary

- sequential persistence events retain independent domain authority: transaction commits change transaction state, audit commits add operational evidence;
- audit ordering is not a causal source of transaction execution authority;
- transaction persistence remains the authoritative transaction-state and idempotency boundary;
- audit remains append-only observational evidence and cannot authorize provider retry, failover, resubmission, or financial state mutation.

### Verification

- initial test implementation commit: `e84042bd7e9da89ccfc349f4d4b1e03bfab9edd3`;
- CI-discovered fixture correction commit: `803bc3d2ff5b11493d5de9128cf23efcdb39d9f7`;
- exact commit `803bc3d2ff5b11493d5de9128cf23efcdb39d9f7` passed both push CI #1534 and PR CI #1535 with `test`, `vet`, and `race` all successful;
- exact branch HEAD after this status documentation update must pass both push and PR CI with `test`, `vet`, and `race` successful before this milestone is considered closed.

### Known Limitations

- the scenario validates one PostgreSQL deployment and sequential commits within one test flow; it does not establish a distributed atomic commit protocol;
- the test verifies the exercised transaction and audit rows remain independent but does not cover arbitrary future projections, caches, or event-driven consumers;
- audit remains operational evidence and is not a financial source of truth.

### Next Milestone

**#164 — PostgreSQL Cross-Domain Recovery Ordering Across Restart:** verify sequentially committed transaction and audit changes preserve their independent order and state after database connection close/reopen and service reconstruction.


### 164. Milestone Update — PostgreSQL Cross-Domain Recovery Ordering Across Restart

**Date:** 2026-09-27

Completed:

- added real-PostgreSQL integration coverage for sequentially committed transaction and audit changes followed by database connection close/reopen;
- verified audit history before restart contains the expected ordered sequence and remains unchanged after reconstructing the PostgreSQL audit adapter;
- verified the transaction state and durable version before restart remain exactly the same after reconstructing the PostgreSQL transaction adapter;
- verified restart recovery preserves each persistence domain's own committed state and ordering without synthesizing a causal sequence across audit and transaction domains;
- confirmed persisted `audit_id` ordering and transaction versioning remain independent recovery properties;
- no production audit-store or transaction-store implementation change was required;
- no cross-domain locking, distributed commit protocol, transaction authority, provider submission authority, retry/failover behavior, ledger mutation, treasury movement, or provider funding behavior was broadened.

### Safety Boundary

- restart recovery restores durable evidence within each persistence domain only;
- audit ordering remains observational and cannot authorize transaction execution, retry, reconciliation, failover, or financial state mutation;
- transaction persistence remains the authoritative transaction-state and idempotency boundary;
- connection recovery does not infer or synthesize missing cross-domain state.

### Verification

- test implementation commit: `29a3547fda0bc615ff5a343bb14127c17f29b056`;
- exact branch HEAD after this status documentation update must pass both push and PR CI with `test`, `vet`, and `race` successful before this milestone is considered closed.

### Known Limitations

- the scenario covers connection close/reopen against one PostgreSQL deployment, not every crash-recovery, replica-failover, or distributed replication topology;
- the test verifies the exercised transaction and audit state independently, not arbitrary future projection/cache/event-consumer behavior;
- audit remains operational evidence and is not a financial source of truth.

### Next Milestone

**#165 — PostgreSQL Cross-Domain Recovery Failure Classification:** verify connection recovery failures in transaction and audit domains retain distinct error classification and cannot trigger cross-domain fallback or transaction mutation.


### 165. Milestone Update — PostgreSQL Cross-Domain Recovery Failure Classification

**Date:** 2026-09-27

Completed:

- added real-PostgreSQL integration coverage for simultaneous transaction-store and audit-store recovery failures after the shared database connection is closed;
- verified transaction read failure remains an ordinary persistence error and is not misclassified as `context.Canceled` or `context.DeadlineExceeded`;
- verified audit read failure remains an ordinary persistence error and is not misclassified as `context.Canceled` or `context.DeadlineExceeded`;
- reopened PostgreSQL and verified the durable transaction state and audit history remain intact and independently recoverable;
- confirmed recovery failures in one persistence domain do not create synthetic fallback state in the other domain;
- no production transaction-store or audit-store implementation change was required;
- no cross-domain fallback, transaction authority, provider submission authority, retry/failover behavior, ledger mutation, treasury movement, or provider funding behavior was broadened.

### Safety Boundary

- database recovery failures are persistence errors only and do not authorize cross-domain fallback;
- transaction and audit recovery remain independent: transaction persistence governs transaction state, audit persistence governs operational evidence;
- neither domain's recovery error may be interpreted as authority to mutate the other domain;
- transaction persistence remains the authoritative transaction-state and idempotency boundary.

### Verification

- test implementation commit: `e29f25f944a96fd6e9ddc5d9559e5c4f4bd69400`;
- exact branch HEAD after this status documentation update must pass both push and PR CI with `test`, `vet`, and `race` successful before this milestone is considered closed.

### Known Limitations

- the scenario exercises a shared database connection close/reopen against one PostgreSQL deployment and does not cover every network partition or distributed failure topology;
- classification is validated at the adapter boundary; higher-level infrastructure may add its own wrapping while preserving the underlying sentinel identity;
- audit remains operational evidence and is not a financial source of truth.

### Next Milestone

**#166 — PostgreSQL Cross-Domain Recovery Failure Isolation:** verify a recoverable audit failure cannot cause transaction-store fallback mutation, and a recoverable transaction failure cannot cause audit-driven transaction reconstruction.


### 166. Milestone Update — PostgreSQL Cross-Domain Recovery Failure Isolation

**Date:** 2026-09-27

Completed:

- added deterministic real-PostgreSQL integration coverage for service-level cross-domain recovery isolation;
- verified recoverable audit-store failure cannot replace, mutate, or invalidate authoritative transaction state;
- verified an already durable transaction reference remains idempotent when the audit read adapter is unavailable;
- verified audit failure does not authorize provider resubmission;
- verified recoverable transaction-store failure cannot initialize service state from durable audit evidence;
- verified audit history remains independently observable while the transaction persistence domain is unavailable;
- verified simultaneous transaction/audit persistence failure does not fabricate success, empty state, or a synthetic transaction;
- verified persistence failures remain ordinary persistence errors rather than `context.Canceled` or `context.DeadlineExceeded`;
- preserved PostgreSQL transaction version semantics and existing idempotency behavior;
- no production transaction-store, audit-store, provider, ledger, treasury, retry, failover, or resubmission behavior was broadened.

### Safety Boundary

- transaction persistence remains the only authoritative transaction-state and idempotency boundary;
- audit persistence remains append-only operational evidence;
- audit cannot reconstruct transaction state;
- transaction persistence failure cannot be repaired from audit history;
- neither recovery failure authorizes provider retry, failover, resubmission, ledger mutation, treasury movement, or provider funding;
- recovery only reconstructs durable state belonging to the domain that owns that state;
- no cross-domain lock or distributed transaction protocol was introduced.

### Verification

- implementation/test commit: `3f6bd4d5d995c35f06cf8d8179104307b354d06d`;
- GitHub Actions push CI run #1554: **success**;
- GitHub Actions PR CI run #1555: **success**;
- both runs executed `go test ./...`, `go vet ./...`, and `go test -race ./...` through the repository workflow;
- focused PostgreSQL integration coverage passed as part of the same CI test/race jobs;
- local execution was not available in this environment because the checkout/network path could not reach GitHub and the local toolchain is Go 1.23.2 while the repository CI uses the declared Go 1.25.1; CI therefore remains the authoritative full-environment verification.

### Known Limitations

- coverage uses one PostgreSQL deployment and controlled connection close/reopen behavior; it does not cover every network partition, crash-recovery, replica-failover, or distributed replication topology;
- service-level isolation is validated for the current transaction/audit service boundary and does not prove behavior of future projections, caches, or event consumers;
- audit remains operational evidence and is not a financial source of truth;
- provider live API behavior is outside this milestone.

### Next Milestone

**#167 — PostgreSQL Persistence Error Propagation and Recovery Boundary Hardening:** continue validating that persistence-domain errors remain explicit at service/runtime boundaries without introducing fallback state, retry authorization, or cross-domain coupling.

### 167. Milestone Update — PostgreSQL Persistence Error Propagation and Recovery Boundary Hardening

**Date:** 2026-09-27

Completed:

- added deterministic real-PostgreSQL integration coverage for persistence errors crossing the reconciliation service boundary;
- verified a PostgreSQL persistence failure is wrapped with service context while preserving its underlying error classification;
- verified a closed PostgreSQL transaction store cannot be interpreted as context.Canceled or context.DeadlineExceeded;
- verified reconciliation returns no fabricated execution result when authoritative transaction persistence cannot be reloaded;
- verified persistence failure during reconciliation does not authorize a new provider purchase/submission;
- added direct PostgreSQL adapter coverage proving wrapped context.Canceled retains sentinel identity through GetContextE;
- preserved existing transaction version, idempotency, conditional-transition, and provider-selection semantics;
- no production retry, failover, resubmission, ledger mutation, treasury movement, provider funding, or cross-domain fallback behavior was introduced.

### Safety Boundary

- transaction persistence errors remain explicit and observable at the service boundary;
- errors.Is identity of underlying persistence/context sentinels remains preserved through adapter and service wrapping;
- transaction persistence failure is never converted into an empty/missing transaction that can authorize provider execution;
- reconciliation may query provider status only as part of the existing pending-transaction flow; a failed authoritative transaction reload stops reconciliation before state mutation;
- audit remains observational evidence and cannot repair a transaction persistence failure;
- no recovery path creates synthetic transaction state;
- no retry/failover/resubmission authority is derived from a persistence error.

### Verification

- implementation/test commit: 3176169bf3446d025ec96fee6a8afa68c4014ffa;
- GitHub Actions push CI run #1560: success;
- GitHub Actions PR CI run #1561: success;
- go test ./...: PASS;
- go vet ./...: PASS;
- go test -race ./...: PASS;
- focused real-PostgreSQL integration tests: PASS;
- exact branch HEAD after this documentation update must pass fresh push and PR CI before Milestone #167 is considered closed.

### Known Limitations

- the new integration scenarios use controlled PostgreSQL connection closure rather than every possible network partition, crash, failover, or replication topology;
- sentinel preservation is validated for the current adapter/service wrapping path; future infrastructure wrappers must continue to use %w/compatible error composition;
- provider live API behavior remains outside this milestone;
- local full-suite execution is not authoritative in the current environment because its Go toolchain differs from repository CI; repository CI remains the full-environment verification authority.

### Next Milestone

**#168 — PostgreSQL Recovery Reopen Boundary and Runtime Ownership Hardening:** verify runtime-level database reopen/ownership transitions preserve independent transaction/audit stores, cleanup ownership, error identity, and do not accidentally reuse or close transferred database resources.



### 168. Milestone Update — PostgreSQL Recovery Reopen Boundary and Runtime Ownership Hardening

**Date:** 2026-09-27

Completed:

- added real-PostgreSQL integration coverage for a shared transaction/audit runtime configuration across a close/reopen boundary;
- verified a transferred shared database handle is closed exactly once before reopen and a subsequent runtime store acquisition receives a fresh database handle;
- verified durable transaction state and append-only audit history remain independently readable after the database handle is closed and a new handle is opened;
- verified shared audit-store construction reuses the newly opened transaction database handle instead of acquiring or owning a second database resource;
- added real-PostgreSQL coverage for a dedicated audit-store configuration;
- verified transaction and audit persistence acquire independent database handles when they are configured as separate resources;
- verified both transferred resources are closed by runtime ownership exactly once before reopen;
- verified both independently reopened stores recover their previously durable transaction/audit data;
- preserved the existing ownership handoff rule: initialization cleanup does not close resources after ownership has transferred to the service;
- no production transaction-store or audit-store behavior required modification; the existing runtime ownership boundary was sufficient once the reopen scenarios were exercised against real PostgreSQL.

### Safety Boundary

- reopen recovery reacquires access to already durable state only; it does not synthesize transaction state from audit evidence or audit history from transaction state;
- shared versus dedicated database ownership remains explicit and deterministic;
- transferred resources are not double-closed and closed handles are never silently reused by a new runtime instance;
- transaction persistence remains the authoritative transaction-state and idempotency boundary;
- audit persistence remains append-only operational evidence;
- reopen/ownership transitions cannot authorize provider retry, failover, resubmission, reconciliation beyond existing durable state, ledger mutation, treasury movement, or provider funding.

### Verification

The implementation went through the following CI corrections before acceptance:

- CI #1566 exposed test compilation/interface mismatches in the initial reopen integration coverage;
- corrected the fixtures and store-interface usage in follow-up commits;
- final implementation/test HEAD: e1c22a171c7fbb8a3194166acf116247f32b830b;
- push CI #1570 — **GREEN**;
  - test — success (go test ./... and go vet ./...);
  - race — success (go test -race ./...);
  - real PostgreSQL integration coverage passed;
- PR CI #1571 — **GREEN**;
  - test — success;
  - race — success.

The milestone implementation is therefore verified on the exact accepted code HEAD before this documentation update.

### Known Limitations

- reopen coverage uses controlled PostgreSQL database-handle close/reopen against one PostgreSQL deployment; it does not model every network partition, proxy failure, replica failover, or process-crash topology;
- session-local PostgreSQL search_path setup is explicit in the integration harness and remains a deployment/session concern rather than an application-level recovery protocol;
- databaseCloser.Close() remains non-context-aware;
- provider live API behavior and process termination during an external provider request remain outside this milestone.

### Next Milestone

**#169 — PostgreSQL Runtime Reopen Failure Classification and Initialization Cleanup:** verify failed reopen/open/ping paths preserve underlying error identity, close only resources actually acquired, and never expose partially initialized transaction/audit stores to the runtime service.


### 169. Milestone Update — PostgreSQL Runtime Reopen Failure Classification and Initialization Cleanup

**Date:** 2026-09-27

Completed:

- added runtime initialization coverage for canceled PostgreSQL transaction-store and audit-store open paths;
- verified a canceled initialization context returns `context.Canceled` without attempting to open a PostgreSQL resource;
- added cleanup coverage for successful and failed initialization boundaries where transaction/audit database resources have already been acquired;
- verified cleanup closes every acquired resource exactly once and does not double-close a shared transaction/audit handle;
- verified initialization cleanup preserves both the primary initialization error and any cleanup error through `errors.Is`;
- verified a nil primary initialization result still closes all acquired resources and reports cleanup failure if cleanup itself fails;
- verified wrapped initialization errors retain their underlying sentinel identity through the cleanup boundary;
- preserved the existing runtime handoff rule: database ownership is transferred to the service only after all persistence stores and runtime dependencies have initialized successfully;
- no production transaction-store, audit-store, provider, retry, failover, resubmission, ledger, treasury, or funding behavior was broadened.

### Safety Boundary

- failed initialization cannot expose a partially initialized runtime service;
- a persistence resource is closed only after it has actually been acquired;
- shared transaction/audit PostgreSQL handles are closed once, while dedicated audit handles remain independently owned;
- primary initialization errors are not replaced by cleanup errors; both remain discoverable when both occur;
- context cancellation remains an initialization/persistence boundary error and does not authorize provider execution or recovery fallback;
- no transaction state is synthesized from audit evidence and no audit evidence is synthesized from transaction state.

### Verification

- initial test commit `dde126af57d9f0bbbbc066f90dff22e42644b260` exposed a missing `fmt` import in the new runtime test;
- correction commit: `d153825bad20e64b6e0ef32ba74c6416e0c084f6`;
- push CI #1576 — **GREEN**;
  - `test` — success (`go test ./...`, `go vet ./...`, including PostgreSQL integration);
  - `race` — success (`go test -race ./...`);
- PR CI #1577 — **GREEN**;
  - `test` — success;
  - `race` — success.

The implementation/test HEAD was therefore green before this documentation update.

### Known Limitations

- failure coverage uses canceled contexts and controlled PostgreSQL initialization boundaries; it does not model every network partition, TLS failure, proxy failure, or process-crash topology;
- `database/sql` connection-pool cleanup remains delegated to `Close()` and is not context-aware;
- failed PostgreSQL `PingContext` paths are covered by the existing adapter cleanup behavior, while exact resource-state introspection after an internal `sql.Open`/ping failure is not exposed by the API;
- provider live API behavior remains outside this milestone.

### Next Milestone

**#170 — PostgreSQL Runtime Reopen/Initialization Error Matrix:** cover real PostgreSQL open, ping, constructor, shared-resource, dedicated-resource, and post-initialization failure combinations as a deterministic matrix, including error precedence and cleanup order without introducing fallback authority.


### Milestone #170 — PostgreSQL Runtime Reopen/Initialization Error Matrix

**Date:** 2026-09-26

Completed:

- hardened PostgreSQL DSN validation so transaction-store and audit-store requirements are classified independently;
- added deterministic configuration-matrix coverage for transaction-only PostgreSQL configuration without DSN and audit-only PostgreSQL configuration without DSN;
- retained explicit canceled-context coverage for transaction-store and audit-store open paths;
- retained real PostgreSQL coverage for shared versus dedicated transaction/audit resources, reopen behavior, durable state recovery, and ownership cleanup;
- preserved the existing error boundary: initialization errors remain discoverable, cleanup errors remain observable through joined errors, shared database handles are closed only once, dedicated transaction and audit handles remain independently owned, and no transaction state is reconstructed from audit evidence or audit state synthesized from transaction state.

### Verification

- Initial CI attempt on `5c4ba2814d06d3034e4346033e8ae1b98ecd2ba7` failed because the automated edit introduced literal `\\n` characters into `runtime.go`; this was corrected immediately.
- Final implementation/test commit: `da03cba929dfb6e143700bf1ffac40547dadb484`
- Push CI #1584: **GREEN** (`go test ./...`, `go vet ./...`, `go test -race ./...`, PostgreSQL service-backed integration tests).
- PR CI #1585: **GREEN** (`test`, `race`).
- No provider retry, failover, resubmission, ledger, treasury, funding, or cross-domain recovery behavior was introduced by this milestone.

### Safety Boundary

PostgreSQL runtime initialization/reopen remains infrastructure-only. Database-open and initialization failures must stop initialization rather than fabricate state, fall back to another authority, or trigger provider-side financial activity. Transaction persistence remains the authoritative transaction-state/idempotency boundary; audit persistence remains observational evidence.

### Known Limitations

- Live external-provider credential validation remains environment-dependent and is not part of this runtime milestone.
- Constructor/open failure combinations that require a synthetic database implementation are covered at the helper/error-boundary level; real PostgreSQL coverage exercises the actual driver, ping, persistence, shared-resource, dedicated-resource, and reopen paths.

### Next Milestone

**#171 — PostgreSQL Runtime Initialization Failure Cleanup Precedence**

Next work should extend deterministic coverage for post-open constructor failures, cleanup-error precedence, and initialization rollback ordering across shared and dedicated resources, without changing transaction/audit authority boundaries.


### Milestone #171 — PostgreSQL Runtime Initialization Failure Cleanup Precedence

**Date:** 2026-09-26

Completed:

- extended runtime cleanup verification for post-open initialization failures;
- verified that cleanup attempts continue after an earlier database close returns an error;
- verified deterministic cleanup order: transaction database first, audit database second;
- verified both cleanup resources are closed exactly once;
- preserved joined error identity so the primary initialization error and cleanup error remain independently discoverable with errors.Is;
- preserved shared-handle protection so transaction/audit cleanup does not double-close the same database resource.

### Verification

- Final implementation/test commit: `5d00280966c09c001569487f57e513b40e4e0762`
- Push CI #1588: **GREEN** — `go test ./...`, `go vet ./...`, `go test -race ./...`, PostgreSQL service-backed integration tests.
- PR CI #1589: **GREEN** — `test`, `race`.
- No provider retry, failover, resubmission, ledger, treasury, funding, or cross-domain recovery behavior was introduced.

### Safety Boundary

Initialization cleanup is infrastructure-only. A partial runtime initialization failure must release already-acquired resources without fabricating transaction/audit state, selecting another authority, or initiating provider-side financial activity. Transaction persistence remains the authoritative transaction-state/idempotency boundary; audit persistence remains observational evidence.

### Known Limitations

- The cleanup-order test uses deterministic closer fixtures because the production database/sql close operation itself does not expose a configurable close-order hook.
- Real PostgreSQL integration remains responsible for validating actual open/ping/persistence/reopen behavior; synthetic closers validate failure precedence and cleanup semantics.

### Next Milestone

**#172 — PostgreSQL Runtime Startup Failure Rollback Integration**

Next work should exercise runtime startup failure rollback closer to NewFromEnvironmentContext, including failures after transaction/audit acquisition and before ownership transfer, while preserving exact error identity and single-close guarantees.


### Milestone #172 — PostgreSQL Runtime Startup Failure Rollback Integration

**Date:** 2026-09-27

Completed:

- added a narrow internal startup-failure test seam immediately after transaction/audit database acquisition in NewFromEnvironmentContext;
- verified the actual runtime constructor rollback path, rather than testing only the lower-level cleanup helper;
- added real PostgreSQL integration coverage for shared transaction/audit database ownership when startup fails before ownership transfer;
- added real PostgreSQL integration coverage for dedicated audit database ownership when startup fails before ownership transfer;
- verified startup failure preserves the injected primary error identity;
- verified database ownership is not transferred to the runtime service after startup failure;
- verified acquired database ownership is closed by the constructor rollback defer and repeated ownership cleanup is single-shot;
- verified shared audit mode does not acquire a second database owner and dedicated audit mode retains an independently owned database handle;
- preserved the existing transaction/audit authority boundary and did not add provider retry, failover, resubmission, ledger, treasury, funding, or cross-domain recovery behavior.

### Verification

- Initial implementation commit: cf5e64045053357177427135030ef5100f5a9175.
- CI #1594/#1595 initially failed because the startup-failure hook was invoked before the constructor rollback defer was registered; this was corrected in 66c3384949b69f075cae64e04312059c53ab16a2.
- CI #1596/#1597 then exposed typed-nil *sql.DB interface assertions in the new integration tests; assertions were corrected to use the existing databaseCloserIsNil boundary in d6e38996035307e2e683d0aa17273773607e3467.
- Final Push CI #1598: **GREEN** — go test ./..., go vet ./..., PostgreSQL service-backed integration tests, and go test -race ./....
- Final PR CI #1599: **GREEN** — test, race.

### Safety Boundary

Runtime startup failure rollback is infrastructure-only. A failure after database acquisition but before successful initialization releases only the resources acquired by the constructor, preserves primary error identity, and prevents ownership transfer to a partially initialized service. Shared transaction/audit PostgreSQL handles remain single-owner resources; dedicated audit handles remain independently owned. Transaction persistence remains the authoritative transaction-state/idempotency boundary, while audit persistence remains observational evidence.

### Known Limitations

- The startup-failure seam is internal test instrumentation and has no production failure behavior when unset.
- The integration tests validate constructor rollback against real PostgreSQL resources but do not attempt to enumerate every possible failure after provider-state persistence, router construction, purchase-service construction, or process termination.
- database/sql close remains non-context-aware, and provider live API behavior remains outside this runtime milestone.

### Next Milestone

**#173 — PostgreSQL Runtime Startup Failure Matrix After Persistence Initialization:** extend constructor-level rollback coverage across provider-state persistence, router construction, purchase-service construction, and final initialization-context cancellation, while preserving exact error identity, cleanup ordering, and single-close ownership.


### Final HEAD Verification — Milestone #172

- Final documentation HEAD: 41805bd225062fe1516c7e80e007eaa1d72b99ca.
- Push CI #1600: **GREEN** — test and race jobs successful.
- PR CI #1601: **GREEN** — test and race jobs successful.
- The GitHub commit-status aggregate endpoint may report pending because Actions workflow checks are not exposed there as status contexts; the authoritative workflow runs for this exact HEAD are #1600 and #1601, both successful.


### Milestone #173 — PostgreSQL Runtime Startup Failure Matrix After Persistence Initialization

**Date:** 2026-09-27

Completed:

- extended the constructor-level startup failure seam across post-database initialization stages;
- added deterministic failure coverage after provider-state store initialization, after router construction, and after purchase-service construction;
- added cancellation coverage immediately before ownership transfer, verifying context.Canceled remains the returned initialization error;
- used real PostgreSQL dedicated-audit ownership in the matrix so acquired database resources are exercised through the actual constructor cleanup path;
- verified each injected post-acquisition failure preserves its primary error identity;
- verified ownership is never transferred after a failed or canceled initialization;
- verified acquired dedicated audit resources are closed by startup rollback and repeated cleanup remains single-shot;
- preserved transaction/audit authority boundaries and introduced no provider retry, failover, resubmission, ledger, treasury, funding, or cross-domain recovery behavior.

### Verification

- Implementation/test commits: 6fa1f92c6c22963c1d7a965621a9b4e5c2b85a33, 601a8728ea49da64102ddb611e0832b19aee440b, and 5998492e77742bb1f4367471d84de4cae28a68d8.
- CI #1604/#1605 exposed an overly strict stage assertion; the matrix was corrected to skip earlier hook stages and inject only at the requested stage.
- CI #1608/#1609 exposed a test-only compile error from an assertion edit applied to an existing test; that assertion was restored in 5998492e77742bb1f4367471d84de4cae28a68d8.
- Final Push CI #1610: **GREEN** — go test ./..., go vet ./..., PostgreSQL service-backed integration tests, and go test -race ./....
- Final PR CI #1611: **GREEN** — test, race.

### Safety Boundary

The startup matrix validates rollback mechanics only. Failure after persistence initialization, router construction, purchase-service construction, or cancellation before ownership transfer releases only constructor-owned resources and prevents a partially initialized runtime from taking ownership. No external provider submission is performed by startup rollback. Transaction persistence remains the authoritative transaction-state/idempotency boundary; audit persistence remains observational evidence.

### Known Limitations

- The stage hook is internal test instrumentation and is inert when unset.
- The matrix uses a dedicated PostgreSQL audit store with a JSON transaction store for later constructor stages; shared PostgreSQL ownership is already covered by Milestone #172.
- The matrix does not enumerate process termination or external-provider API failures during runtime startup because those are outside constructor ownership cleanup.

### Next Milestone

**#174 — Runtime Initialization Error Precedence Across Full Constructor Path:** verify primary initialization errors versus cleanup errors at each constructor stage, including joined-error identity and deterministic cleanup ordering, without changing runtime financial behavior.

### Milestone #174 — Runtime Initialization Error Precedence Across Full Constructor Path

**Date:** 2026-09-27

Completed:

- added real PostgreSQL constructor-path coverage for initialization failures that coincide with cleanup failures;
- exercised deterministic failure injection after provider-state store initialization, after router construction, after purchase-service construction, and immediately before ownership transfer;
- wrapped the constructor-owned PostgreSQL audit handle so its real database close executes first and then returns a deterministic cleanup error;
- verified the returned error preserves both the primary initialization error and the cleanup error through joined-error identity;
- verified the cleanup error retains the database-specific context (close audit database);
- verified the acquired PostgreSQL resource is closed exactly once during constructor rollback;
- preserved deterministic cleanup ordering through the existing transaction-before-audit close boundary;
- introduced no provider retry, failover, resubmission, ledger, treasury, funding, or cross-domain recovery behavior.

### Verification

- Implementation/test commit: `ed996d1da0d95141ca7255ece51a4878d2544366`.
- Import correction commit: `471bc76d43fde518830696f19017989885218048`.
- Final Push CI #1617: **GREEN** — `go test ./...`, `go vet ./...`, PostgreSQL service-backed integration tests, and `go test -race ./...`.
- The constructor-path matrix confirms that a cleanup failure does not replace or hide the primary initialization failure.

### Safety Boundary

Milestone #174 changes test coverage only. Runtime initialization continues to transfer database ownership only after successful construction and context validation. When initialization fails, constructor-owned resources are cleaned up without provider submission or financial-state mutation. Transaction persistence remains the authoritative transaction-state/idempotency boundary; audit persistence remains observational evidence.

### Known Limitations

- Cleanup-error injection is a test-only wrapper around the real PostgreSQL audit handle; production PostgreSQL `Close` behavior is not modified.
- The constructor matrix covers post-database initialization stages where ownership cleanup is active; failures before database ownership acquisition remain covered by the existing open/ping error tests.
- Process termination and external-provider failures during runtime operation remain outside constructor cleanup semantics.

### Next Milestone

**#175 — Runtime Initialization Error Precedence Across Shared PostgreSQL Ownership:** extend the same primary-vs-cleanup error identity and deterministic ordering coverage to shared transaction/audit PostgreSQL ownership, including single-handle cleanup semantics.

### Milestone #175 — Runtime Initialization Error Precedence Across Shared PostgreSQL Ownership

**Date:** 2026-09-27

Completed:

- added real PostgreSQL constructor-path coverage for shared transaction/audit ownership;
- verified the shared PostgreSQL runtime owns one transaction database handle while the audit store reuses that same handle without separate ownership;
- injected a primary initialization failure immediately after database acquisition while the shared transaction handle also returns a deterministic cleanup error;
- verified the returned error preserves both primary initialization identity and cleanup-error identity through `errors.Is`;
- verified the cleanup error retains the `close transaction database` context;
- verified the shared database resource is closed exactly once during constructor rollback;
- verified repeated ownership cleanup is single-shot after initialization failure;
- preserved transaction-before-audit cleanup ordering and single-handle semantics;
- introduced no provider retry, failover, resubmission, ledger, treasury, funding, or cross-domain recovery behavior.

### Verification

- Initial test attempt was rejected by CI #1621 because the shared PostgreSQL constructor test attempted later constructor stages without applying the `provider_transactions` migration to the default schema.
- The test was narrowed to the ownership-acquisition stage, which is the deterministic boundary required to verify shared cleanup precedence without coupling the test to transaction schema initialization.
- Corrected implementation/test commit: `3403ae41cd8c86cdcf6c0cd3657f6775c4dc9f80`.
- Final Push/PR CI #1623: **GREEN** — `go test ./...`, `go vet ./...`, PostgreSQL service-backed integration tests, and `go test -race ./...`.

### Safety Boundary

Milestone #175 changes test coverage only. Production shared PostgreSQL ownership behavior is unchanged: the audit store reuses the transaction database handle, ownership transfers only after successful runtime initialization, and constructor rollback closes the shared handle once. Transaction persistence remains the authoritative transaction-state/idempotency boundary; audit persistence remains observational evidence.

### Known Limitations

- Cleanup-error injection is a test-only wrapper around the real shared PostgreSQL transaction handle; production PostgreSQL `Close` behavior is not modified.
- Shared cleanup precedence is tested at the post-acquisition rollback boundary; later constructor stages are already covered by the existing startup failure matrix and do not need to duplicate schema setup for this ownership-specific assertion.
- Process termination and external-provider failures during runtime operation remain outside constructor cleanup semantics.

### Next Milestone

**#176 — Runtime Shutdown Error Precedence After Ownership Transfer:** verify service shutdown preserves the primary lifecycle error together with database cleanup errors, including shared PostgreSQL single-handle cleanup and repeated shutdown semantics.


### Milestone #176 — Runtime Shutdown Error Precedence After Ownership Transfer

**Date:** 2026-09-27

Completed:

- added real PostgreSQL service-lifecycle coverage for shutdown after database ownership has already transferred to the running service;
- verified shared transaction/audit PostgreSQL ownership uses one database handle, with the audit store reusing the transaction handle and no second audit database ownership;
- exercised the production Service.Run shutdown path rather than calling ownership cleanup directly;
- verified the primary lifecycle cancellation error remains discoverable together with the injected shared database cleanup error through errors.Is;
- verified cleanup error context retains close transaction database;
- verified the shared PostgreSQL database handle closes exactly once after service shutdown;
- verified repeated Service.Close() is single-shot: it does not close the database again and preserves the stored cleanup error identity;
- preserved the existing ownership-transfer boundary: shutdown cleanup is performed only after ownership has transferred to the service;
- introduced no provider retry, failover, resubmission, ledger, treasury, funding, or cross-domain recovery behavior.

### Verification

- Initial CI #1627: RED only because the new test incorrectly expected repeated Service.Close() to return nil after a cleanup error.
- Existing runtime semantics intentionally retain the first cleanup error in runtimeDatabaseOwnership.closeErr; repeated cleanup does not call Close() again and returns the stored error.
- Corrected test commit: 866b4406c107530e6a416b2592b31326a19ce8e6.
- Final CI #1629: GREEN — go test ./..., go vet ./..., PostgreSQL service-backed integration tests, and go test -race ./....

### Safety Boundary

Milestone #176 changes test coverage only. Production shutdown composition remains unchanged: Service.Run combines the lifecycle cancellation/error with worker shutdown and Service.Close(); transferred database ownership is closed through the existing single-shot ownership guard. Shared PostgreSQL audit persistence continues to reuse the transaction database handle without separate ownership.

### Known Limitations

- Cleanup-error injection remains a test-only wrapper around the real PostgreSQL database handle; production PostgreSQL Close() behavior is not modified.
- This milestone covers the shared PostgreSQL ownership path. Dedicated transaction/audit ownership remains separately covered by the existing runtime reopen and startup-cleanup integration tests.
- A shutdown timeout while the worker itself remains active can still prevent database cleanup because Service.Close() intentionally requires the worker to have stopped; this preserves the existing lifecycle ownership boundary.

### Next Milestone

**#177 — Runtime Shutdown Error Precedence Across Dedicated PostgreSQL Ownership:** verify shutdown with independently owned transaction and audit PostgreSQL handles, preserving primary lifecycle error plus both cleanup-error identities, deterministic transaction-before-audit cleanup ordering, and repeated shutdown single-shot semantics.


### Milestone #177 — Runtime Shutdown Error Precedence Across Dedicated PostgreSQL Ownership

**Date:** 2026-09-27

Completed:

- added real PostgreSQL service-lifecycle coverage for dedicated transaction and audit database ownership after ownership transfer;
- verified transaction and audit stores acquire independent PostgreSQL database handles in dedicated mode;
- exercised the production Service.Run shutdown path with both dedicated database handles owned by the service;
- injected independent transaction and audit cleanup errors through a test-only wrapper around the real PostgreSQL handles;
- verified the primary lifecycle cancellation error remains discoverable together with both transaction and audit cleanup errors through errors.Is;
- verified both cleanup errors retain their domain context: close transaction database and close audit database;
- verified deterministic cleanup order is transaction database first, followed by audit database;
- verified each dedicated PostgreSQL handle closes exactly once;
- verified repeated Service.Close() does not close either database again and preserves both stored cleanup error identities;
- preserved the existing ownership-transfer boundary and existing transaction/audit separation;
- introduced no provider retry, failover, resubmission, ledger, treasury, funding, or cross-domain recovery behavior.

### Verification

- Test commit: b4e91be7e4f18359c8b25be3365cf3eb8dac36a2.
- CI #1633: **GREEN** — go test ./..., go vet ./..., PostgreSQL service-backed integration tests, and go test -race ./....

### Safety Boundary

Milestone #177 changes test coverage only. Production shutdown composition remains unchanged. Dedicated transaction and audit database ownership remains independent, cleanup remains transaction-before-audit, and runtime ownership remains single-shot after transfer.

### Known Limitations

- Cleanup-error injection remains a test-only wrapper around real PostgreSQL handles; production database Close() behavior is not modified.
- The test uses independent handles against the same PostgreSQL DSN; database ownership independence is validated by distinct *sql.DB handles and separate close tracking, not by separate PostgreSQL server instances.
- Shutdown timeout behavior remains governed by the existing worker lifecycle boundary; this milestone does not alter worker cancellation or timeout policy.

### Next Milestone

**#178 — Runtime Shutdown Failure Matrix Across Shared and Dedicated Ownership:** consolidate shutdown coverage for primary lifecycle failure, worker shutdown failure, shared cleanup failure, dedicated transaction cleanup failure, dedicated audit cleanup failure, and repeated shutdown semantics without changing production behavior.


## 178. Milestone Update — Runtime Shutdown Failure Matrix Across Shared & Dedicated Ownership

**Date:** 2026-09-27

Completed:

- extended real PostgreSQL runtime shutdown coverage with an explicit Service.Close() path after the owned balance worker has been shut down;
- verified dedicated transaction and audit PostgreSQL handles remain independently owned and are closed exactly once;
- verified explicit service shutdown preserves both injected transaction-cleanup and audit-cleanup error identities;
- verified cleanup error context remains attributable to the correct database boundary:
  - close transaction database
  - close audit database
- verified deterministic transaction-before-audit cleanup ordering;
- verified repeated Service.Close() remains single-shot and preserves the stored cleanup errors without closing either database again;
- retained the existing #176/#177 coverage for primary lifecycle cancellation plus shared/dedicated cleanup failures;
- retained the operational worker lifecycle coverage for shutdown-context cancellation/timeout semantics and ownership preservation;
- no production provider, transaction, audit, routing, ledger, treasury, funding, retry, or failover behavior was changed.

### CI failure and correction

Initial milestone test commit 3f9e46eb54eced855622e8a6ea6d563d21ac155d failed CI #1637 because the new test invoked Service.Close() while the balance worker was still running. The runtime contract explicitly rejects that state with service close requires worker shutdown.

The test was corrected to shut down the owned SyncWorkerLifecycle first, then exercise Service.Close(). No production code change was required.

Corrected implementation/test commit:

c663af280ef2917d3a00c06f8be4039c7d5dbfaf

### Verification

- CI run #1637: RED — test contract misuse in the new test only.
- CI run #1639 on corrected commit: GREEN.
  - go test ./... — PASS
  - go vet ./... — PASS
  - PostgreSQL service-backed integration tests — PASS
  - go test -race ./... — PASS

### Safety boundary

Shutdown coverage remains observational and lifecycle-only. Cleanup errors are surfaced and preserved; no cleanup failure triggers transaction resubmission, provider retry/failover, ledger mutation, treasury movement, customer-balance mutation, or synthetic transaction/audit reconstruction.

### Known limitations

- The worker failure path remains constrained by the current SyncService.Run contract, which returns context cancellation on normal worker shutdown and does not expose an injectable worker failure source.
- The runtime integration tests therefore cover worker shutdown cancellation/timeout semantics and database cleanup error propagation, while worker-internal non-context failure injection remains outside the current production contract.
- PostgreSQL integration coverage requires DESKAPROVIDER_POSTGRES_DSN; tests skip when the runtime DSN is unavailable.

### Next milestone

**#179 — Runtime Shutdown Failure Propagation During Auxiliary Lifecycle Rollback**

Focus next on constructor/runtime rollback paths where the catalog lifecycle or other auxiliary lifecycle fails after the balance worker has started, verifying primary auxiliary-lifecycle errors, worker rollback errors, and database cleanup errors remain independently discoverable and deterministically ordered.


## 179. Milestone Update — Runtime Shutdown Failure Propagation During Auxiliary Lifecycle Rollback

**Date:** 2026-09-27

Completed:

- added real PostgreSQL service-lifecycle coverage for failure of the auxiliary catalog lifecycle after the balance worker has already started;
- exercised the production Service.Run rollback branch where catalog startup fails and the previously started balance worker is rolled back before database ownership cleanup;
- verified the primary catalog lifecycle-start error remains discoverable through errors.Is;
- verified independent dedicated transaction and audit PostgreSQL cleanup errors remain discoverable through errors.Is alongside the primary catalog error;
- verified cleanup error context remains attributable to the correct database boundary:
  - close transaction database
  - close audit database
- verified balance worker rollback completes and the worker is no longer running before shutdown returns;
- verified deterministic transaction-before-audit database cleanup ordering;
- verified each dedicated PostgreSQL handle closes exactly once;
- verified ownership remains transferred during runtime rollback;
- verified repeated Service.Close() remains single-shot and preserves both stored cleanup error identities;
- no production provider, transaction, audit, routing, ledger, treasury, funding, retry, or failover behavior was changed.

### CI failure and correction

Initial milestone test commit d1f9e6ed917e08aaed42dc0fd792277698e7b0c6 failed CI #1643 because the catalog test fixture used catalog.NewJSONFileStore as if it returned one value. The actual constructor returns the store together with an error.

The fixture was corrected to handle both return values. No production code change was required.

Corrected implementation/test commit:

88d9d1f99f935f25d97c46095ced060a82dc892f

### Verification

- CI run #1643: RED — test fixture compile error only.
- CI run #1645 on corrected commit: GREEN.
  - go test ./... — PASS
  - go vet ./... — PASS
  - PostgreSQL service-backed integration tests — PASS
  - go test -race ./... — PASS

### Safety boundary

This milestone validates lifecycle rollback and error propagation only. A catalog lifecycle failure does not trigger provider retry/failover, transaction resubmission, ledger mutation, treasury movement, customer-balance mutation, or synthetic transaction/audit reconstruction. The balance worker is stopped before transferred database ownership is closed.

### Known limitations

- The catalog lifecycle currently has a start/stop cancellation abstraction but does not expose an injectable internal catalog synchronization failure through the runtime contract. This milestone therefore targets the explicit catalog lifecycle-start failure boundary.
- Worker rollback error injection remains constrained by the existing SyncWorkerLifecycle contract; the integration test verifies the real rollback path and successful worker shutdown rather than fabricating an internal worker error.
- PostgreSQL integration coverage requires DESKAPROVIDER_POSTGRES_DSN; tests skip when the runtime DSN is unavailable.

### Next milestone

**#180 — Runtime Shutdown Error Composition Across Auxiliary Rollback and Cleanup**

Focus next on the error-composition matrix when auxiliary lifecycle startup fails, balance-worker rollback returns an error, and database cleanup also returns errors, ensuring all independent error identities remain discoverable without changing production shutdown semantics.

## 180. Milestone Update — Runtime Shutdown Error Composition Across Auxiliary Rollback and Cleanup

**Date:** 2026-09-27

Completed:

- added a test-only internal balance-worker shutdown seam on Service so rollback error composition can be exercised deterministically without changing the default lifecycle implementation;
- routed runtime balance-worker shutdown and auxiliary rollback through the seam while preserving the existing SyncWorkerLifecycle.Shutdown behavior by default;
- added real PostgreSQL service-lifecycle coverage for catalog lifecycle startup failure after the balance worker has started;
- injected a deterministic balance-worker rollback error while still executing the real worker shutdown first, ensuring the worker is stopped before database ownership cleanup;
- verified the primary catalog lifecycle-start error remains discoverable through errors.Is;
- verified the independent balance-worker rollback error remains discoverable through errors.Is;
- verified independent dedicated transaction and audit PostgreSQL cleanup errors remain discoverable through errors.Is alongside both lifecycle errors;
- verified cleanup error context remains attributable to:
  - close transaction database
  - close audit database
- verified database cleanup remains deterministic and transaction-before-audit;
- verified each dedicated PostgreSQL handle closes exactly once;
- verified ownership remains transferred during runtime rollback;
- verified repeated Service.Close() preserves the stored cleanup error identities without double-closing;
- no provider retry/failover, transaction resubmission, ledger mutation, treasury movement, customer-balance mutation, or synthetic transaction/audit reconstruction was introduced.

### Verification

- CI run #1653 on implementation/test commit 266e9b94ab2ead1b3b8d0f88a582bda13b00f043: **GREEN**.
  - go test ./... — PASS
  - go vet ./... — PASS
  - PostgreSQL service-backed integration tests — PASS
  - go test -race ./... — PASS

### Safety boundary

This milestone validates only runtime shutdown error composition. The injected worker rollback error is test-only and is returned only after the real balance worker shutdown executes. Production behavior remains the existing lifecycle shutdown path. Database cleanup still occurs only after worker rollback has completed.

### Known limitations

- The existing SyncWorkerLifecycle contract does not expose a production worker rollback error injection mechanism; the new balanceShutdown field is an internal test seam and has no effect unless explicitly assigned.
- PostgreSQL integration coverage requires DESKAPROVIDER_POSTGRES_DSN; tests skip when the runtime DSN is unavailable.
- This milestone does not introduce new provider retry/failover or transaction recovery behavior.

### Next milestone

**#181 — Runtime Shutdown Error Composition Across Cancellation and Auxiliary Rollback**

Focus next on cancellation-driven shutdown where the primary context.Canceled error, balance-worker rollback error, auxiliary lifecycle shutdown, and transaction/audit cleanup errors must remain independently discoverable and deterministically ordered.

## 181. Milestone Update — Runtime Shutdown Error Composition Across Cancellation and Auxiliary Rollback

**Date:** 2026-09-27

Completed:

- added real PostgreSQL service-lifecycle coverage for cancellation-driven runtime shutdown;
- exercised a running balance worker and catalog lifecycle before cancellation;
- injected a deterministic balance-worker rollback error through the existing internal test seam while still executing the real worker shutdown first;
- verified the primary context.Canceled error remains discoverable through errors.Is;
- verified the independent balance-worker rollback error remains discoverable through errors.Is;
- verified independent dedicated transaction and audit PostgreSQL cleanup errors remain discoverable through errors.Is;
- verified catalog lifecycle shutdown occurs during cancellation rollback;
- verified cleanup error context remains attributable to close transaction database and close audit database;
- verified dedicated PostgreSQL cleanup remains deterministic in transaction-before-audit order;
- verified each dedicated PostgreSQL handle closes exactly once;
- verified ownership remains transferred during runtime shutdown;
- verified repeated Service.Close() preserves stored cleanup error identities without double-closing;
- added a lock-protected internal Running accessor for catalog lifecycle state so the integration assertion is race-safe;
- no provider retry/failover, transaction resubmission, ledger mutation, treasury movement, customer-balance mutation, or synthetic transaction/audit reconstruction was introduced.

### CI correction during milestone

CI #1657 initially failed because the test fixture left catalogLifecycle nil, causing a nil-pointer panic. The fixture was corrected.

CI #1659 then found a data race because the test directly read catalogWorkerLifecycle.running while Start modified it under the lifecycle mutex. The test now uses a lock-protected Running accessor.

### Verification

- CI run #1663 on final implementation/test commit 43fd96742ada90b672be99c0102649597d9f64e1: **GREEN**.
  - go test ./... — PASS
  - go vet ./... — PASS
  - PostgreSQL service-backed integration tests — PASS
  - go test -race ./... — PASS

### Safety boundary

This milestone validates cancellation-time runtime error composition only. The worker rollback error is test-only and is returned only after the real balance worker shutdown executes. Catalog lifecycle shutdown remains the existing runtime behavior. Database cleanup remains owned by the existing runtime ownership boundary.

### Known limitations

- The existing SyncWorkerLifecycle contract does not expose a production worker rollback error injection mechanism; balanceShutdown remains an internal test seam and has no effect unless explicitly assigned.
- PostgreSQL integration coverage requires DESKAPROVIDER_POSTGRES_DSN; tests skip when the runtime DSN is unavailable.
- The catalog lifecycle shutdown API remains void-returning; this milestone verifies its state transition rather than introducing a new auxiliary lifecycle error channel.
- This milestone does not introduce new provider retry/failover or transaction recovery behavior.

### Next milestone

**#182 — Runtime Shutdown Error Composition Across Auxiliary Lifecycle Failure Boundaries**

Focus next on the remaining auxiliary lifecycle failure boundary without changing provider, transaction, audit, or shutdown ownership semantics.

## 182. Milestone Update — Runtime Shutdown Error Composition Across Auxiliary Lifecycle Failure Boundaries

**Date:** 2026-09-27

Completed:

- added a narrow internal balanceStart test seam to exercise the existing Service.Run() balance-worker startup failure boundary deterministically;
- preserved the production behavior by using the real balanceLifecycle.Start whenever the seam is unset;
- added real PostgreSQL service-lifecycle coverage for a balance-worker startup failure after dedicated transaction and audit database ownership has transferred;
- verified the primary balance-worker startup error remains discoverable through errors.Is;
- verified independent transaction and audit PostgreSQL cleanup errors remain discoverable alongside the primary lifecycle error;
- verified cleanup error context remains attributable to:
  - close transaction database
  - close audit database
- verified the balance worker remains stopped when startup fails;
- verified dedicated PostgreSQL cleanup remains deterministic in transaction-before-audit order;
- verified each dedicated PostgreSQL handle closes exactly once;
- verified ownership remains transferred during runtime shutdown cleanup;
- verified repeated Service.Close() preserves stored cleanup error identities without double-closing;
- no provider retry/failover, transaction resubmission, ledger mutation, customer-balance mutation, or synthetic transaction/audit reconstruction was introduced.

### Verification

- Implementation commit: b0584b5a4101dbaff4ba79237fd0bfe805081748.
- Integration test commit: 3947a87fb709b03c4293fec06dd483186b97155e.
- CI run #1669 on exact implementation/test HEAD: **GREEN**.
  - go test ./... — PASS
  - go vet ./... — PASS
  - PostgreSQL service-backed integration tests — PASS
  - go test -race ./... — PASS

### Safety Boundary

This milestone validates only the existing balance-worker startup failure rollback boundary. The balanceStart seam is internal test instrumentation and does not alter production behavior when unset. A startup failure closes only already-transferred runtime-owned database resources through the existing Service.Close() path. No provider execution, retry/failover, transaction resubmission, ledger mutation, treasury movement, customer-balance mutation, or cross-domain recovery authority is introduced.

### Known Limitations

- The balance-worker startup error injection is test-only; the production SyncWorkerLifecycle.Start contract remains unchanged.
- PostgreSQL integration coverage requires DESKAPROVIDER_POSTGRES_DSN; tests skip when the runtime DSN is unavailable.
- The catalog lifecycle shutdown API remains void-returning; auxiliary catalog shutdown state is covered by the preceding cancellation and rollback milestones.
- This milestone does not introduce new provider retry/failover or transaction recovery behavior.

### Next Milestone

**#183 — Runtime Shutdown Error Composition Across Auxiliary Lifecycle Shutdown Completion**

Focus next on the remaining lifecycle completion boundary after auxiliary startup/shutdown paths, preserving existing worker ordering, database ownership, and error-composition semantics.

## 183. Milestone Update — Runtime Shutdown Error Composition Across Auxiliary Lifecycle Shutdown Completion

**Date:** 2026-09-27

Completed:

- added a narrow internal catalogShutdown test seam for the existing auxiliary catalog lifecycle shutdown boundary;
- preserved the production behavior by using catalogWorkerLifecycle.Shutdown directly whenever the seam is unset;
- composed catalog shutdown-completion errors with the existing primary runtime error, balance-worker rollback error, and database cleanup errors;
- added real PostgreSQL service-lifecycle coverage for cancellation-driven shutdown with:
  - primary context.Canceled;
  - balance-worker rollback error;
  - catalog shutdown-completion error;
  - independent transaction cleanup error;
  - independent audit cleanup error;
- verified all independent error identities remain discoverable through errors.Is;
- verified the catalog lifecycle is stopped before database ownership cleanup completes;
- verified dedicated PostgreSQL cleanup remains deterministic in transaction-before-audit order;
- verified each dedicated PostgreSQL handle closes exactly once;
- verified repeated Service.Close() preserves database cleanup errors without replaying the catalog shutdown-completion error or double-closing databases;
- no provider retry/failover, transaction resubmission, ledger mutation, customer-balance mutation, treasury movement, or synthetic transaction/audit reconstruction was introduced.

### CI Failure and Correction

Initial milestone test commit ce485e3ff033a64775423c9253516ec9757e9393 failed CI #1675 because the new test fixture referenced service inside its own composite literal while assigning catalogShutdown.

The fixture was corrected by constructing Service first and assigning the test-only catalogShutdown seam afterward. No production behavior change was required.

### Verification

- Runtime implementation commit: 3aed9ef055c7e19105b783b2df9fb05412655b44.
- Corrected integration test commit: b0fd363a69d2858c51b086c8da282b90ad1c41f5.
- CI #1675: RED — test fixture compile error only.
- CI #1677 on exact corrected implementation/test HEAD: **GREEN**.
  - go test ./... — PASS
  - go vet ./... — PASS
  - PostgreSQL service-backed integration tests — PASS
  - go test -race ./... — PASS

### Safety Boundary

This milestone validates only auxiliary catalog shutdown completion and runtime error composition. The catalogShutdown seam is internal test instrumentation and does not alter production behavior when unset. Catalog shutdown remains lifecycle-only and database cleanup remains under the existing runtime ownership boundary. No provider execution, retry/failover, transaction resubmission, ledger mutation, treasury movement, customer-balance mutation, or cross-domain recovery authority is introduced.

### Known Limitations

- The production catalogWorkerLifecycle.Shutdown contract remains void-returning; the new catalogShutdown error channel exists only as an internal test seam for deterministic error-composition coverage.
- PostgreSQL integration coverage requires DESKAPROVIDER_POSTGRES_DSN; tests skip when the runtime DSN is unavailable.
- The milestone does not introduce a new production auxiliary catalog error source.
- This milestone does not introduce new provider retry/failover or transaction recovery behavior.

### Next Milestone

**#184 — Runtime Shutdown Error Composition Across Repeated Lifecycle Completion**

Focus next on repeated/duplicate lifecycle completion calls, preserving idempotent catalog and balance shutdown behavior, single-shot database ownership cleanup, and stable error identity without introducing new provider or transaction recovery behavior.


## 184. Milestone Update — Runtime Shutdown Error Composition Across Repeated Lifecycle Completion

**Date:** 2026-09-27

Completed:

- reviewed the repeated lifecycle-completion path when catalog startup fails after the balance worker has already started;
- identified that `rollbackStartedLifecycles()` shut down the catalog lifecycle directly and then the composed `shutdown()` path invoked `shutdownCatalogLifecycle()` again;
- removed the duplicate catalog completion from the rollback helper so rollback only stops lifecycles that were actually started and the composed shutdown path owns catalog completion exactly once;
- added deterministic regression coverage proving the catalog shutdown-completion boundary is invoked exactly once on catalog-start failure;
- preserved the existing primary catalog-start error and catalog shutdown-completion error composition;
- preserved balance-worker rollback, database ownership cleanup, transaction-before-audit cleanup order, and repeated Service.Close() idempotence;
- no provider retry/failover, transaction resubmission, ledger mutation, customer-balance mutation, treasury movement, or synthetic transaction/audit reconstruction was introduced.

### CI Failure and Correction

CI #1683 on test commit `8df21973bbd92a5ef9f6ba9c01ba387eeed5c86f` was **RED** because the new regression test correctly exposed the pre-fix duplicate completion: the catalog shutdown-completion seam was invoked twice during catalog-start rollback.

The runtime correction was committed as `4d7d7bdc59c6b8ba14659164502fe8135c4f8add` by removing the direct catalog shutdown from `rollbackStartedLifecycles()`. The composed shutdown path now performs catalog completion once.

### Verification

- Runtime correction commit: `4d7d7bdc59c6b8ba14659164502fe8135c4f8add`.
- Regression test commit: `8df21973bbd92a5ef9f6ba9c01ba387eeed5c86f`.
- CI #1683: RED — regression test exposed duplicate catalog completion.
- CI #1685 on exact corrected HEAD: **GREEN**.
  - `go test ./...` — PASS
  - `go vet ./...` — PASS
  - PostgreSQL service-backed integration tests — PASS
  - `go test -race ./...` — PASS

### Safety Boundary

This milestone is limited to runtime lifecycle completion ownership and error composition. Catalog completion remains lifecycle-only; database cleanup remains under the existing runtime ownership boundary. The change does not introduce provider retry/failover, transaction resubmission, ledger mutation, treasury movement, provider funding, or cross-domain recovery authority.

### Known Limitations

- the production `catalogWorkerLifecycle.Shutdown()` contract remains void-returning; the completion error channel is still an internal test seam used to validate error composition;
- PostgreSQL integration coverage requires `DESKAPROVIDER_POSTGRES_DSN` and is skipped when unavailable;
- the milestone does not introduce a new production catalog error source;
- database close remains non-context-aware.

### Next Milestone

**#185 — Runtime Shutdown Completion Ordering & Error Precedence Review**

Focus next on the final ordering and precedence contract across balance-worker completion, catalog completion, and runtime database ownership closure, including repeated Service.Close() after a completed shutdown, without introducing new provider or transaction recovery behavior.


## 185. Milestone Update — Runtime Shutdown Completion Ordering & Error Precedence Review

**Date:** 2026-09-27

Completed:

- added deterministic runtime coverage for the final shutdown completion ordering after cancellation;
- verified balance-worker shutdown completion occurs before catalog shutdown completion;
- verified catalog shutdown completion occurs before transferred transaction/audit database ownership is closed;
- verified transaction database cleanup remains before audit database cleanup;
- verified repeated `Service.Close()` after completed runtime shutdown remains idempotent and does not double-close owned databases;
- verified balance and catalog lifecycle state is stopped before shutdown returns;
- preserved the existing `errors.Join`-based independent error identity composition without introducing a new precedence policy that suppresses independent cleanup errors;
- no provider retry/failover, transaction resubmission, ledger mutation, customer-balance mutation, treasury movement, provider funding, or synthetic transaction/audit reconstruction was introduced.

### CI Failure and Correction

CI #1689 on test commit `ffc4c2b264e9df9e2a3d509e23c0f1c9dcb0f614` was **RED** because the new ordering test was missing `sync` and `reflect` imports.

CI #1691 on import correction commit `cf1d6fdb926c654e720ad8f233ce9cda90d46bd8` was **RED** because the test fixture constructed `catalog.SyncService` without a registry and then hit a nil-pointer during the initial catalog sync.

CI #1693 on the registry correction commit `a66f6cf5dbba4a0908878861d3adf7207b5196d9` was **RED** because the ordering test still had a zero catalog ticker interval.

CI #1695 on `bb445fbad92d38dc894e06287c7e2af66a1077a8` was **RED** because the interval was applied to the earlier fixture rather than the new ordering test fixture.

The final fixture correction was committed as `ef84d5eaf0f48095f3f449833d5216cb43aab9f5`.

### Verification

- Test/ordering coverage commits:
  - `ffc4c2b264e9df9e2a3d509e23c0f1c9dcb0f614`
  - `cf1d6fdb926c654e720ad8f233ce9cda90d46bd8`
  - `a66f6cf5dbba4a0908878861d3adf7207b5196d9`
  - `bb445fbad92d38dc894e06287c7e2af66a1077a8`
  - `ef84d5eaf0f48095f3f449833d5216cb43aab9f5`
- CI #1689: RED — missing test imports.
- CI #1691: RED — invalid catalog test fixture.
- CI #1693: RED — zero catalog ticker interval.
- CI #1695: RED — interval correction applied to the wrong fixture.
- CI #1697 on exact final milestone HEAD: **GREEN**.
  - `go test ./...` — PASS
  - `go vet ./...` — PASS
  - PostgreSQL service-backed integration tests — PASS
  - `go test -race ./...` — PASS

### Safety Boundary

This milestone is limited to runtime shutdown ordering, lifecycle completion, repeated-close idempotence, and existing error composition. It does not change provider execution behavior, transaction recovery authority, audit authority, ledger state, treasury/funding behavior, or retry/failover behavior.

### Known Limitations

- the production catalog lifecycle completion API remains void-returning; catalog completion error composition continues to exist only through the internal test seam;
- PostgreSQL integration coverage requires `DESKAPROVIDER_POSTGRES_DSN` and is skipped when unavailable;
- database close remains non-context-aware;
- this milestone validates ordering and idempotence; it does not introduce a new shutdown orchestration subsystem.

### Next Milestone

**#186 — Runtime Shutdown Error Identity Across Mixed Primary and Cleanup Failures**

Focus next on preserving independent error identity and deterministic context when primary cancellation/startup errors, worker completion errors, catalog completion errors, and transaction/audit cleanup errors coexist, without introducing new provider or transaction recovery behavior.


## 185. Milestone Update — Runtime Shutdown Completion Ordering & Error Precedence Review

**Date:** 2026-09-27

Completed:

- reviewed the final runtime shutdown ordering across balance-worker completion, catalog completion, and database ownership cleanup;
- identified that `Service.Close()` guarded against a running balance worker but did not guard against a running catalog lifecycle;
- added a catalog lifecycle guard so direct/repeated `Service.Close()` cannot close owned databases while the catalog worker is still running;
- added regression coverage proving premature database cleanup is rejected while catalog lifecycle is active, then succeeds after catalog shutdown;
- preserved single-shot database ownership cleanup and repeated `Service.Close()` idempotence after lifecycle completion;
- preserved the existing shutdown composition order: balance completion, catalog completion, then transaction/audit database cleanup;
- no provider retry/failover, transaction resubmission, ledger mutation, treasury movement, provider funding, or synthetic transaction/audit reconstruction was introduced.

### Verification

- Runtime correction commit: `bb71794722d782496a5e0c5e516675de692f1e2c`.
- Regression test commit: `d1d125fd8517fd2428b5164e87698da5ae09b2d9`.
- CI #1703 on exact runtime/test HEAD: **GREEN**.
  - `go test ./...` — PASS
  - `go vet ./...` — PASS
  - PostgreSQL service-backed integration tests — PASS
  - `go test -race ./...` — PASS

### Safety Boundary

This milestone is limited to shutdown ordering and runtime ownership protection. The catalog lifecycle remains operational-only; database ownership remains single-shot and closes transaction storage before audit storage when handles are distinct. No new financial authority or provider recovery behavior is introduced.

### Known Limitations

- `catalogWorkerLifecycle.Shutdown()` remains void-returning in production; catalog completion errors remain an internal test seam;
- PostgreSQL integration coverage requires `DESKAPROVIDER_POSTGRES_DSN` and is skipped when unavailable;
- database close remains non-context-aware;
- the guard protects direct `Service.Close()`; normal `Service.Run()` shutdown continues to own lifecycle completion before invoking `Close()`.

### Next Milestone

**#186 — Runtime Shutdown Error Identity Across Direct Close Guards**

Focus next on stable error identity and cleanup-error preservation when direct `Service.Close()` is attempted during active auxiliary lifecycle shutdown, including repeated calls after the lifecycle transitions to stopped, without introducing new provider or transaction recovery behavior.


## 185. Milestone Update — Runtime Shutdown Completion Ordering & Error Precedence Review

**Date:** 2026-09-27

Completed:

- reviewed the final runtime shutdown ordering across balance-worker completion, catalog completion, and owned database cleanup;
- added deterministic regression coverage proving the shutdown sequence remains:
  1. balance worker completion;
  2. catalog lifecycle completion;
  3. transaction database close;
  4. dedicated audit database close;
- verified that primary context cancellation remains discoverable together with independent balance shutdown, catalog shutdown-completion, transaction close, and audit close errors;
- verified repeated `Service.Close()` preserves database cleanup error identity without re-closing owned resources;
- kept the production shutdown implementation unchanged because the existing composition already satisfied the reviewed ordering and precedence contract;
- corrected the regression fixture after CI exposed an invalid test setup that attempted catalog sync with a nil registry; the final fixture isolates the shutdown path directly;
- no provider retry/failover, transaction resubmission, ledger mutation, customer-balance mutation, treasury movement, provider funding, or synthetic transaction/audit reconstruction was introduced.

### CI Failure and Correction

CI #1709 on test commit `9ff26451301cd8fde98881d889ca41bfac089d34` was **RED** because the regression fixture populated `catalogSync` with a service whose registry was nil, causing the test to panic during the initial catalog sync instead of reaching shutdown.

The fixture was corrected in commit `3809c3b280cc48bd0669f81f2e2cbc6a3a5f135a` by leaving catalog sync unset so `Service.Run()` enters the cancellation/shutdown path directly.

### Verification

- Initial regression/test fixture commit: `9ff26451301cd8fde98881d889ca41bfac089d34`.
- CI #1709: RED — invalid catalog-sync test fixture.
- Corrected runtime/test HEAD: `3809c3b280cc48bd0669f81f2e2cbc6a3a5f135a`.
- CI #1711 on exact corrected HEAD: **GREEN**.
  - `go test ./...` — PASS
  - `go vet ./...` — PASS
  - PostgreSQL service-backed integration tests — PASS
  - `go test -race ./...` — PASS

### Safety Boundary

This milestone is limited to runtime shutdown ordering, lifecycle completion, error composition, and owned database cleanup. It does not change transaction authorization or persistence authority. Transaction state remains authoritative and audit remains observational.

### Known Limitations

- the production `catalogWorkerLifecycle.Shutdown()` contract remains void-returning; catalog completion error composition remains exercised through the internal test seam;
- PostgreSQL integration coverage requires `DESKAPROVIDER_POSTGRES_DSN` and is skipped when unavailable;
- database close remains non-context-aware;
- this milestone does not introduce a new production error source.

### Next Milestone

**#186 — Runtime Shutdown Idempotence Under Repeated Run/Close Boundaries**

Focus next on repeated shutdown entry points after `Service.Run()` completion, ensuring lifecycle state, owned database cleanup, and stable error identity remain single-shot across repeated `Run()`/`Close()` boundaries without introducing new provider or transaction recovery behavior.


## 186. Milestone Update — Runtime Shutdown Idempotence Under Repeated Run/Close Boundaries

**Date:** 2026-09-27

Completed:

- identified that a completed Service.Run() could previously be entered again after transferred runtime database ownership had already been closed;
- added a stable ErrServiceClosed sentinel for the closed-runtime boundary;
- added a lock-protected runtime ownership state check before Service.Run() starts worker lifecycle execution;
- repeated Service.Run() after owned shutdown now returns ErrServiceClosed without restarting the balance lifecycle or attempting to reuse closed database ownership;
- repeated Service.Close() remains single-shot and preserves the original database cleanup error identity;
- added regression coverage for the complete run → shutdown → repeated run → repeated close sequence;
- preserved the existing shutdown ordering and independent error composition from milestone #185;
- no provider retry/failover, transaction resubmission, ledger mutation, customer-balance mutation, treasury movement, provider funding, or synthetic transaction/audit reconstruction was introduced.

### Verification

- Runtime implementation commit: 3afad2d229e432dd0dddf255e0ea3067b5bb0120.
- Regression test commit: 03b6e43484d0d8b20d656b66de8ed9cf47e98b2d.
- CI #1717 on exact implementation/test HEAD: **GREEN**.
  - go test ./... — PASS
  - go vet ./... — PASS
  - PostgreSQL service-backed integration tests — PASS
  - go test -race ./... — PASS

### Safety Boundary

This milestone is limited to runtime lifecycle reuse prevention after owned database shutdown. The closed-state check does not alter transaction persistence authority, audit authority, provider execution, routing, retry/failover, treasury, or funding behavior.

### Known Limitations

- the single-use guard is tied to transferred runtime database ownership; services constructed without database ownership can still be run according to their existing lifecycle semantics;
- catalogWorkerLifecycle.Shutdown() remains void-returning in production; catalog completion error composition remains an internal test seam;
- PostgreSQL integration coverage requires DESKAPROVIDER_POSTGRES_DSN and is skipped when unavailable;
- database close remains non-context-aware.

### Next Milestone

**#187 — Runtime Close-State Error Composition Across Direct and Run-Owned Shutdown**

Focus next on stable closed-state error identity and cleanup-error preservation across direct Service.Close(), Run-owned shutdown, and repeated close attempts, without introducing new provider or transaction recovery behavior.


## 185. Milestone Update — Runtime Shutdown Completion Ordering & Error Precedence Review

**Date:** 2026-09-27

Completed:

- reviewed the full runtime shutdown path after lifecycle startup and cancellation;
- confirmed the production completion ordering is deterministic:
  1. balance worker completion,
  2. catalog lifecycle completion,
  3. transaction database close,
  4. audit database close;
- confirmed shutdown error composition preserves the primary runtime error plus balance shutdown error, catalog shutdown completion error, and database cleanup errors through `errors.Join` composition;
- confirmed repeated `Service.Close()` does not replay catalog or balance completion errors and does not double-close owned transaction/audit database handles;
- confirmed repeated `Service.Run()` after transferred runtime ownership has been closed returns `ErrServiceClosed` without restarting lifecycle workers;
- retained the existing deterministic regression coverage in `runtime_test.go` for completion ordering, error identity, repeated close, and repeated run behavior;
- removed only an accidentally duplicated copy of the existing shutdown-ordering regression test introduced during milestone review; no production runtime behavior was changed by this cleanup;
- no provider retry/failover, transaction resubmission, ledger mutation, customer-balance mutation, treasury movement, funding, or synthetic transaction/audit reconstruction was introduced.

### Verification

- Final source/test cleanup commit: `6a440e66527031cd7e98227e53966fc9533b406f`.
- CI #1723: RED — duplicate regression test introduced during review caused a compile-time redeclaration.
- CI #1725: RED — follow-up cleanup temporarily left an orphaned duplicate test body.
- CI #1727: RED — follow-up cleanup still had an incomplete test-file boundary.
- CI #1729 on exact restored HEAD: **GREEN**.
  - `go test ./...` — PASS
  - `go vet ./...` — PASS
  - PostgreSQL service-backed integration tests — PASS
  - `go test -race ./...` — PASS

### Safety Boundary

This milestone is a shutdown ordering/error-precedence review only. The authoritative transaction-state boundary, append-only audit boundary, runtime database ownership transfer, and single-shot cleanup semantics remain unchanged. No recovery authority was added across transaction, audit, provider, ledger, treasury, or funding domains.

### Known Limitations

- production `catalogWorkerLifecycle.Shutdown()` remains void-returning; catalog completion error composition continues to be validated through the internal test seam;
- PostgreSQL integration coverage requires `DESKAPROVIDER_POSTGRES_DSN` and is skipped when unavailable;
- database close remains non-context-aware;
- no new production catalog error source was introduced.

### Next Milestone

**#186 — Runtime Shutdown Idempotence Across Repeated Run/Close Interleavings**

Focus next on repeated `Run()` / `Close()` interleavings after partial and completed shutdown, preserving lifecycle single-shot semantics, ownership closure, stable error identity, and the existing recovery safety boundary.


## 185. Milestone Update — Runtime Shutdown Completion Ordering & Error Precedence Review

**Date:** 2026-09-27

Completed:

- reviewed the final runtime shutdown ordering across the balance worker, catalog lifecycle, and owned database resources;
- confirmed and regression-tested the shutdown completion order: balance worker, catalog lifecycle, transaction database, then audit database;
- added regression coverage for error precedence across the first `Service.Run()` shutdown and repeated `Service.Close()`;
- preserved all first-shutdown error identities: context cancellation, balance completion error, catalog completion error, transaction cleanup error, and audit cleanup error;
- verified that repeated `Service.Close()` does not replay the catalog lifecycle completion error;
- verified that repeated `Service.Close()` continues to expose persisted transaction/audit cleanup errors while underlying database handles remain single-shot;
- no provider retry/failover, transaction resubmission, ledger mutation, customer-balance mutation, treasury movement, provider funding, or synthetic transaction/audit reconstruction was introduced.

### Verification

- Implementation/regression-test commit: `5706098c42342348523ffd4b0eaf48b4a2c56244`.
- CI #1733 on exact implementation/test HEAD: **GREEN**.
  - `go test ./...` — PASS
  - `go vet ./...` — PASS
  - PostgreSQL service-backed integration tests — PASS
  - `go test -race ./...` — PASS
- Final documentation commit is verified separately below before milestone closure.

### Safety Boundary

This milestone is limited to runtime shutdown ordering, error composition, and repeated-close semantics. Transaction persistence remains authoritative for transaction state/idempotency; audit persistence remains append-only observational evidence. No recovery authority or financial movement behavior was added.

### Known Limitations

- the production `catalogWorkerLifecycle.Shutdown()` contract remains void-returning; catalog completion error injection remains an internal test seam used only to validate runtime error composition;
- PostgreSQL integration coverage requires `DESKAPROVIDER_POSTGRES_DSN` and is skipped when unavailable;
- database close remains non-context-aware;
- repeated `Service.Close()` exposes stored database cleanup errors but does not replay lifecycle completion errors, by design.

### Next Milestone

**#186 — Runtime Shutdown Ownership State & Re-entry Guard Review**

Focus next on explicit runtime ownership state transitions around shutdown/re-entry, including repeated `Run()`, repeated `Close()`, and lifecycle completion boundaries, without expanding provider or transaction recovery authority.


## 185. Milestone Update — Runtime Shutdown Completion Ordering & Error Precedence Review

**Date:** 2026-09-27

Completed:

- reviewed the final shutdown ordering contract across balance-worker completion, catalog completion, and runtime database ownership closure;
- added deterministic coverage for a balance-worker shutdown timeout while the worker is still owned/running;
- verified that a shutdown timeout preserves the worker ownership boundary and prevents `Service.Close()` from closing runtime databases while the worker remains active;
- verified that database cleanup becomes available only after the worker has completed and lifecycle ownership has been released;
- preserved existing ordering: balance completion first, catalog completion second, transaction database cleanup third, audit database cleanup fourth;
- preserved error identity composition for cancellation, balance shutdown, catalog completion, transaction cleanup, and audit cleanup;
- preserved single-shot database cleanup and non-replay of catalog completion errors on repeated `Service.Close()`;
- no provider retry/failover, transaction resubmission, ledger mutation, customer-balance mutation, treasury movement, provider funding, or synthetic transaction/audit reconstruction was introduced.

### CI Failure and Correction

CI #1737 on test commit `87359e2a03ec3a69c1904ec86f00bdcd566190c0` was **RED** because the first fixture assumed that a shutdown seam returning an error would leave the worker running. In the real runtime path, cancellation of the parent context also allowed the worker to exit normally, so that fixture did not model a genuine shutdown-timeout condition.

The regression test was corrected at `cb537ff0a7b41efa5721e0e239c45578a9c68d6b` using a deterministic blocking balance provider. CI #1739 on that exact corrected HEAD is **GREEN**.

### Verification

- Initial test commit: `87359e2a03ec3a69c1904ec86f00bdcd566190c0`.
- CI #1737: RED — fixture did not model a worker that remains running after shutdown timeout.
- Corrected test commit: `cb537ff0a7b41efa5721e0e239c45578a9c68d6b`.
- CI #1739 on exact corrected HEAD: **GREEN**.
  - `go test ./...` — PASS
  - `go vet ./...` — PASS
  - PostgreSQL service-backed integration tests — PASS
  - `go test -race ./...` — PASS

### Safety Boundary

This milestone is limited to runtime lifecycle ownership, shutdown ordering, error composition, and database-close guards. A worker that has not completed shutdown retains ownership of the runtime process boundary, so database cleanup is deferred until lifecycle completion.

The change does not introduce provider retry/failover, transaction resubmission, ledger mutation, treasury movement, provider funding, or cross-domain recovery authority.

### Known Limitations

- the production `catalogWorkerLifecycle.Shutdown()` contract remains void-returning; the catalog completion error channel remains an internal test seam;
- PostgreSQL integration coverage requires `DESKAPROVIDER_POSTGRES_DSN` and is skipped when unavailable;
- database close remains non-context-aware;
- shutdown-timeout behavior remains bounded by the caller-provided shutdown context.

### Next Milestone

**#186 — Runtime Shutdown Failure Isolation Across Auxiliary Lifecycle Boundaries**

Focus next on isolating independent balance/catalog lifecycle failures so one auxiliary completion failure cannot suppress another completion attempt or runtime ownership cleanup, while preserving the existing transaction/audit persistence boundary and single-shot cleanup.


## 185. Milestone Update — Runtime Shutdown Completion Ordering & Error Precedence Review

**Date:** 2026-09-27

Completed:

- reviewed the final shutdown ordering across the balance worker, catalog lifecycle, and runtime database ownership;
- added regression coverage for the catalog-start failure path, where rollback must complete in deterministic order: balance rollback, catalog completion, transaction database close, then audit database close;
- verified that primary catalog-start failure, balance rollback failure, catalog completion failure, transaction cleanup failure, and audit cleanup failure all remain discoverable through the returned error;
- verified repeated `Service.Close()` preserves database cleanup errors without replaying lifecycle completion errors and without double-closing either database;
- confirmed the existing successful shutdown path preserves the same lifecycle-to-database ordering and repeated-close semantics;
- no production behavior change was required for the ordering contract; the milestone closes the remaining regression-coverage gap around the catalog-start rollback boundary;
- no provider retry/failover, transaction resubmission, ledger mutation, customer-balance mutation, treasury movement, or synthetic transaction/audit reconstruction was introduced.

### Verification

- Regression test commit: `ccfcf4c82c1f00e192b2c1443e9a1f504ff08263`.
- CI #1743 on exact test HEAD: **GREEN**.
  - `go test ./...` — PASS
  - `go vet ./...` — PASS
  - PostgreSQL service-backed integration tests — PASS
  - `go test -race ./...` — PASS

### Safety Boundary

This milestone is limited to shutdown ordering, error identity/composition, lifecycle completion, and existing runtime database ownership cleanup. It does not introduce new provider execution, retry/failover, transaction recovery, ledger authority, treasury movement, or funding behavior.

### Known Limitations

- the production `catalogWorkerLifecycle.Shutdown()` contract remains void-returning; completion error injection remains an internal test seam;
- PostgreSQL integration coverage requires `DESKAPROVIDER_POSTGRES_DSN` and is skipped when unavailable;
- database close remains non-context-aware;
- error composition uses Go error joining rather than a single exclusive precedence winner, so callers should use error identity checks rather than parse a combined error string.

### Next Milestone

**#186 — Runtime Shutdown Reentrancy & Ownership Guard Matrix**

Focus next on repeated/concurrent shutdown entry points and ownership guards, including `Service.Close()`, completed `Run()`, shutdown timeout, and concurrent lifecycle completion, while preserving single-shot database ownership cleanup and avoiding new provider/transaction recovery behavior.


## 186. Milestone Update — Runtime Shutdown Reentrancy & Ownership Guard Matrix

**Date:** 2026-09-27

Completed:

- serialized runtime shutdown completion, direct `Service.Close()`, and `Service.Run()` re-entry through a dedicated runtime shutdown mutex;
- moved the initial closed-ownership check and balance-worker startup under the same serialization boundary so a concurrent `Close()` cannot close transferred database ownership between the closed-state check and worker start;
- changed the composed Run shutdown path to complete catalog lifecycle shutdown and database ownership cleanup while holding the same boundary, preventing direct `Close()` from closing databases during an in-progress lifecycle completion;
- preserved the existing shutdown ordering: balance worker completion, catalog completion, transaction database cleanup, then audit database cleanup;
- added regression coverage for concurrent `Close()` and `Run()` re-entry while catalog shutdown completion is deliberately blocked;
- verified concurrent callers cannot observe an intermediate state or close runtime databases before the active shutdown completion finishes;
- verified the re-entry attempt observes `ErrServiceClosed` after the first owned shutdown completes;
- no provider retry/failover, transaction resubmission, ledger mutation, customer-balance mutation, treasury movement, provider funding, or synthetic transaction/audit reconstruction was introduced.

### CI Failure and Correction

CI #1749 on implementation/test commit `2bd5f63cc0e1a1d9c24c8a5971ca4ad9fe877258` was **RED** because the new interleaving fixture omitted `catalogInterval`, causing `time.NewTicker(0)` to panic in the test.

The fixture was corrected at `9e160b10373eb6614b11e84737ddc5dc2c0bc944` by setting a valid catalog interval. The runtime serialization change itself was retained unchanged.

### Verification

- Runtime implementation commit: `769c4fbe9c3b62a7bb8e44c1b52d2622de334646`.
- Initial regression test commit: `2bd5f63cc0e1a1d9c24c8a5971ca4ad9fe877258`.
- Corrected regression test commit / exact HEAD: `9e160b10373eb6614b11e84737ddc5dc2c0bc944`.
- CI #1749: RED — test fixture panic from zero catalog interval.
- CI #1751 on exact corrected HEAD: **GREEN**.
  - `go test ./...` — PASS
  - `go vet ./...` — PASS
  - PostgreSQL service-backed integration tests — PASS
  - `go test -race ./...` — PASS

### Safety Boundary

This milestone is limited to runtime lifecycle serialization, shutdown re-entry, and database ownership closure ordering. The mutex does not create transaction or audit recovery authority and does not alter the authoritative transaction-state/idempotency boundary or append-only audit boundary.

The change does not introduce provider retry/failover, transaction resubmission, ledger mutation, treasury movement, provider funding, or cross-domain recovery behavior.

### Known Limitations

- production `catalogWorkerLifecycle.Shutdown()` remains void-returning; catalog completion error composition continues to use the existing internal test seam;
- PostgreSQL integration coverage requires `DESKAPROVIDER_POSTGRES_DSN` and is skipped when unavailable;
- database close remains non-context-aware;
- services constructed without transferred runtime database ownership retain their existing lifecycle semantics.

### Next Milestone

**#187 — Runtime Close-State Error Composition Across Direct and Run-Owned Shutdown**

Focus next on stable closed-state error identity and cleanup-error preservation across direct `Service.Close()`, Run-owned shutdown, and repeated close attempts, without introducing new provider or transaction recovery behavior.


## 185. Milestone Update — Runtime Shutdown Completion Ordering & Error Precedence Review

**Date:** 2026-09-27

Completed:

- reviewed the final runtime shutdown ordering across balance-worker completion, catalog completion, and runtime database ownership closure;
- confirmed the existing full lifecycle shutdown order is `balance -> catalog -> transaction database -> audit database`;
- added regression coverage for the balance-only shutdown path, where catalog synchronization is not active;
- verified balance completion errors remain discoverable together with the primary context cancellation and transaction/audit cleanup errors;
- verified balance-only shutdown performs database cleanup only after the balance worker completion boundary;
- verified transaction cleanup remains before audit cleanup;
- verified repeated `Service.Close()` preserves database cleanup error identity without replaying lifecycle completion errors;
- no provider retry/failover, transaction resubmission, ledger mutation, customer-balance mutation, treasury movement, provider funding, or synthetic transaction/audit reconstruction was introduced.

### Verification

- Regression test commit: `83beaf6480affbf11aa78ac9f12ee31667daf379`.
- CI #1755 on exact test HEAD: **GREEN**.
  - `go test ./...` — PASS
  - `go vet ./...` — PASS
  - PostgreSQL service-backed integration tests — PASS
  - `go test -race ./...` — PASS

### Safety Boundary

This milestone is limited to shutdown ordering, error identity, and ownership cleanup verification. It does not alter transaction authority, audit authority, provider retry/failover behavior, provider funding, customer ledger semantics, or cross-domain recovery.

### Known Limitations

- the production `catalogWorkerLifecycle.Shutdown()` contract remains void-returning; catalog completion errors remain available only through the internal test seam;
- PostgreSQL integration coverage requires `DESKAPROVIDER_POSTGRES_DSN` and is skipped when unavailable;
- database close remains non-context-aware.

### Next Milestone

**#186 — Runtime Shutdown Completion Under Partial Lifecycle Start**

Focus next on partial-start combinations where balance or catalog startup fails at different lifecycle boundaries, ensuring only successfully started lifecycle components are completed and database ownership cleanup remains single-shot.


### 185. Milestone Update — Runtime Shutdown Completion Ordering & Error Precedence Review

**Date:** 2026-09-27

Completed:

- reviewed the complete Service shutdown composition after the repeated lifecycle-completion hardening in milestone #184;
- verified the shutdown boundary remains deterministic: primary lifecycle error first, balance-worker completion error second, catalog completion error third, transaction database cleanup fourth, and audit database cleanup fifth;
- added a focused regression test locking the deterministic composed error order while also verifying every underlying error remains discoverable through `errors.Is`;
- retained the existing lifecycle ordering coverage proving balance shutdown completes before catalog shutdown and both complete before transaction/audit database cleanup;
- retained the existing repeated `Service.Close()` coverage proving lifecycle completion errors are not replayed while recorded database cleanup errors remain observable and database resources remain single-shot;
- no production runtime behavior change was required beyond the #184 single-shot catalog completion fix;
- no provider retry/failover, transaction resubmission, ledger mutation, treasury movement, provider funding, or cross-domain recovery authority was introduced.

### Verification

- regression test commit: `1d4947c1ccccf42e5921f2db28490a371dd3fd7a`;
- CI #1759 on exact test HEAD: **GREEN**;
  - `go test ./...` — PASS
  - `go vet ./...` — PASS
  - PostgreSQL integration — PASS
  - `go test -race ./...` — PASS

### Safety Boundary

The shutdown ordering is an infrastructure lifecycle contract only. Error precedence/composition does not authorize provider execution, transaction retry, failover, resubmission, ledger mutation, treasury movement, or provider funding. Transaction persistence remains authoritative for transaction state; audit remains observational evidence.

### Known Limitations

- deterministic error ordering is an application-level `errors.Join` composition contract and does not imply any additional financial or provider semantics;
- `databaseCloser.Close()` remains non-context-aware;
- PostgreSQL integration remains environment-gated by `DESKAPROVIDER_POSTGRES_DSN`.

### Next Milestone

**#186 — Runtime Shutdown Re-entry & Post-Completion State Review:** verify repeated/concurrent `Run()` and `Close()` calls after completed shutdown remain stable, single-shot, and preserve the recorded cleanup result without reopening lifecycle resources.


## 186. Milestone Update — Runtime Shutdown Reentrancy, Partial Lifecycle Completion & Ownership Guard Matrix

**Date:** 2026-09-27

Completed:

- hardened the runtime shutdown boundary so balance-only shutdown keeps the same `shutdownMu` serialization through worker completion and database ownership cleanup;
- preserved the invariant that direct `Service.Close()` cannot close runtime-owned databases while an active lifecycle completion callback is still executing;
- verified partial lifecycle-start behavior: an unstarted catalog lifecycle is not completed after catalog startup failure, while successfully started balance lifecycle work is rolled back;
- verified repeated/concurrent `Run()` and `Close()` calls preserve single-shot database ownership cleanup and stable closed-state behavior;
- preserved the existing shutdown ordering: balance completion, catalog completion, transaction database cleanup, audit database cleanup;
- retained worker deadline identity after a shutdown timeout instead of converting `context.DeadlineExceeded` into a false clean-success result;
- no provider retry/failover, transaction resubmission, ledger mutation, customer-balance mutation, treasury movement, provider funding, or synthetic transaction/audit reconstruction was introduced.

### CI Failure and Corrections

The exact branch HEAD was checked before closure and required three correction rounds:

1. CI #1787 on `355e570073086b085632ef8b87dfaa7fe73dd8d6` — **RED**.
   - A regression fixture replaced a valid catalog service with an empty `catalog.SyncService`, causing a nil registry panic.
2. CI #1789 on `0ecbdbe6ca891f8fe860b9e659e2c3da6f59ea5c` — **RED**.
   - A timeout fixture expected a clean worker shutdown, but the worker correctly preserved its parent `context.DeadlineExceeded` result.
   - The same run exposed a real balance-only shutdown race: `Close()` could observe the worker as stopped while the shutdown completion callback was still executing.
3. CI #1793 on `4d2d05c0e6a122d6b878f7dafc20350f35eb1e00` — **RED**.
   - The production serialization fix compiled with an obsolete unused shutdown wrapper.

The corrections were:
- regression fixture cleanup in `7e50ac031be9176d538700aa84d90aeea385ccc7`;
- production balance-only shutdown serialization in `4d2d05c0e6a122d6b878f7dafc20350f35eb1e00`;
- removal of the obsolete wrapper in `d121c2b16d2e32d295ee859ab56324e530949bec`.

### Verification

- Final exact branch HEAD: `d121c2b16d2e32d295ee859ab56324e530949bec`.
- CI #1795 on exact final HEAD: **GREEN**.
  - `go test ./...` — PASS
  - `go vet ./...` — PASS
  - PostgreSQL service-backed integration tests — PASS
  - `go test -race ./...` — PASS

### Safety Boundary

This milestone is limited to runtime lifecycle serialization, partial-start rollback, shutdown reentrancy, and database ownership closure. It does not create transaction or audit recovery authority and does not alter the authoritative transaction-state/idempotency boundary or append-only audit boundary.

The change does not introduce provider retry/failover, transaction resubmission, ledger mutation, treasury movement, provider funding, or cross-domain recovery behavior.

### Known Limitations

- production `catalogWorkerLifecycle.Shutdown()` remains void-returning; catalog completion error composition continues through the existing internal test seam;
- PostgreSQL integration coverage requires `DESKAPROVIDER_POSTGRES_DSN` and is skipped when unavailable;
- database close remains non-context-aware;
- worker shutdown preserves its originating context error when the worker's parent context expires.

### Next Milestone

**#187 — Runtime Close-State Error Composition Across Direct and Run-Owned Shutdown**

Focus next on stable closed-state error identity and cleanup-error preservation across direct `Service.Close()`, Run-owned shutdown, and repeated close attempts, without introducing new provider or transaction recovery behavior.


## 185. Milestone Update — Runtime Shutdown Completion Ordering & Error Precedence Review

**Date:** 2026-09-27

Completed:

- reviewed the final shutdown ordering across the balance worker, catalog lifecycle, and runtime-owned transaction/audit databases;
- added regression coverage that locks the successful shutdown completion order to `balance → catalog → transaction database → audit database`;
- verified repeated `Service.Close()` does not replay lifecycle completion callbacks or double-close runtime-owned databases;
- preserved shutdown error composition and `errors.Is` identity for primary lifecycle and database cleanup errors;
- confirmed database closure remains conditional on both started lifecycles being stopped;
- no provider retry/failover, transaction resubmission, ledger mutation, treasury movement, provider funding, or synthetic transaction/audit reconstruction was introduced.

### Verification

- Implementation/test commit: `1c07cda6a594badb28a38e5493896d0533247036`.
- CI #1799 on exact test HEAD: **GREEN**.
  - `go test ./...` — PASS
  - `go vet ./...` — PASS
  - PostgreSQL service-backed integration tests — PASS
  - `go test -race ./...` — PASS

### Safety Boundary

This milestone only formalizes runtime shutdown ordering and error-precedence observability. Transaction persistence remains authoritative; audit persistence remains observational. Database cleanup remains an infrastructure lifecycle operation and cannot authorize provider retry/failover/resubmission, ledger mutation, treasury movement, or provider funding.

### Known Limitations

- the production `catalogWorkerLifecycle.Shutdown()` contract remains void-returning; completion error injection remains an internal test seam;
- PostgreSQL integration coverage requires `DESKAPROVIDER_POSTGRES_DSN` and is skipped when unavailable;
- database close remains non-context-aware.

### Next Milestone

**#186 — Runtime Shutdown Failure Ordering Under Partial Worker Completion**

Focus next on partial completion/timeout boundaries where one lifecycle stops successfully while another remains running, preserving database ownership until all started lifecycles are safely stopped and keeping all primary/cleanup error identities observable.


## 187. Milestone Update — Runtime Close-State Error Composition Across Direct and Run-Owned Shutdown

**Date:** 2026-09-27

Completed:

- added regression coverage for the closed-state contract across both direct `Service.Close()` and Run-owned shutdown;
- verified a direct `Service.Close()` records and preserves the underlying database cleanup error across repeated `Close()` calls;
- verified a Run-owned shutdown preserves both the primary cancellation error and the recorded database cleanup error;
- verified repeated `Close()` after Run-owned shutdown preserves the recorded cleanup error without replaying lifecycle completion errors;
- verified any subsequent `Run()` after runtime ownership is already closed returns the stable `ErrServiceClosed` state error rather than replaying the historical cleanup error;
- no production runtime refactor was required; this milestone locks the existing closed-state/error-identity contract with regression coverage;
- no provider retry/failover, transaction resubmission, ledger mutation, customer-balance mutation, treasury movement, provider funding, or synthetic transaction/audit reconstruction was introduced.

### Verification

- Regression test commit: `30a55b46e0d355a899c45bb60a09766e5538866e`.
- CI #1803 on exact test HEAD: **GREEN**.
  - `go test ./...` — PASS
  - `go vet ./...` — PASS
  - PostgreSQL service-backed integration tests — PASS
  - `go test -race ./...` — PASS

### Safety Boundary

This milestone only verifies runtime closed-state identity and preservation of recorded infrastructure cleanup errors. `ErrServiceClosed` is a lifecycle state signal and is not a provider result, transaction authorization signal, ledger instruction, or recovery authority.

Transaction persistence remains authoritative for transaction state/idempotency, while audit persistence remains observational evidence. Database cleanup remains an infrastructure lifecycle operation.

### Known Limitations

- the production `catalogWorkerLifecycle.Shutdown()` contract remains void-returning; catalog completion errors continue through the existing internal test seam;
- PostgreSQL integration coverage requires `DESKAPROVIDER_POSTGRES_DSN` and is skipped when unavailable;
- database close remains non-context-aware.

### Next Milestone

**#188 — Runtime Close-State Error Precedence Across Mixed Lifecycle and Database Failures**

Focus next on mixed shutdown outcomes where lifecycle completion, cancellation, and database cleanup failures coexist, preserving stable `errors.Is` identity and closed-state behavior without introducing new provider or transaction recovery behavior.


## 188. Milestone Update — Runtime Close-State Error Precedence Across Mixed Lifecycle and Database Failures

**Date:** 2026-09-27

Completed:

- added regression coverage for a Run-owned shutdown where cancellation, balance-worker completion failure, catalog completion failure, transaction database cleanup failure, and audit database cleanup failure coexist;
- verified all five error identities remain discoverable through `errors.Is` from the composed Run error;
- verified repeated `Service.Close()` preserves recorded database cleanup errors without replaying historical lifecycle/cancellation errors;
- verified repeated `Service.Close()` remains single-shot for transaction and audit database cleanup;
- verified subsequent `Run()` after runtime ownership is closed returns stable `ErrServiceClosed` without replaying historical shutdown or cleanup errors;
- confirmed no production runtime refactor was required because the existing shutdown composition already provides the intended mixed-error boundary;
- no provider retry/failover, transaction resubmission, ledger mutation, customer-balance mutation, treasury movement, provider funding, or synthetic transaction/audit reconstruction was introduced.

### Verification

- Regression test commit: `590512e781478bea115284c433bbdb15c39e0c6a`.
- CI #1807 on exact test HEAD: **GREEN**.
  - `go test ./...` — PASS
  - `go vet ./...` — PASS
  - PostgreSQL service-backed integration tests — PASS
  - `go test -race ./...` — PASS

### Safety Boundary

This milestone is limited to runtime error composition and closed-state identity. Error joining preserves observable infrastructure/lifecycle failures but does not assign financial or provider authority to any of them.

Transaction persistence remains authoritative for transaction state/idempotency; audit persistence remains observational evidence. Database cleanup remains an infrastructure lifecycle operation.

### Known Limitations

- the production `catalogWorkerLifecycle.Shutdown()` contract remains void-returning; catalog completion errors continue through the existing internal test seam;
- PostgreSQL integration coverage requires `DESKAPROVIDER_POSTGRES_DSN` and is skipped when unavailable;
- database close remains non-context-aware;
- error composition uses Go error joining rather than a single exclusive precedence winner, so callers should use error identity checks rather than parse a combined error string.

### Next Milestone

**#189 — Runtime Shutdown Error Retention Across Repeated Close/Re-entry**

Focus next on persistence of the recorded cleanup result across repeated direct close, Run-owned shutdown, and re-entry boundaries, including mixed database cleanup failures, without introducing new provider or transaction recovery behavior.


## 185. Milestone Update — Runtime Shutdown Completion Ordering & Error Precedence Review

**Date:** 2026-09-27

Completed:

- added deterministic regression coverage for the final runtime shutdown ordering contract;
- verified shutdown completion order remains:
  `balance worker → catalog lifecycle → transaction database → audit database`;
- verified the first shutdown result preserves the primary cancellation error plus balance shutdown, catalog shutdown, transaction cleanup, and audit cleanup error identities;
- verified repeated `Service.Close()` does not replay historical lifecycle errors or repeat lifecycle completion;
- verified transaction and audit database handles are each closed exactly once;
- verified a subsequent `Service.Run()` rejects closed service state with `ErrServiceClosed` rather than replaying historical shutdown errors;
- no provider retry/failover, transaction resubmission, ledger mutation, customer-balance mutation, treasury movement, or synthetic transaction/audit reconstruction was introduced.

### Verification

- Regression test commit: `accdc6244823ff7913cfa3d805e57dc8ada5c916`.
- CI #1810 on exact test HEAD: **GREEN**.
  - `go test ./...` — PASS
  - `go vet ./...` — PASS
  - PostgreSQL service-backed integration tests — PASS
  - `go test -race ./...` — PASS

### Safety Boundary

This milestone is limited to runtime shutdown ordering, error identity, repeated lifecycle completion, and database ownership cleanup. It does not change transaction authority, audit authority, provider routing, provider retry/failover, funding, ledger behavior, or cross-domain recovery.

### Known Limitations

- the production catalog lifecycle completion API remains void-returning; error composition is validated through the internal runtime test seam;
- PostgreSQL integration coverage requires `DESKAPROVIDER_POSTGRES_DSN` and is skipped when unavailable;
- database close remains non-context-aware.

### Next Milestone

**#186 — Runtime Shutdown Context/Timeout Boundary**

Focus next on the boundary between shutdown context deadlines, worker completion, lifecycle completion, and database ownership closure, preserving the rule that database ownership must not be released while an owned worker remains running.


## 185. Milestone Update — Runtime Shutdown Completion Ordering & Error Precedence Review

**Date:** 2026-09-27

Completed:

- reviewed the final runtime shutdown ordering across balance-worker completion, catalog completion, and owned database cleanup;
- confirmed the production shutdown path composes completion in deterministic order: primary/context error, balance-worker shutdown error, catalog shutdown completion error, transaction database cleanup error, then audit database cleanup error;
- added regression coverage that validates the **actual `Service.Run()` error string ordering**, not only `errors.Is` identity or independent lifecycle event order;
- retained transaction-before-audit database cleanup ordering;
- retained single-shot database ownership cleanup and repeated `Service.Close()` error identity;
- retained the catalog single-completion boundary established by milestone #184;
- no provider retry/failover, transaction resubmission, ledger mutation, customer-balance mutation, treasury movement, provider funding, or synthetic transaction/audit reconstruction was introduced.

### Verification

- Test hardening commit: `6e5e50cf6d858689f3c8a5474052c1e5729b2cb8`.
- CI #1815 on exact test HEAD: **GREEN**.
  - `go test ./...` — PASS
  - `go vet ./...` — PASS
  - PostgreSQL service-backed integration tests — PASS
  - `go test -race ./...` — PASS
- Expected runtime error ordering is locked as:
  `context canceled`
  → `balance shutdown failed`
  → `catalog shutdown failed`
  → `close transaction database: ...`
  → `close audit database: ...`

### Safety Boundary

This milestone only hardens lifecycle shutdown ordering, error composition, and cleanup idempotence. It does not change transaction authority, provider execution semantics, recovery authority, or financial/ledger behavior.

### Known Limitations

- the production `catalogWorkerLifecycle.Shutdown()` contract remains void-returning; catalog completion errors are still represented through the internal test seam;
- PostgreSQL integration coverage requires `DESKAPROVIDER_POSTGRES_DSN` and is skipped when unavailable;
- database close remains non-context-aware;
- error ordering is deterministic within the runtime composition boundary, but external provider error semantics remain provider-specific.

### Next Milestone

**#186 — Runtime Shutdown Partial-Completion Safety**

Focus next on shutdown paths where one lifecycle reports an error or remains running, verifying that database ownership is not released prematurely and that repeated shutdown attempts preserve stable lifecycle and cleanup state, without introducing new provider or transaction recovery behavior.


## 185. Milestone Update — Runtime Shutdown Completion Ordering & Error Precedence Review

**Date:** 2026-09-27

Completed:

- reviewed the final runtime shutdown ordering across the balance worker, catalog lifecycle, transaction database ownership, and audit database ownership;
- confirmed the composed shutdown boundary preserves the deterministic completion order:
  `balance -> catalog -> transaction database -> audit database`;
- added regression coverage proving a catalog shutdown-completion error does not suppress transaction/audit database cleanup errors;
- verified all shutdown errors remain discoverable through `errors.Is`, including the primary cancellation error, catalog completion error, and independent database cleanup errors;
- verified repeated `Service.Close()` preserves recorded database cleanup errors without replaying historical lifecycle/primary errors and without double-closing databases;
- kept runtime ownership closure conditional on both lifecycle boundaries being stopped, preventing database ownership from being released while an active worker remains;
- no provider retry/failover, transaction resubmission, ledger mutation, treasury movement, provider funding, or synthetic transaction/audit reconstruction was introduced.

### CI Failure and Correction

CI #1819 on test commit `3ce6ffe73f7caef40cb2810f2874e38bd0782c06` was **RED** because the new test fixture left `catalogInterval` at its zero value while entering the catalog worker loop, causing `time.NewTicker` to panic with a non-positive interval.

Fix applied:

- initialized the test fixture's catalog interval to `time.Hour`;
- production runtime behavior was not changed.

### Verification

- Initial regression test commit: `3ce6ffe73f7caef40cb2810f2874e38bd0782c06`.
- CI #1819: RED — test fixture used a zero catalog interval.
- Corrected test commit: `1202889788ba8dbebe995519a1e2effdae561dfa`.
- CI #1821 on exact corrected HEAD: **GREEN**.
  - `go test ./...` — PASS
  - `go vet ./...` — PASS
  - PostgreSQL service-backed integration tests — PASS
  - `go test -race ./...` — PASS

### Safety Boundary

This milestone is limited to lifecycle shutdown ordering, error identity/composition, and runtime database ownership closure. Transaction persistence remains authoritative for transaction state and idempotency; audit persistence remains observational. The change does not create any new recovery authority or provider-side transaction retry/failover behavior.

### Known Limitations

- the production `catalogWorkerLifecycle.Shutdown()` contract remains void-returning; catalog completion error composition is still exercised through the internal test seam;
- PostgreSQL integration coverage requires `DESKAPROVIDER_POSTGRES_DSN` and is skipped when unavailable;
- database close remains non-context-aware;
- this milestone does not add a new production catalog error source.

### Next Milestone

**#186 — Runtime Shutdown Ownership Boundary Under Repeated Run/Close Attempts**

Focus next on repeated `Run()`/`Close()` attempts after partial and completed shutdown states, preserving `ErrServiceClosed`, single-shot database ownership cleanup, lifecycle idempotence, and historical error separation without introducing provider or transaction recovery behavior.


## 185. Milestone Update — Runtime Shutdown Completion Ordering & Error Precedence Review

**Date:** 2026-09-27

Completed:

- reviewed the runtime shutdown ordering contract across balance-worker completion, catalog completion, and owned database closure;
- preserved the invariant that runtime database ownership is closed only after all lifecycle components owned by the current shutdown path are confirmed stopped;
- added regression coverage for a catalog completion error that intentionally leaves the catalog lifecycle active;
- verified that the primary cancellation error and catalog completion error remain observable while database cleanup is withheld;
- verified that explicit `Service.Close()` also refuses to close active catalog-owned resources in this injected failure scenario;
- preserved transaction-before-audit database cleanup ordering and single-shot ownership cleanup;
- preserved repeated `Service.Close()` behavior and closed-state rejection semantics from the existing shutdown matrix;
- no provider retry/failover, transaction resubmission, ledger mutation, treasury movement, provider funding, or synthetic transaction/audit reconstruction was introduced.

### Safety Boundary

The shutdown path treats lifecycle completion as a prerequisite for database ownership release. A lifecycle completion error does not by itself authorize database closure when the lifecycle remains active.

The intended ordering remains:

```
balance lifecycle completion
        ↓
catalog lifecycle completion
        ↓
verify lifecycles stopped
        ↓
transaction database close
        ↓
audit database close
```

The database ownership guard remains single-shot, and repeated cleanup returns the recorded cleanup result without replaying lifecycle or primary shutdown errors.

### Verification

- Regression test commit: `706d0afac81870e3d9bd210bf965481c2c8dd9ea`.
- CI #1825 on exact test HEAD: **GREEN**.
  - `go test ./...` — PASS
  - `go vet ./...` — PASS
  - PostgreSQL service-backed integration tests — PASS
  - `go test -race ./...` — PASS
- Status documentation is updated in the follow-up commit and must itself pass the same CI gate before #185 is considered closed.

### Known Limitations

- the production catalog shutdown operation remains void-returning; completion-error behavior is still exercised through the internal runtime test seam;
- the injected failure test intentionally models a lifecycle that reports a completion error without stopping, which is a safety-boundary test rather than a claim about a current production catalog failure mode;
- database close remains non-context-aware;
- PostgreSQL integration coverage requires `DESKAPROVIDER_POSTGRES_DSN` and is skipped when unavailable.

### Next Milestone

**#186 — Runtime Shutdown Re-entry & Ownership State Convergence**

Focus next on repeated/concurrent shutdown entry after partial lifecycle completion, ensuring ownership state, lifecycle state, and recorded cleanup errors converge deterministically without reopening or resubmitting provider transactions.


## 186. Milestone Update — Runtime Shutdown Re-entry & Ownership State Convergence

**Date:** 2026-09-27

Completed:

- identified the partial-shutdown state where catalog lifecycle completion can report an error while the catalog lifecycle remains active;
- added an explicit `ErrServiceLifecycleActive` runtime boundary for re-entry attempts while an active catalog lifecycle remains after partial shutdown;
- preserved the existing `operational.ErrSyncWorkerRunning` behavior for re-entry while the balance lifecycle is already active;
- ensured rejected re-entry does not restart the balance worker and does not close runtime database ownership;
- added regression coverage for partial catalog shutdown followed by Run re-entry, manual lifecycle convergence, and final idempotent Service.Close();
- verified that database ownership remains open until lifecycle state converges to stopped;
- preserved single-shot database cleanup and historical cleanup-error semantics;
- no provider retry/failover, transaction resubmission, ledger mutation, treasury movement, provider funding, or synthetic transaction/audit reconstruction was introduced.

### Verification

- Runtime correction commit: `3097d0bf48d58763ef6e51a6b0bad6e0382191e1`.
- Regression test commit: `259889739a7bca8f5ee9aa216bd5732d75a05caa`.
- CI #1831 on exact code/test HEAD: **GREEN**.
  - `go test ./...` — PASS
  - `go vet ./...` — PASS
  - PostgreSQL service-backed integration tests — PASS
  - `go test -race ./...` — PASS

### Safety Boundary

The runtime now treats an active catalog lifecycle after partial shutdown as an ownership-state boundary. A new Run attempt is rejected before starting another balance lifecycle or releasing database ownership. Database closure remains conditional on lifecycle completion and remains single-shot.

### Known Limitations

- the production `catalogWorkerLifecycle.Shutdown()` contract remains void-returning; partial completion error behavior is still exercised through the internal runtime test seam;
- PostgreSQL integration coverage requires `DESKAPROVIDER_POSTGRES_DSN` and is skipped when unavailable;
- database close remains non-context-aware;
- this milestone does not introduce a new provider or transaction recovery authority.

### Next Milestone

**#187 — Runtime Shutdown Re-entry After Ownership Cleanup Failure**

Focus next on repeated Run/Close behavior when lifecycle completion succeeds but database ownership cleanup records an error, ensuring closed-state rejection, stable cleanup-error identity, and no lifecycle restart or double-close.


## 187. Milestone Update — Runtime Shutdown Re-entry After Ownership Cleanup Failure

**Date:** 2026-09-27

Completed:

- verified the runtime ownership boundary when lifecycle shutdown completes but database cleanup returns one or more errors;
- added regression coverage for simultaneous transaction and audit database cleanup failures after a cancellation-owned shutdown;
- verified the first `Service.Run()` result preserves the primary cancellation plus both independent database cleanup error identities;
- verified runtime database ownership is marked closed after the cleanup attempt and each owned database is closed exactly once;
- verified a subsequent `Service.Run()` is rejected with `ErrServiceClosed` and does not replay the historical cancellation or database cleanup errors;
- verified a subsequent `Service.Close()` preserves the recorded database cleanup errors without re-closing either database;
- preserved lifecycle idempotence and prevented any lifecycle restart after ownership cleanup has completed with an error;
- no provider retry/failover, transaction resubmission, ledger mutation, treasury movement, provider funding, or synthetic transaction/audit reconstruction was introduced.

### Verification

- Regression test commit: `f1af5e1a4345f00b9082ff1e825b2156d2f4a1bf`.
- CI #1835 on exact test HEAD: **GREEN**.
  - `go test ./...` — PASS
  - `go vet ./...` — PASS
  - PostgreSQL service-backed integration tests — PASS
  - `go test -race ./...` — PASS
- Existing runtime ownership implementation already records the cleanup result and closed state atomically under the ownership mutex; this milestone adds the regression boundary rather than changing production cleanup semantics.

### Safety Boundary

A database cleanup error after ownership closure is a terminal runtime ownership state, not a signal to reopen resources or restart lifecycle workers. Historical cleanup errors remain available through repeated `Service.Close()`, while a new `Run()` observes only the closed-state rejection.

Transaction persistence remains authoritative for transaction state and idempotency; audit persistence remains observational. No recovery authority is added.

### Known Limitations

- database close remains non-context-aware;
- PostgreSQL integration coverage requires `DESKAPROVIDER_POSTGRES_DSN` and is skipped when unavailable;
- this milestone validates runtime ownership/error semantics and does not add a retry or reconnection mechanism for failed database cleanup.

### Next Milestone

**#188 — Runtime Ownership Error Persistence Across Shared vs Dedicated Database Topologies**

Focus next on the same closed-state and cleanup-error contract across shared transaction/audit database ownership versus independently owned dedicated database handles, preserving single-shot cleanup and no lifecycle restart.


## 185. Milestone Update — Runtime Shutdown Completion Ordering & Error Precedence Review

**Date:** 2026-09-27

Completed:

- reviewed the final runtime shutdown completion order across the balance worker, catalog worker, and owned transaction/audit databases;
- confirmed the runtime shutdown sequence is balance lifecycle completion, catalog lifecycle completion, then transaction database cleanup followed by audit database cleanup;
- added deterministic regression coverage that locks the composed shutdown error ordering to the same sequence;
- verified the composed error preserves the primary cancellation error plus balance shutdown, catalog shutdown, transaction cleanup, and audit cleanup identities;
- verified database ownership cleanup remains single-shot after lifecycle completion;
- retained the existing rule that repeated Service.Close() preserves recorded database cleanup errors without replaying historical primary/lifecycle shutdown errors;
- no provider retry/failover, transaction resubmission, ledger mutation, customer-balance mutation, treasury movement, or synthetic transaction/audit reconstruction was introduced.

### Verification

- Regression test commit: `f69049d8dfac7d6b876e4cb240777dcfbf98859d`.
- CI #1839 on exact test HEAD: **GREEN**.
  - `go test ./...` — PASS
  - `go vet ./...` — PASS
  - PostgreSQL service-backed integration tests — PASS
  - `go test -race ./...` — PASS
- The final documentation commit must receive its own CI result before this milestone is considered closed.

### Safety Boundary

This milestone is limited to shutdown ordering and error-composition verification. Lifecycle completion remains separate from database ownership cleanup, and database cleanup remains transaction-before-audit. No new provider or transaction recovery authority is introduced.

### Known Limitations

- production `catalogWorkerLifecycle.Shutdown()` remains void-returning; catalog completion error injection remains an internal test seam;
- PostgreSQL integration coverage requires `DESKAPROVIDER_POSTGRES_DSN` and is skipped when unavailable;
- database close remains non-context-aware;
- the milestone validates the existing precedence/order contract and does not redesign runtime lifecycle APIs.

### Next Milestone

**#186 — Runtime Shutdown Partial-Completion Recovery Boundary**

Focus next on a lifecycle that fails to converge during shutdown, ensuring runtime ownership remains open, re-entry remains rejected, and explicit cleanup can converge safely without replaying historical shutdown errors.


## 185. Milestone Update — Runtime Shutdown Completion Ordering & Error Precedence Review

**Date:** 2026-09-27

Completed:

- reviewed the final runtime shutdown completion path across balance-worker shutdown, catalog shutdown, and owned database cleanup;
- confirmed the production shutdown path preserves the deterministic completion order: balance lifecycle, catalog lifecycle, transaction database, then audit database;
- added regression coverage for the complete composed error precedence order: primary shutdown error, balance lifecycle error, catalog lifecycle error, transaction cleanup error, then audit cleanup error;
- verified repeated `Service.Close()` preserves recorded database cleanup errors without replaying the primary or lifecycle completion errors;
- verified the existing closed-state boundary rejects re-entry after owned shutdown without replaying historical shutdown errors;
- retained the existing safety rule that database ownership is closed only after required lifecycles have stopped;
- no provider retry/failover, transaction resubmission, ledger mutation, customer-balance mutation, treasury movement, or synthetic transaction/audit reconstruction was introduced.

### Verification

- Test commit: `ef60661b0d613591bf47523292970faeb486c793`.
- CI #1843 on exact implementation/test HEAD: **GREEN**.
  - `go test ./...` — PASS
  - `go vet ./...` — PASS
  - PostgreSQL service-backed integration tests — PASS
  - `go test -race ./...` — PASS

### Safety Boundary

This milestone only locks runtime lifecycle completion ordering, error identity, repeated-close behavior, and database ownership sequencing. It does not change provider transaction semantics or recovery authority.

### Known Limitations

- the production `catalogWorkerLifecycle.Shutdown()` contract remains void-returning; catalog completion error injection remains an internal test seam;
- PostgreSQL integration coverage requires `DESKAPROVIDER_POSTGRES_DSN` and is skipped when unavailable;
- database close remains non-context-aware.

### Next Milestone

**#186 — Runtime Shutdown Partial-Completion Recovery Boundary**

Focus next on partial lifecycle completion where one shutdown boundary remains active, ensuring re-entry remains rejected, database ownership remains open, and a subsequent explicit completion can safely close ownership without replaying historical lifecycle errors.


## 189. Milestone Update — Runtime Shutdown Error Retention Across Repeated Close/Re-entry

**Date:** 2026-09-27

Completed:

- added focused regression coverage for recorded database cleanup errors across both direct `Service.Close()` and Run-owned shutdown;
- verified repeated `Service.Close()` returns the same recorded cleanup error object rather than reconstructing or replacing the stored cleanup result;
- verified repeated close remains single-shot for database ownership cleanup;
- verified Run-owned shutdown preserves the primary cancellation error together with the recorded cleanup error on the first `Run()` result;
- verified subsequent `Close()` returns only the recorded database cleanup result and does not replay the historical Run primary error;
- verified subsequent `Run()` after ownership closure returns `ErrServiceClosed` without replaying historical cleanup or lifecycle errors;
- preserved the existing runtime ownership mutex/closed-state semantics; no production runtime refactor was required;
- no provider retry/failover, transaction resubmission, ledger mutation, customer-balance mutation, treasury movement, provider funding, or synthetic transaction/audit reconstruction was introduced.

### Verification

- Regression test commit: `899a8adc34ca281be0ee962f938155c0625d1d7f`.
- CI #1847 on exact test HEAD: **GREEN**.
  - `go test ./...` — PASS
  - `go vet ./...` — PASS
  - PostgreSQL service-backed integration tests — PASS
  - `go test -race ./...` — PASS

### Safety Boundary

Recorded database cleanup errors remain infrastructure lifecycle evidence. Repeated `Close()` preserves the stored cleanup result, while closed-state `Run()` rejection remains a distinct runtime state and does not replay historical shutdown errors.

Transaction persistence remains authoritative for transaction state/idempotency; audit persistence remains observational evidence. No recovery or financial authority is derived from cleanup errors.

### Known Limitations

- database close remains non-context-aware;
- PostgreSQL integration coverage requires `DESKAPROVIDER_POSTGRES_DSN` and is skipped when unavailable;
- catalog completion errors continue through the existing internal test seam because the production catalog lifecycle shutdown contract remains void-returning;
- this milestone validates error-result retention and does not introduce a database reconnection or retry mechanism.

### Next Milestone

**#190 — Runtime Shutdown Error Retention Across Mixed Shared/Dedicated Ownership**

Focus next on the same retained-error contract across shared transaction/audit database handles and independently owned dedicated handles, including single-close behavior and closed-state re-entry, without introducing provider or transaction recovery behavior.


## 190. Milestone Update — Runtime Shutdown Error Retention Across Mixed Shared/Dedicated Ownership

**Date:** 2026-09-27

Completed:

- added a topology matrix covering shared transaction/audit ownership and independently owned dedicated transaction/audit handles;
- verified direct `Service.Close()` retains the recorded cleanup result across repeated close calls for both ownership topologies;
- verified Run-owned shutdown preserves the primary cancellation error together with the applicable transaction/audit cleanup errors;
- verified shared ownership closes the single shared database handle exactly once and does not replay a second independent audit cleanup error;
- verified dedicated ownership closes transaction and audit handles exactly once each and preserves both independent cleanup error identities;
- verified repeated `Service.Close()` returns the same recorded ownership cleanup result object after the first cleanup attempt;
- verified subsequent `Run()` after ownership closure returns `ErrServiceClosed` without replaying historical cleanup errors;
- no production runtime ownership refactor was required; the existing shared-handle guard and stored `closeErr` semantics are now covered by the mixed-topology regression matrix;
- no provider retry/failover, transaction resubmission, ledger mutation, customer-balance mutation, treasury movement, provider funding, or synthetic transaction/audit reconstruction was introduced.

### Verification

- Regression test/fix commits:
  - `7d1c8b5856fa1847ef08a7662afaa6d3d92b5e4e` — initial topology matrix;
  - `42c455404a1a76bfd693ccf591a62ac4a7c3fe18` — corrected shared-topology assertion.
- CI #1853 on exact corrected HEAD: **GREEN**.
  - `go test ./...` — PASS
  - `go vet ./...` — PASS
  - PostgreSQL service-backed integration tests — PASS
  - `go test -race ./...` — PASS

### Safety Boundary

This milestone is limited to runtime database ownership topology, cleanup-error retention, single-shot close semantics, and closed-state re-entry. Shared versus dedicated database cleanup does not create transaction/audit recovery authority.

Transaction persistence remains authoritative for transaction state/idempotency; audit persistence remains observational evidence. No provider retry/failover, transaction resubmission, ledger mutation, treasury movement, provider funding, or cross-domain recovery behavior is introduced.

### Known Limitations

- database close remains non-context-aware;
- PostgreSQL integration coverage requires `DESKAPROVIDER_POSTGRES_DSN` and is skipped when unavailable;
- production `catalogWorkerLifecycle.Shutdown()` remains void-returning; catalog completion error composition continues through the existing internal test seam;
- this milestone validates shared/dedicated ownership retention but does not add database reconnection or cleanup retry behavior.

### Next Milestone

**#191 — Runtime Shutdown Error Retention Across Mixed Lifecycle + Ownership Failures**

Focus next on combined lifecycle completion failures and shared/dedicated database cleanup failures, preserving independent error identity, single-shot ownership cleanup, and closed-state separation without introducing provider or transaction recovery behavior.


## 185. Milestone Update — Runtime Shutdown Completion Ordering & Error Precedence Review

**Date:** 2026-09-27

Completed:

- reviewed the final runtime shutdown ordering across balance-worker completion, catalog completion, and runtime database ownership closure;
- confirmed the production shutdown path composes errors in deterministic order: primary lifecycle error, balance-worker shutdown error, catalog shutdown completion error, transaction database cleanup error, then audit database cleanup error;
- confirmed database ownership closes only after the balance and catalog lifecycle boundaries report stopped, preventing database cleanup while an auxiliary lifecycle remains active;
- extended PostgreSQL-backed integration coverage to assert the composed shutdown error message preserves the same primary/lifecycle/database precedence ordering as the deterministic unit boundary;
- preserved single-shot database cleanup and repeated `Service.Close()` behavior;
- no production runtime behavior was changed by this milestone; the change is verification coverage only;
- no provider retry/failover, transaction resubmission, ledger mutation, customer-balance mutation, treasury movement, or synthetic transaction/audit reconstruction was introduced.

### Verification

- PostgreSQL shutdown-order regression test commit: `29cbeb7a619b9e2d92e36a92add77a348b903f4a`.
- CI #1857 on exact test HEAD: **GREEN**.
  - `go test ./...` — PASS
  - `go vet ./...` — PASS
  - PostgreSQL service-backed integration tests — PASS
  - `go test -race ./...` — PASS
- PostgreSQL integration now verifies the composed error ordering:
  1. primary cancellation;
  2. balance-worker rollback error;
  3. catalog shutdown completion error;
  4. transaction database cleanup error;
  5. audit database cleanup error.
- Cleanup order remains transaction database before audit database.
- Repeated `Service.Close()` does not replay lifecycle/primary errors and does not double-close runtime-owned database handles.

### Safety Boundary

This milestone only strengthens verification of runtime lifecycle shutdown ordering and error composition. Database cleanup remains a resource-ownership boundary, not a provider transaction result or financial authorization signal. No retry, failover, resubmission, ledger, treasury, funding, or cross-domain recovery authority is introduced.

### Known Limitations

- the production `catalogWorkerLifecycle.Shutdown()` contract remains void-returning; the completion error channel remains an internal test seam;
- PostgreSQL integration coverage requires `DESKAPROVIDER_POSTGRES_DSN` and is skipped when unavailable;
- database close remains non-context-aware;
- the milestone does not introduce a new production source of catalog shutdown errors.

### Next Milestone

**#186 — Runtime Shutdown Re-entry & Partial Lifecycle Convergence**

Focus next on re-entry after partial shutdown completion, including convergence of an auxiliary lifecycle that remains active after an injected shutdown-completion error, while preserving closed-state rejection, single-shot database ownership cleanup, and no replay of historical lifecycle errors.


## 185. Milestone Update — Runtime Shutdown Completion Ordering & Repeated Lifecycle Completion

**Date:** 2026-09-27

Completed:

- reviewed the runtime shutdown ordering after Milestone #184;
- verified the established shutdown sequence remains: balance lifecycle -> catalog lifecycle -> transaction database -> audit database;
- verified database ownership is closed only after the relevant lifecycles have stopped;
- verified repeated Service.Close() returns the recorded runtime database cleanup result without double-closing owned database handles;
- added regression coverage for repeated balance/catalog lifecycle completion through the existing internal completion seams;
- verified repeated completion leaves both lifecycle states stopped and preserves the injected completion-error identity;
- no production provider retry/failover, transaction resubmission, ledger mutation, treasury movement, funding, or synthetic transaction/audit recovery behavior was introduced.

### Verification

- Implementation/test HEAD: `ddee74d6154e2223fea840c18c9bf7a478564b08`.
- CI #1861 on exact implementation/test HEAD: **GREEN**.
  - `go test ./...` — PASS
  - `go vet ./...` — PASS
  - PostgreSQL service-backed integration tests — PASS
  - `go test -race ./...` — PASS
- Status documentation is updated only after the implementation/test HEAD passed the CI gate.

### Safety Boundary

This milestone is limited to shutdown completion ordering, repeated lifecycle completion behavior, and preservation of existing database ownership cleanup semantics. The transaction persistence boundary remains authoritative; audit remains observational; lifecycle completion does not authorize provider resubmission or financial mutation.

### Known Limitations

- the production catalog shutdown method remains void-returning; error composition coverage continues to use the internal catalogShutdown seam;
- database close remains non-context-aware;
- PostgreSQL integration tests require DESKAPROVIDER_POSTGRES_DSN and are skipped when unavailable;
- this milestone does not introduce new production retry/failover/recovery behavior.

### Next Milestone

**#186 — Runtime Shutdown Cancellation/Deadline Boundary**

Focus next on cancellation/deadline behavior during the shutdown phase itself, including preservation of the original lifecycle error composition and ownership cleanup boundary without allowing shutdown-time cancellation to create provider resubmission or financial-state mutation.


## 185. Milestone Update — Runtime Shutdown Completion Ordering & Error Precedence Review

**Date:** 2026-09-27

Completed:

- reviewed the final runtime shutdown ordering across auxiliary lifecycle completion and database ownership cleanup;
- confirmed the intended successful shutdown ordering is balance lifecycle completion, catalog lifecycle completion, transaction database cleanup, then audit database cleanup;
- added regression coverage for the partial-failure boundary where balance shutdown returns an error while the balance lifecycle remains active;
- verified that catalog completion is still attempted after the balance completion error, while database ownership cleanup is deferred because the balance lifecycle remains active;
- verified that `Service.Close()` refuses to close database ownership while an auxiliary lifecycle remains active;
- verified that cleanup resumes after lifecycle convergence and preserves transaction-before-audit database close ordering;
- preserved stable primary/lifecycle/database error identity and repeated `Service.Close()` behavior;
- no provider retry/failover, transaction resubmission, ledger mutation, treasury movement, provider funding, or synthetic transaction/audit reconstruction was introduced.

### Verification

- Regression test commit: `8b934925dbe4717f17d5d01fdc20c047afa8e788`.
- CI #1865 on exact test HEAD: **GREEN**.
  - `go test ./...` — PASS
  - `go vet ./...` — PASS
  - PostgreSQL service-backed integration tests — PASS
  - `go test -race ./...` — PASS

### Safety Boundary

This milestone validates shutdown ordering and ownership gating only. A database close is permitted only after the runtime confirms that the relevant auxiliary lifecycles have stopped. The change does not grant shutdown/recovery authority to provider adapters, transaction state, audit state, treasury, or customer balances.

### Known Limitations

- balance and catalog lifecycle completion error seams remain internal test seams;
- production catalog shutdown remains void-returning;
- PostgreSQL integration coverage requires `DESKAPROVIDER_POSTGRES_DSN` and is skipped when unavailable;
- database close remains non-context-aware.

### Next Milestone

**#186 — Runtime Shutdown Re-entry & Lifecycle Convergence Matrix**

Focus next on re-entry after partial lifecycle shutdown, convergence after an initially active lifecycle becomes stoppable, and ensuring no historical lifecycle error is replayed as a new Run result, without introducing new provider or transaction recovery behavior.

### CI #1867 Flake / Rerun

- Initial CI #1867 race attempt was **RED** in an existing PostgreSQL race test because a pre-existing fixture attempted to insert `amount=0` into `provider_transactions`, violating the database `amount_check` constraint.
- The `test` and `vet` jobs passed on the same exact HEAD.
- The failed race job was rerun without source changes and **PASS**ed.
- No #185 production/runtime code change was required for this CI failure.

### Final Gate

The exact documentation HEAD must have a subsequent CI run with both test and race jobs green before this milestone is considered closed.
