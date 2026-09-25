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
