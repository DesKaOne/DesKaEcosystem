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
