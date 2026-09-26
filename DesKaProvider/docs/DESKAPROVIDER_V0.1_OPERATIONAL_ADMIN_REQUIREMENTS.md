# DesKaProvider v0.1 — Operational & Admin Requirements

**Branch:** `dev/deskaprovider-v0.1`  
**Date:** 2026-09-25  
**Status:** Architecture/requirements baseline — implementation follows incrementally

> Dokumen ini mencatat hasil keputusan arsitektur dan kebutuhan operasional DesKaProvider agar keputusan penting dari planning/meeting tidak hilang ketika implementasi berlanjut.

---

## 1. Tujuan

DesKaProvider adalah **internal financial infrastructure dan provider orchestration layer** milik DesKaEcosystem.

DesKaProvider bukan aplikasi provider external dan bukan database pusat seluruh user DesKaEcosystem.

Tanggung jawab utamanya:

- financial infrastructure / ledger untuk kebutuhan bookkeeping;
- external provider integration;
- provider routing;
- provider balance dan health;
- webhook gateway dan normalization;
- KYC engine/workflow;
- treasury dan provider funding operations;
- reconciliation;
- audit;
- internal service API;
- Admin Web / Control Panel.

DesKa* applications bergantung pada capability DesKaProvider, bukan langsung ke IndoChain atau external provider.

---

## 2. Boundary Utama

```text
                    ┌────────────────────┐
                    │     IndoChain      │
                    │ Blockchain / RPC   │
                    └─────────▲──────────┘
                              │
                              │ internal gateway
                              │
                    ┌─────────┴──────────┐
                    │   DesKaProvider    │
                    │                    │
                    │ Financial Core     │
                    │ Provider Router    │
                    │ Treasury           │
                    │ KYC Engine         │
                    │ Webhook Gateway    │
                    │ Reconciliation     │
                    │ Audit              │
                    │ Admin API          │
                    └─────────▲──────────┘
                              │
                 ┌────────────┼────────────┐
                 │            │            │
                 ▼            ▼            ▼
            DesKaCash   DesKaWallet   DesKaPay
```

DesKaCash tidak menyimpan atau mengetahui detail seperti:

- `INDOCHAIN_RPC_URL`;
- endpoint IAK;
- endpoint XP;
- endpoint RCB;
- signature provider;
- format webhook provider;
- credential provider;
- provider-specific status code.

Semua detail tersebut berhenti di DesKaProvider.

---

## 3. Admin Web adalah Komponen Inti

DesKaProvider **wajib memiliki Admin Web / Control Panel**, bukan hanya API.

Admin Web digunakan untuk operasi internal:

- monitoring;
- provider management;
- financial operations;
- treasury;
- KYC review;
- webhook monitoring;
- reconciliation;
- audit;
- configuration;
- operational controls.

Boundary yang diharapkan:

```text
Admin Web
    ↓
Admin API
    ↓
Application / Domain Services
    ↓
Financial / Provider / KYC / Treasury Core
    ↓
Database / External Provider
```

Admin Web **tidak boleh bypass domain/service layer dan langsung UPDATE database**.

---

## 4. Admin Dashboard

Dashboard minimum:

- total financial accounts;
- transaction volume;
- pending transactions;
- failed transactions;
- provider liquidity;
- provider health;
- webhook processing state;
- reconciliation warnings;
- KYC review queue;
- system health.

Dashboard boleh menggunakan read model/projection untuk performa, tetapi financial source of truth tetap mengikuti ledger.

---

## 5. External Provider Management

Admin harus dapat melihat dan mengelola provider external dari satu tempat.

Contoh provider:

- IAK;
- XP SINDONESIA;
- Midtrans;
- RCB;
- DOKU;
- Ezeelink;
- provider lain yang ditambahkan kemudian.

### 5.1 Provider Lifecycle

Minimum v0.1:

```text
ENABLED
DISABLED
```

Lifecycle dapat berkembang menjadi:

```text
REGISTERED
    ↓
CONFIGURED
    ↓
ENABLED
    ↓
DISABLED / MAINTENANCE / SUSPENDED
```

`enabled` dan `health_status` harus dipisahkan.

Contoh:

```text
IAK
enabled = true
health_status = UNHEALTHY
```

Provider dapat otomatis dikeluarkan dari routing jika health policy menyatakan provider tidak layak menerima transaksi.

### 5.2 Provider Capability

Enable/disable sebaiknya dapat dilakukan pada level capability, bukan hanya provider.

Contoh:

```text
IAK
├── PPOB             ENABLED
├── Payment          DISABLED
├── Payout           DISABLED
└── Balance Sync     ENABLED
```

Contoh:

```text
Midtrans
├── Payment / Collection ENABLED
├── QRIS                ENABLED
├── Bank Transfer       ENABLED
├── PPOB                DISABLED
└── Payout              DISABLED
```

Capability harus ditentukan berdasarkan API/capability provider yang benar-benar terverifikasi.

---

## 6. Provider Router

DesKaCash cukup meminta capability:

```text
"buat payment"
"beli pulsa"
"buat payout"
```

DesKaProvider menentukan provider yang digunakan.

Router dapat mempertimbangkan:

- capability;
- product availability;
- cached provider balance;
- provider health;
- priority;
- cost/price;
- latency;
- success history;
- routing/failover policy.

Provider-specific protocol tetap berada di adapter.

```text
Request
  ↓
Provider Router
  ↓
Capability + Health + Liquidity + Priority
  ↓
Provider Adapter
  ↓
External Provider
```

---

## 7. Provider Balance

DesKaProvider menyimpan **cached operational provider balance**.

Target:

- polling configurable;
- initial interval sekitar 30–60 detik;
- `last_checked_at`;
- health;
- error count;
- last successful synchronization;
- optional refresh setelah transaksi penting.

Cached balance bersifat advisory/operational snapshot dan **bukan pengganti hasil transaksi provider**.

Customer balance dan provider liquidity adalah konsep berbeda:

```text
DesKaCash customer funds
        ≠
DesKaProvider treasury
        ≠
External provider balance
```

---

## 8. Treasury

DesKaProvider membutuhkan domain **Treasury** untuk mengelola hubungan dana operasional dengan external providers.

Treasury bertanggung jawab atas:

- collection/settlement tracking;
- provider funding;
- provider deposit;
- treasury movements;
- funding status;
- reconciliation;
- operational liquidity;
- audit trail.

Contoh:

```text
Treasury
├── Collection
├── Settlement
├── Provider Funding
├── Provider Deposit
├── Reconciliation
└── Audit
```

---

## 9. Midtrans sebagai Collection / Funding Rail

Midtrans **tidak diposisikan sebagai provider PPOB**.

Peran utamanya dalam arsitektur awal dapat berupa:

- customer payment / collection;
- payment methods;
- settlement;
- funding source untuk treasury sesuai settlement flow yang tersedia.

Flow konseptual:

```text
Customer
   ↓
Payment / QRIS / VA / Collection
   ↓
Midtrans
   ↓
Settlement
   ↓
DesKaProvider Treasury
   ↓
Provider Funding
   ├── IAK
   ├── XP SINDONESIA
   ├── RCB
   └── other PPOB providers
```

Detail settlement dan withdrawal tetap harus mengikuti capability serta API/operational contract Midtrans yang terverifikasi.

---

## 10. Provider Deposit / Funding

Admin tidak seharusnya harus membuka website setiap provider satu per satu jika provider menyediakan API untuk deposit/funding.

DesKaProvider harus menyediakan satu operational flow:

```text
Admin Web
    ↓
Treasury / Provider Funding
    ↓
DesKaProvider
    ↓
Provider Deposit API
    ↓
External Provider
```

Contoh:

```text
Provider: IAK
Amount: Rp10.000.000
Source: Treasury
Type: PROVIDER_FUNDING
```

### 10.1 API Deposit

Jika provider menyediakan API deposit/funding yang terverifikasi:

- DesKaProvider memanggil provider adapter;
- adapter menerjemahkan request ke format provider;
- provider reference disimpan;
- status deposit dilacak;
- webhook/status provider diproses;
- reconciliation dilakukan.

DesKaProvider **tidak boleh mengasumsikan HTTP 200 berarti dana sudah masuk**.

Target lifecycle:

```text
CREATED
   ↓
SUBMITTED
   ↓
PENDING
   ├── SUCCESS
   ├── FAILED
   └── EXPIRED
```

### 10.2 Manual Deposit

Jika provider tidak menyediakan API deposit, sistem tetap harus mendukung pencatatan manual secara terkontrol.

Contoh:

```text
Manual Provider Funding
Provider: XP
Amount: Rp10.000.000
Reference: ...
Proof: ...
Status: PENDING
```

Manual funding tetap masuk audit dan reconciliation.

### 10.3 Jangan Auto-Move Uang Tanpa Authorization Flow

Liquidity monitoring boleh menghasilkan rekomendasi:

```text
Provider balance
+
Expected outflow
+
Safety buffer
        ↓
Liquidity gap
        ↓
Funding recommendation
```

Untuk v0.1, sistem **tidak boleh diam-diam memindahkan dana** hanya karena balance rendah.

Target awal:

```text
Detect shortage
    ↓
Funding recommendation
    ↓
Admin approval
    ↓
Funding execution
    ↓
Provider confirmation
    ↓
Reconciliation
```

Threshold approval dapat dibuat kemudian sesuai risk policy.

---

## 11. Treasury Movement

Pergerakan dana operational harus memiliki record eksplisit.

Contoh field konseptual:

- movement ID;
- source;
- destination;
- amount;
- currency;
- type;
- status;
- internal reference;
- provider reference;
- created_at;
- completed_at;
- audit metadata.

Contoh:

```text
Source:      MIDTRANS_SETTLEMENT
Destination: IAK
Amount:      Rp10.000.000
Type:        PROVIDER_FUNDING
Status:      PENDING
```

Hal ini mencegah treasury flow bercampur dengan customer wallet ledger.

---

## 12. RCB

RCB dapat menjadi provider yang memiliki capability lebih luas dibanding provider PPOB murni, **jika capability tersebut telah diverifikasi dan aktif**.

Potensi capability yang perlu dimodelkan secara capability-based:

- QRIS;
- Virtual Account;
- e-wallet payment;
- payout;
- PPOB/top-up;
- webhook;
- API gateway.

Status RCB saat baseline ini tetap mengikuti status verifikasi aktual. Jangan menganggap capability production aktif hanya karena tersedia pada website, sandbox, atau dokumentasi publik.

Jika RCB sudah verified/active untuk capability yang dibutuhkan, routing dapat dikonfigurasi misalnya:

```text
Payment
├── Midtrans ENABLED
└── RCB      ENABLED

PPOB
├── IAK      ENABLED
├── XP       ENABLED
└── RCB      ENABLED
```

Ini tetap harus dikontrol melalui capability registry dan verification state.

---

## 13. KYC Admin

DesKaProvider memiliki KYC engine/workflow internal.

DesKa* memberikan user-facing KYC UI, sedangkan workflow verification berada di DesKaProvider.

Admin Web harus menyediakan:

- KYC review queue;
- identity/document review;
- approve;
- reject;
- request revision;
- status history;
- reviewer identity;
- reason;
- audit trail.

Status internal:

```text
NOT_STARTED
PENDING
UNDER_REVIEW
VERIFIED
REJECTED
SUSPENDED
```

Provider onboarding/KYC status harus dipisahkan dari internal DesKa KYC.

---

## 14. Webhook Gateway

External provider webhook masuk ke DesKaProvider.

```text
External Provider
       ↓
DesKaProvider Webhook Gateway
       ↓
Signature Verification
       ↓
Timestamp / Replay Check
       ↓
Idempotency
       ↓
Persist Event
       ↓
Normalize
       ↓
Financial / Domain Processing
       ↓
Internal Domain Event
       ↓
DesKa*
```

DesKaCash tidak boleh mengetahui endpoint provider-specific seperti:

- `/api/webhooks/iak`;
- `/api/webhooks/midtrans`;
- `/api/webhooks/xp`.

Provider-specific webhook format harus berhenti di adapter/gateway DesKaProvider.

---

## 15. Financial Ledger Boundary

DesKaProvider adalah financial infrastructure, tetapi bukan central user database.

Setiap aplikasi tetap memiliki user/account/profile sendiri:

```text
DesKaCash
├── users
├── auth
├── profile
└── application data
```

DesKaProvider menyimpan data finansial yang diperlukan untuk bookkeeping:

- financial accounts;
- immutable postings;
- transactions;
- reservations;
- settlement;
- reconciliation;
- financial audit;
- provider integration records;
- application/scope/account mapping.

Konsep financial identity:

```text
(scope, owner_ref, currency)
```

Balance projection pada DesKa* boleh digunakan untuk read/UI performance, tetapi source of truth financial tetap berada pada ledger DesKaProvider.

Tidak boleh ada dua ledger yang sama-sama dapat diedit secara independen.

---

## 16. Admin Financial Safety

Admin Web harus memperlakukan ledger sebagai financial system.

Tidak boleh ada operasi:

```text
UPDATE balance = ...
```

sebagai cara normal melakukan koreksi.

Koreksi harus melalui transaction baru, reversal, atau adjustment yang memiliki:

- authorization;
- reason;
- actor;
- timestamp;
- audit trail;
- immutable postings.

Webhook replay dan provider funding juga harus melalui service/domain layer dan dicatat dalam audit.

---

## 17. Admin Audit Log

Perubahan operational harus tercatat.

Contoh:

```text
ADMIN-001
Provider: IAK
Action: DISABLE
Previous: ENABLED
New: DISABLED
Reason: Provider maintenance
Timestamp: ...
```

Audit minimal mencakup:

- login/logout;
- provider enable/disable;
- capability changes;
- credential rotation;
- routing changes;
- funding request;
- funding approval;
- KYC decision;
- webhook replay;
- reconciliation action;
- account freeze/unfreeze;
- financial adjustment.

---

## 18. Security Requirements

Provider credential:

- tidak boleh commit ke Git;
- tidak boleh ditampilkan plaintext di Admin Web;
- disimpan melalui environment/secret manager/encrypted storage;
- rotation/revocation harus tersedia sesuai capability.

DesKaProvider sebaiknya tidak langsung public jika tidak diperlukan.

Target network:

```text
Internet
  ↓
WAF / Edge / DDoS protection
  ↓
DesKaCash public API
  ↓
Private service boundary
  ↓
DesKaProvider
  ↓
PostgreSQL / IndoChain / External Providers
```

Admin interface harus memiliki authentication dan authorization terpisah/terkontrol.

---

## 19. Initial Provider Role Model

Arsitektur awal yang dibahas:

| Provider | Peran awal |
|---|---|
| Midtrans | Payment / collection / settlement rail |
| IAK | PPOB / product fulfillment |
| XP SINDONESIA | PPOB / product fulfillment |
| RCB | Multi-capability provider jika verified/active |
| DOKU | Candidate / deferred sampai readiness terpenuhi |
| Ezeelink | Candidate / deferred sampai readiness terpenuhi |
| provider lain | Capability-based |

Provider status dan capability harus tetap diverifikasi sebelum production enablement.

---

## 20. Open API

Open API **bukan prioritas v0.1**.

Urutan evolusi:

```text
v0.1
Internal API
    ↓
Multi-provider / routing / health / liquidity
    ↓
Stable public API v1
    ↓
Developer Portal
    ↓
Sandbox
    ↓
API Keys / OAuth
    ↓
External Developers
```

Jangan membebani v0.1 dengan public developer platform sebelum internal infrastructure stabil.

---

## 21. Target Admin Web Navigation

Target awal:

```text
Admin Web
├── Dashboard
├── Financial
│   ├── Accounts
│   ├── Transactions
│   ├── Postings
│   └── Reservations
├── Providers
│   ├── Provider List
│   ├── Enable / Disable
│   ├── Capabilities
│   ├── Health
│   ├── Balance
│   └── Routing
├── Treasury
│   ├── Overview
│   ├── Settlements
│   ├── Provider Funding
│   ├── Manual Deposit
│   └── Reconciliation
├── Webhooks
│   ├── Events
│   ├── Failed
│   └── Replay
├── KYC
│   ├── Queue
│   ├── Review
│   └── Audit
├── Applications
│   ├── Client IDs
│   ├── Scopes
│   └── Credentials
├── Audit
└── Settings
```

Menu dapat diimplementasikan bertahap; daftar ini adalah target architecture, bukan kewajiban seluruhnya selesai di milestone pertama.

---

## 22. Prinsip yang Tidak Boleh Dilupakan

1. **DesKaCash tidak berbicara langsung dengan external provider.**
2. **DesKaCash tidak berbicara langsung dengan IndoChain.**
3. **DesKaProvider adalah internal infrastructure, bukan user-facing provider.**
4. **Admin Web adalah bagian resmi DesKaProvider.**
5. **Provider dapat enable/disable dari Admin Web.**
6. **Capability provider dapat dikontrol secara terpisah dari lifecycle provider.**
7. **Provider health berbeda dari enabled/disabled state.**
8. **Provider balance berbeda dari customer balance.**
9. **Treasury berbeda dari customer wallet.**
10. **Midtrans bukan provider PPOB dalam arsitektur awal.**
11. **Midtrans dapat menjadi collection/settlement rail untuk funding treasury.**
12. **Provider funding harus memiliki workflow dan audit.**
13. **Jika provider menyediakan deposit API, DesKaProvider menjadi satu pintu operasional agar admin tidak perlu membuka dashboard provider satu per satu.**
14. **Jika provider tidak menyediakan deposit API, gunakan manual funding workflow yang tetap tercatat dan direconcile.**
15. **HTTP 200/provider acceptance bukan otomatis berarti financial success.**
16. **Provider-specific protocol tidak boleh bocor ke DesKaCash.**
17. **Webhook harus idempotent dan dapat diaudit.**
18. **Ledger tidak dikoreksi dengan UPDATE saldo.**
19. **Credential tidak pernah masuk source control atau plaintext ke browser.**
20. **Capability production hanya boleh di-enable setelah provider contract/activation benar-benar terverifikasi.**
21. **Open API public dikerjakan setelah internal infrastructure stabil.**

---

## 23. Implementation Priority

Untuk solo development, kebutuhan ini dikerjakan bertahap:

```text
Phase 1
Provider Registry
Provider Config
Provider Health
Provider Balance
        ↓
Phase 2
Provider Router
Webhook Gateway
Idempotency
        ↓
Phase 3
Admin Auth
Admin Dashboard
Provider Management
        ↓
Phase 4
Financial / Ledger Viewer
Treasury
Provider Funding
Reconciliation
        ↓
Phase 5
KYC Admin
Audit
Advanced Routing
        ↓
Phase 6
Stable Internal API
        ↓
Future
Public Open API
```

Jangan mengimplementasikan seluruh Admin Web sekaligus sebelum domain/service layer stabil.

---

## 24. Final Architecture Intent

DesKaProvider diharapkan menjadi **control center internal** untuk financial operations dan external provider orchestration:

```text
                         DESKAPROVIDER
                              │
        ┌─────────────────────┼─────────────────────┐
        │                     │                     │
        ▼                     ▼                     ▼
 Financial Core          Provider Layer          Admin Web
        │                     │                     │
        │              ┌──────┼──────┐              │
        │              ▼      ▼      ▼              │
        │             IAK     XP     RCB            │
        │                                            │
        └────────────── Treasury ───────────────────┘
                              │
                              ▼
                       Midtrans Settlement
                              │
                              ▼
                       Provider Funding
```

Tujuan akhirnya bukan membuat DesKaCash mengetahui semua detail sistem finansial, tetapi membuat **DesKaProvider menjadi satu boundary yang konsisten, dapat diaudit, dapat dikontrol admin, dan dapat berkembang menjadi infrastructure layer untuk seluruh DesKaEcosystem**.
