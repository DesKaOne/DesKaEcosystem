# DesKaCash v0.1 — Development Status / Handover

Status: **Development / Core backend prototype — explicit IDR/dIDR asset boundary introduced**  
Branch: `dev/deskacash-v0.1`  
Repository: `DesKaOne/DesKaEcosystem`  
Last reviewed: 2026-09-23

> Dokumen ini adalah titik handover untuk melanjutkan development DesKaCash jika percakapan/chat sebelumnya sudah penuh.

## 1. Executive Summary

Per 2026-09-23, DesKaCash sudah melewati tahap skeleton dan memiliki core backend Go untuk prototype financial transaction service.

Yang sudah nyata di branch:
- Go backend module.
- HTTP server dan `/health`.
- Domain ledger/account/transaction/money.
- In-memory ledger repository dan tests.
- PostgreSQL repository dan migration.
- Atomic credit/debit dengan database transaction dan row locking.
- Payment abstraction/provider boundary.
- Payment creation + idempotency key handling.
- Webhook event model dan duplicate-event protection.
- Payment reconciliation/state transition logic.
- Provider amount/status validation.
- PostgreSQL integration tests untuk ledger/debit path.
- Reconciliation tests yang menggunakan memory ledger nyata untuk reversal/refund.
- Immutable posting domain model awal dengan validasi debit/credit.\n- Explicit ledger asset model untuk membedakan `IDR` fiat dan `dIDR` native IndoChain.\n- `Money` sekarang memakai integer base units yang maknanya ditentukan oleh asset: IDR memakai rupiah sebagai base unit, sedangkan dIDR memakai 0.001 dIDR.\n- Default account/transaction asset DesKaCash v0.1 sekarang `IDR`; `dIDR` tetap tersedia sebagai asset terpisah untuk integrasi IndoChain.\n- Database asset constraints sekarang menerima `IDR` dan `dIDR`, sehingga boundary asset tidak lagi dipaksa menjadi dIDR.\n- Immutable posting persistence contract (`PostingStore`).\n- Memory repository posting storage + duplicate protection.\n- PostgreSQL `ledger_postings` table + create/list persistence.\n- Memory/PostgreSQL tests untuk immutable posting persistence.
- GitHub Actions CI dengan PostgreSQL service, `go test ./...`, dan `go vet ./...`.
- Feature-scope document untuk arah v0.1.
- Arah integrasi IndoChain/dIDR sudah terdokumentasi.

Belum production-ready sebagai e-wallet. Belum tersedia implementasi lengkap untuk authentication/user service, wallet business API, real Midtrans adapter, PPOB adapter, payout/withdrawal, persistent webhook store PostgreSQL, complete double-entry ledger, reservation/hold, real IndoChain RPC integration, Flutter frontend, dan production deployment.

## 2. Current Development Classification

Posisi saat ini:

Architecture / Scope → Domain Prototype → Ledger Prototype → Payment/Reconciliation Core → **Ledger Hardening** → Real Provider Adapter → Wallet API → PPOB / External Money Movement → IndoChain Integration → Production Hardening

DesKaCash sekarang paling tepat disebut **Core financial backend prototype — ledger + payment lifecycle foundation**.

## 3. Repository Structure

DesKaCash/
  README.md
  backend/
    go.mod
    go.sum
    cmd/deskacash/main.go
    internal/config/
    internal/httpapi/
    internal/ledger/
      posting.go
      posting_test.go
    internal/payment/
    internal/storage/postgres/
    migrations/001_init_ledger.sql
  docs/
    DESKACASH_V0.1_FEATURE_SCOPE.md
    DESKACASH_V0.1_DEVELOPMENT_STATUS.md

## 4. Ledger Core

Package `internal/ledger` memiliki Account, Money, Entry, Transaction, Posting, Repository, Service, memory repository, dan tests.

Account memiliki account ID, user ID, asset, balance dalam base units, dan version.

Current code sekarang menggunakan `AssetIDR = "IDR"` sebagai default application ledger v0.1, sementara `AssetDIDR = "dIDR"` tetap didefinisikan sebagai asset terpisah.

Account sengaja dipisahkan dari alamat IndoChain.

### Immutable Posting Model — current progress

Model `Posting` sudah diperkenalkan sebagai langkah awal menuju immutable double-entry ledger.

Posting memiliki:
- posting ID;
- transaction ID;
- account ID;
- asset;
- debit/credit type;
- positive amount;
- reference.

Validasi posting menolak identifier kosong, asset kosong, amount non-positive, dan posting type yang tidak dikenal.

Model ini sudah memiliki persistence layer terpisah melalui `PostingStore`, memory repository, dan PostgreSQL `ledger_postings`. Namun model ini **belum menjadi source of truth repository** dan belum menggantikan `ledger_entries`/account balance.

### IDR vs dIDR boundary — current progress

Boundary asset sekarang sudah eksplisit di domain dan schema. `IDR` adalah default asset untuk wallet ledger v0.1, sedangkan `dIDR` tetap merupakan asset native IndoChain dan tidak otomatis menjadi saldo fiat DesKaCash. `Money` tetap memakai integer base units, tetapi interpretasi unit mengikuti asset yang melekat pada account/transaction/posting.

Perubahan ini baru memperjelas denomination boundary; conversion/settlement IDR ↔ dIDR, address mapping, dan on-chain verification belum diimplementasikan.

### Important architecture gap

Feature scope menetapkan PostgreSQL double-entry ledger sebagai source of truth. Implementation saat ini masih menggunakan model account balance + ledger_entries + transactions, dan operasi credit/debit masih meng-update balance account secara langsung di dalam database transaction.

Jadi **double-entry immutable ledger penuh belum selesai di code**.

Target berikutnya:
- debit/credit pair;
- balanced transaction validation;
- posting persistence;
- account normal balance;
- reversal sebagai transaction baru;
- balance projection dari postings;
- concurrency/invariant tests.

## 5. PostgreSQL

Migration `001_init_ledger.sql` membuat:
- `accounts`;
- `transactions`;
- `ledger_entries`.

Constraint saat ini:
- asset harus `dIDR`;
- amount harus positif;
- balance tidak boleh negatif;
- transaction status dibatasi;
- entry type hanya credit/debit;
- satu transaction memiliki satu ledger entry melalui unique constraint saat ini.

Repository PostgreSQL sudah memiliki create/get account, save account, create/get transaction, create ledger entry, list ledger entries, dan duplicate error mapping.

Immutable `Posting` sekarang sudah memiliki migration dan repository persistence. Repository hanya menyediakan create/read; tidak ada update/delete API untuk posting.

## 6. Atomic Credit / Debit

`internal/storage/postgres/atomic.go` melakukan:
1. begin DB transaction;
2. select account dengan `FOR UPDATE`;
3. validasi amount;
4. validasi insufficient funds untuk debit;
5. insert transaction;
6. update account balance;
7. insert ledger entry;
8. commit.

Integration tests PostgreSQL untuk path debit/ledger juga sudah tersedia.

Reconciliation tests sekarang juga memverifikasi reversal/refund terhadap memory ledger nyata, termasuk actual balance mutation dan insufficient-funds behavior.

## 7. Payment Core

Package `internal/payment` sudah memiliki Payment, status, Provider interface, ProviderPayment, payment creation service, memory payment store, provider create service, webhook event, webhook store, reconciliation service, dan tests.

Payment memiliki internal payment ID, account ID, provider, provider ID, idempotency key, amount, status, reference, dan timestamps.

## 8. Payment State Handling

Status yang tersedia:
- pending
- succeeded
- failed
- expired
- reversed
- refunded

Transition yang dimodelkan:
- pending → succeeded / failed / expired
- succeeded → reversed / refunded
- status yang sama diperbolehkan.

Webhook status divalidasi agar status yang tidak dikenal tidak langsung mempengaruhi payment.

## 9. Webhook Idempotency / Validation

Reconciliation sudah melakukan validasi event ID, provider, provider transaction ID, status, payment/provider matching, amount matching, valid state transition, dan duplicate webhook detection.

Duplicate event yang konsisten diperlakukan sebagai idempotent. Event dengan metadata penting berbeda ditolak.

Untuk reversal/refund, ledger effect menggunakan transaction ID turunan yang deterministik sehingga retry tidak menggandakan financial effect.

## 10. Provider Boundary

Interface provider saat ini memiliki Name, CreatePayment, dan GetPayment.

Provider-specific implementation dipisahkan dari application/payment domain.

Feature scope menempatkan Midtrans sebagai payment rail utama untuk collection/top-up v0.1. Provider lain dapat ditambahkan melalui adapter.

**Real provider adapter belum terlihat pada branch saat review ini.**

## 11. Provider Create Flow

Alur saat ini:
check idempotency key → create local Payment → create provider payment → validate provider amount → save provider ID/reference/status.

Provider amount mismatch ditolak.

## 12. Reconciliation Flow

Alur saat ini:
Provider → Webhook → Validate event → Find payment → Validate provider ID → Validate amount → Validate status transition → Apply ledger effect → Update payment status → Record webhook.

Untuk `succeeded`, reconciliation melakukan credit ledger.

Untuk `reversed/refunded`, reconciliation membuat transaction baru untuk debit.

Current tests juga mencakup:
- reversal terhadap memory ledger nyata;
- refund terhadap memory ledger nyata;
- insufficient funds tidak mengubah status payment dan tidak membuat ledger entry.

## 13. HTTP/API Status

HTTP server sudah tersedia, tetapi endpoint bisnis belum.

Endpoint yang terlihat saat review:
- `GET /health`

Belum ada endpoint nyata untuk create wallet, balance, transaction history, P2P, top-up, provider webhook, PPOB, atau payout.

Jadi service dapat dijalankan sebagai service dasar, tetapi API bisnis DesKaCash masih tahap berikutnya.

## 14. IndoChain / dIDR Direction

Feature scope sudah menetapkan boundary DesKaCash ↔ IndoChain:
- DesKaCash menangani application/payment/fiat side;
- IndoChain menangani blockchain state;
- dIDR adalah native asset IndoChain;
- DesKaCash tidak menjadi source of truth blockchain state;
- IDR fiat dan dIDR on-chain harus dibedakan.

Target blockchain integration mencakup address mapping, native RPC client, read balance, transaction submission/tracking, finality verification, dan IDR ↔ dIDR conversion.

**Namun current code masih menggunakan `dIDR` sebagai asset pada PostgreSQL application ledger.**

Ini perlu diperjelas pada fase berikutnya karena feature scope mendefinisikan v0.1 sebagai fiat IDR wallet dan memisahkan IDR dari dIDR.

## 15. PPOB

Feature scope sudah mendefinisikan PPOBProvider dengan GetProducts, Inquiry, Purchase, GetStatus, dan HandleWebhook.

Target adapter mencakup DigiflazzAdapter, future provider adapter, dan mock provider.

Target flow: product → validate → check balance → reserve → provider transaction → success → commit ledger. Failure harus release reservation. Pending tetap menunggu confirmation.

**PPOB package/adapter belum terlihat pada current backend tree.**

## 16. Current Provider Plan

Midtrans ditargetkan untuk collection/top-up v0.1: bank transfer/VA, dynamic QRIS, dan supported e-wallet payment methods.

Feature scope secara eksplisit tidak mengasumsikan Midtrans sebagai provider untuk arbitrary bank payout, arbitrary e-wallet payout, permanent unique VA per user, atau merchant QRIS scan sebagai wallet payment.

Future provider boundary mencakup unique VA, bank payout, e-wallet payout, withdrawal, dan merchant payment.

## 17. CI

Workflow `.github/workflows/deskaone.yml` mendefinisikan PostgreSQL 16 service, Go 1.24, `go mod tidy`, `go test ./...`, dan `go vet ./...`.

CI berjalan pada `main`, `dev/**`, dan pull request yang menyentuh DesKaCash/workflow.

Setelah perubahan posting model, CI harus dicek berdasarkan SHA branch terbaru sebelum melanjutkan perubahan berikutnya.

## 18. Branch Position

Branch aktif: `dev/deskacash-v0.1`.

Snapshot commit sebelum update status ini: `fd277643a7240bb7fb222664707a7735e0979645e`. Setelah commit status, SHA branch akan berubah.

Angka comparison terhadap `main` dapat berubah setelah commit baru.

## 19. Important Architecture Gaps

### A. Double-entry ledger belum final
Current implementation masih memiliki `account.balance_base_units` dan satu ledger entry per transaction. Target membutuhkan immutable postings, debit/credit pair, balanced transaction, account normal balance, reversal transaction, dan balance projection.

### B. IDR vs dIDR boundary — domain/schema progress
Feature scope menetapkan IDR fiat sebagai wallet v0.1 dan dIDR sebagai native asset IndoChain. Domain dan migration sekarang sudah membedakan kedua asset tersebut, dengan IDR sebagai default wallet asset. Conversion/settlement dan on-chain mapping masih belum tersedia.

### C. Transaction state machine belum lengkap
Payment reconciliation memiliki transition rules, tetapi ledger transaction belum menjadi state machine lengkap dengan seluruh invariant target.

### D. Reservation / hold belum ada
PPOB membutuhkan available balance → reserve → provider processing → commit atau release.

### E. Persistent webhook store belum lengkap
Webhook store yang terlihat masih memory implementation. Production membutuhkan persistent idempotent storage.

### F. Real provider adapter belum ada
Provider interface sudah ada, tetapi adapter production Midtrans/PPOB belum terlihat.

### G. User/auth layer belum ada
Belum terlihat registration, login, authentication, authorization, KYC/KYB boundary, session/token management.

### H. HTTP API belum menjadi business API
Saat ini baru `/health`.

### I. IndoChain integration belum diimplementasikan
Dokumentasi architecture sudah ada, tetapi RPC client, transaction submission, finality verification, dan dIDR conversion belum menjadi service nyata pada DesKaCash.

## 20. Recommended Development Order

### Phase 1 — Ledger Hardening
1. Finalize account model.
2. Pisahkan IDR vs dIDR secara eksplisit. **Domain dan schema boundary sudah diperkenalkan; conversion/settlement belum.**
3. Implement immutable postings. **Domain model dan persistence dasar sudah ada; balancing dan transactional posting belum.**
4. Implement debit/credit balancing.
5. Add transaction status machine.
6. Add reversal transaction relation.
7. Add balance projection.
8. Add concurrency tests.
9. Add PostgreSQL invariant tests.

### Phase 2 — Wallet Domain
1. User/account boundary.
2. Create wallet.
3. Get balance.
4. Transaction history.
5. P2P transfer.
6. Idempotency.
7. Atomic debit/credit.

### Phase 3 — Payment Collection
1. Payment provider adapter.
2. Midtrans sandbox adapter.
3. Top-up create.
4. Provider webhook endpoint.
5. Persistent webhook event.
6. Reconciliation.
7. Provider polling.
8. Integration tests.

### Phase 4 — PPOB
1. PPOBProvider interface.
2. Product catalog.
3. Inquiry.
4. Reservation.
5. Purchase.
6. Provider status.
7. Webhook.
8. Settlement.
9. Margin/revenue ledger.

### Phase 5 — External Money Movement
1. Payout provider abstraction.
2. Bank payout.
3. E-wallet payout.
4. Withdrawal.
5. Reconciliation.

### Phase 6 — IndoChain
1. Address mapping.
2. Native RPC client.
3. Read dIDR balance.
4. Submit transaction.
5. Track transaction.
6. Verify finality.
7. IDR ↔ dIDR conversion.
8. EVM interaction.
9. Native fee sponsorship integration.

### Phase 7 — Production Hardening
1. Authentication.
2. Authorization.
3. Rate limiting.
4. Audit logging.
5. Secrets management.
6. Observability.
7. Fraud/risk controls.
8. Security testing.
9. Disaster recovery.
10. Reconciliation operations.
11. Deployment.
12. Production runbook.

## 21. Architecture Guardrails

1. Ledger adalah source of truth untuk saldo application/fiat yang memang dikelola DesKaCash.
2. Provider bukan source of truth saldo DesKaCash.
3. Webhook harus idempotent.
4. Duplicate webhook tidak boleh menggandakan financial effect.
5. Provider transaction ID dan DesKaCash transaction ID harus dibedakan.
6. Reversal menggunakan transaction baru.
7. Financial postings tidak boleh diedit sembarangan.
8. Provider-specific logic berada di adapter.
9. Business service tetap provider-agnostic.
10. IDR fiat dan dIDR on-chain tidak boleh dicampur secara konseptual.
11. Blockchain state tidak boleh dibuat seolah-olah berasal dari PostgreSQL DesKaCash.
12. Feature yang belum tersedia harus tetap ditandai `COMING_SOON`.
13. Jangan mengklaim production readiness hanya karena unit/integration tests sudah ada.

## 22. Handover Starting Point

Jika chat sebelumnya penuh, chat baru cukup diberi instruksi:

> Baca `DesKaCash/docs/DESKACASH_V0.1_DEVELOPMENT_STATUS.md` pada branch `dev/deskacash-v0.1`, lalu lanjutkan development DesKaCash dari status terakhir. Sebelum coding, cek kondisi branch dan file yang disebut dalam handover.

Prioritas saat ini:

**Ledger Hardening → IDR vs dIDR boundary → immutable posting persistence → debit/credit balancing → transaction state machine → Wallet API → Provider integration**

## 23. Related Documentation

- `DesKaCash/docs/DESKACASH_V0.1_FEATURE_SCOPE.md`
- `DesKaCash/README.md`
- IndoChain documentation under `IndoChain/docs/`

---

**Latest progression:** CI pada `e538eb7f7b1e99c16e39a212bc7bcb115b76628e` sudah hijau. Setelah itu boundary `IDR` vs `dIDR` diperjelas pada domain `Asset`, `Money`, default account/transaction, dan PostgreSQL schema; immutable postings masih belum menjadi financial source of truth dan belum dipaksa balanced.\n\n**Handover principle:** Jangan menebak status dari chat lama. Gunakan branch `dev/deskacash-v0.1` sebagai kondisi aktual dan dokumen ini sebagai peta handover. Jika code dan dokumen berbeda, verifikasi code terlebih dahulu lalu update dokumentasi.


### CI follow-up — 2026-09-23

CI pada commit `f5187511163d5408e7b9b1b0b4202b554b20b8f2` gagal di step `go test ./...`. Root cause ditemukan sebagai syntax error pada `money_test.go`: fungsi `TestFromIDR` belum ditutup sebelum deklarasi `TestFromDIDR`.

Fix committed sebagai `a5629b2d4fc34968aca407e129b64d2ec2f7706b` dengan perubahan hanya pada penutupan fungsi test tersebut. PostgreSQL integration tests sendiri melewati step test; kegagalan bukan berasal dari schema atau repository integration.


### CI follow-up — 2026-09-23 (second pass)

Run CI setelah commit dokumentasi `8fca06ceb7dd08cde8c5f3c74b1cd08f094d7200` masih gagal karena fix sebelumnya hanya menutup blok `if`, tetapi fungsi `TestFromIDR` sendiri belum ditutup. Log CI mengonfirmasi error parser yang sama pada `money_test.go:16`.

Fix final dibuat sebagai `f8314cfd95d4322c42d149d614d60d547a033b04` dengan menutup blok `if` dan fungsi `TestFromIDR` secara lengkap. Tidak ada perubahan pada production code.
