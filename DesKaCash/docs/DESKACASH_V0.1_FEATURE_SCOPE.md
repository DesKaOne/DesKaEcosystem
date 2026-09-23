# DesKaCash v0.1 — Feature Scope & Provider Capability

Status: Draft / Development  
Branch: `dev/deskacash-v0.1`

## 1. Tujuan v0.1

DesKaCash v0.1 dibangun sebagai wallet ledger yang dapat menerima dana, menyimpan saldo internal, memindahkan saldo antar-user, dan menggunakan saldo tersebut untuk kebutuhan digital/PPOB.

Provider eksternal diposisikan sebagai **payment rail / service provider**, bukan sebagai source of truth saldo user.

Source of truth saldo:
- PostgreSQL double-entry ledger DesKaCash.
- Posting finansial immutable.
- Semua perubahan saldo harus melalui transaction ledger yang balanced.
- Provider transaction tidak boleh langsung mengubah saldo tanpa proses verifikasi, idempotency, dan settlement.

## 2. Fitur Wallet

### Available / target v0.1

- User wallet/account.
- Saldo IDR.
- Transaction history.
- P2P transfer antar-user DesKaCash.
- Top up melalui payment provider.
- Pembayaran produk digital/PPOB menggunakan saldo internal.
- Idempotency dan webhook processing.
- Provider transaction mapping.
- Reconciliation/status polling untuk transaksi provider yang pending.

### Belum tersedia / Coming Soon

- VA permanen/unik untuk setiap user.
- Withdrawal ke rekening bank user.
- Transfer ke e-wallet eksternal.
- Cash withdrawal.
- Scan QRIS merchant untuk pembayaran keluar.
- Merchant settlement.

## 3. Top Up v0.1

Top up tidak harus menunggu VA unik per user.

### Bank Transfer / VA

Target:

```
User
  -> DesKaCash create topup
  -> Payment Provider
  -> Bank Transfer / VA
  -> Provider webhook
  -> Verify notification
  -> Idempotency check
  -> Ledger SUCCESS
  -> User balance +
```

VA unik/per-user masih dianggap **future capability** sampai provider yang sesuai tersedia.

### Dynamic QRIS

Dynamic QRIS dapat digunakan sebagai salah satu metode top up.

Flow:

```
User
  -> Top Up Rp100.000
  -> Create provider payment
  -> Dynamic QRIS
  -> User bayar dengan aplikasi yang mendukung QRIS
  -> Provider webhook
  -> Verify + idempotency
  -> Ledger SUCCESS
  -> Balance +
```

Catatan: kemampuan QRIS ini adalah **collection/top up**, bukan berarti DesKaCash sudah dapat scan QRIS merchant untuk pembayaran keluar.

### E-wallet sebagai metode top up

Jika payment provider mendukung e-wallet sebagai payment method, user dapat memilih e-wallet tersebut saat membuat top up.

Contoh capability yang perlu diuji melalui Midtrans:
- GoPay
- ShopeePay
- OVO
- DANA
- metode lain yang tersedia pada akun/konfigurasi provider.

Catatan penting: kemampuan menerima pembayaran menggunakan e-wallet **tidak sama** dengan kemampuan melakukan payout dari saldo DesKaCash ke e-wallet user.

## 4. P2P DesKaCash

Transfer internal tidak membutuhkan provider eksternal.

Contoh:

```
User A
  -100.000
       |
       +---- P2P Transfer ----+
                              |
User B                       +100.000
```

Implementasi harus menggunakan ledger transaction yang atomic dan balanced.

Provider tidak dilibatkan untuk transfer internal.

## 5. PPOB / Digital Products

DesKaCash v0.1 sebaiknya sudah memiliki use case PPOB agar saldo wallet mempunyai utility nyata.

Target kategori:

- Pulsa.
- Paket data.
- Token PLN.
- PLN pascabayar.
- PDAM.
- BPJS.
- Internet.
- TV/cable.
- Multifinance/cicilan.
- PBB.
- Samsat/pajak yang tersedia dari provider.
- Voucher digital/game.
- Kategori PPOB lain yang tersedia dari provider.

Kategori aktual harus mengikuti produk yang benar-benar tersedia pada provider yang dipilih; jangan mengklaim coverage yang belum diverifikasi.

## 6. PPOB Architecture

Business logic DesKaCash tidak boleh bergantung langsung pada satu provider.

Gunakan abstraction:

```go
type PPOBProvider interface {
    GetProducts(ctx context.Context, req ProductRequest) (
        []Product,
        error,
    )

    Inquiry(ctx context.Context, req InquiryRequest) (
        InquiryResult,
        error,
    )

    Purchase(ctx context.Context, req PurchaseRequest) (
        PurchaseResult,
        error,
    )

    GetStatus(ctx context.Context, ref string) (
        PurchaseStatus,
        error,
    )

    HandleWebhook(
        ctx context.Context,
        payload []byte,
    ) error
}
```

Adapter:

```
PPOBService
    |
    +-- PPOBProvider
           |
           +-- DigiflazzAdapter
           +-- FutureProviderAdapter
           +-- MockPPOBProvider
```

Provider pertama dapat menggunakan provider PPOB yang menyediakan API produk digital dan tagihan. Digiflazz merupakan salah satu kandidat yang perlu diuji sebelum production.

## 7. PPOB Payment Flow

### Product purchase

```
User
  -> pilih produk
  -> validasi nomor tujuan
  -> cek saldo
  -> reserve saldo
  -> create provider transaction
  -> provider SUCCESS
  -> commit ledger
  -> return result
```

Jika provider FAILED:

```
Provider FAILED
  -> release reservation
  -> wallet kembali ke available balance
```

Jika provider PENDING:

```
Provider PENDING
  -> transaction tetap PROCESSING/PENDING
  -> jangan commit final debit sebagai SUCCESS
  -> polling/webhook
  -> settlement saat provider confirmed
```

## 8. Inquiry Sebelum Payment

Untuk tagihan/pascabayar:

```
Customer ID
   -> Inquiry
   -> nama pelanggan
   -> periode/tagihan
   -> nominal
   -> admin fee
   -> total
   -> user confirmation
   -> reserve balance
   -> payment
```

DesKaCash tidak boleh melakukan payment berdasarkan input nominal user saja apabila provider membutuhkan inquiry.

## 9. Ledger dan Revenue

Contoh pembelian PPOB:

Provider cost:
- Rp18.500

Harga ke user:
- Rp20.000

Margin:
- Rp1.500

Secara konseptual ledger harus dapat merepresentasikan:
- pengurangan liability/customer wallet.
- biaya/settlement provider.
- platform revenue/margin.

Jangan mengubah saldo hanya dengan UPDATE balance biasa.

## 10. Midtrans Positioning

Untuk v0.1, Midtrans digunakan terutama sebagai **payment rail** untuk collection/top up dan payment methods yang tersedia pada akun DesKaCash.

Midtrans bukan source of truth wallet user.

Target:

```
DesKaCash Ledger
       |
       +-- Midtrans Adapter
       |     +-- Bank Transfer / VA
       |     +-- Dynamic QRIS
       |     +-- supported e-wallet payment
       |
       +-- Future Wallet/Payout Provider
```

Capability berikut tidak boleh diasumsikan tersedia hanya karena Midtrans mendukung payment method terkait:

- payout arbitrary ke rekening bank setiap user;
- payout arbitrary ke e-wallet setiap user;
- scan QRIS merchant sebagai wallet payment;
- VA permanen unik per user.

Capability tersebut harus diverifikasi dari produk/provider yang memang mendukung use case tersebut.

## 11. Feature Matrix

| Feature | v0.1 | Provider dependency | Status |
|---|---:|---:|---|
| Wallet balance | Yes | No | Target |
| P2P DesKaCash | Yes | No | Target |
| Transaction history | Yes | No | Target |
| Bank transfer top up | Yes | Yes | Midtrans |
| Dynamic QRIS top up | Yes | Yes | Midtrans |
| E-wallet top up | Yes | Yes | Depends on enabled method |
| Unique VA per user | No | Yes | Coming Soon |
| Bank withdrawal | No | Yes | Coming Soon |
| E-wallet payout | No | Yes | Coming Soon |
| Scan QRIS merchant | No | Yes | Coming Soon |
| Pulsa | Yes | Yes | PPOB provider |
| Paket data | Yes | Yes | PPOB provider |
| Token PLN | Yes | Yes | PPOB provider |
| PLN pascabayar | Yes | Yes | PPOB provider |
| PDAM | Yes* | Yes | Provider coverage |
| BPJS | Yes* | Yes | Provider coverage |
| Internet | Yes* | Yes | Provider coverage |
| TV | Yes* | Yes | Provider coverage |
| Multifinance | Yes* | Yes | Provider coverage |
| PBB | Yes* | Yes | Provider coverage |
| Samsat/pajak | Yes* | Yes | Provider coverage |
| Voucher digital/game | Yes* | Yes | Provider coverage |

`*` Target capability; produk/channel aktual wajib diverifikasi pada provider sebelum diaktifkan.

## 11A. Provider Gateway Boundary

DesKaCash v0.1 menetapkan arah arsitektur provider sebagai **separate service boundary** melalui `DesKaProvider`. Implementasi service terpisah belum termasuk pada tahap saat ini, tetapi business logic DesKaCash harus tetap provider-agnostic agar integrasi provider dapat dipindahkan ke gateway tersebut tanpa mengubah core wallet/ledger.

Target architecture:

```
DesKaCash
    |
    v
DesKaProvider
    |
    +-- Payment adapters
    +-- PPOB adapters
    +-- Payout adapters
    +-- Wallet adapters
    |
    +-- External providers
```

Provider-specific concerns seperti authentication, request/response mapping, provider status, provider transaction ID, webhook normalization, signature verification, dan provider-specific errors berada pada adapter/gateway layer.

DesKaCash tetap menjadi pemilik business semantics dan ledger source of truth. Provider eksternal tidak boleh dianggap sebagai canonical balance source.

### Planned Provider Categories

- Payment / collection: Midtrans, DOKU, Xendit, RCB, dan provider lain yang telah diverifikasi.
- PPOB: XP SINDONESIA, Digiflazz, RCB, dan provider lain yang telah diverifikasi.
- Payout / external money movement: provider yang memang mendukung capability tersebut.
- Wallet / platform: hanya jika capability provider sesuai dengan use case dan hasil verifikasi.

Daftar tersebut adalah candidate/provider direction, bukan klaim bahwa seluruh capability sudah aktif atau tersedia pada DesKaCash v0.1.

### Future Open API Direction

Setelah internal provider contract, security, compliance, sandbox, dan operational model matang, DesKaProvider dapat dikembangkan menjadi Open API untuk developer eksternal.

Target konseptual:

```
Developer / DesKaCash
        |
        v
DesKaProvider API v1
        |
        +-- Payment
        +-- PPOB
        +-- Payout
        +-- Future wallet capabilities
        |
        v
External Providers / DesKa Infrastructure
```

Open API merupakan **future direction**, bukan fitur v0.1 yang sudah tersedia.

## 12. Prinsip Implementasi

1. Ledger DesKaCash adalah source of truth saldo.
2. Provider transaction tidak boleh langsung menjadi saldo.
3. Semua webhook harus idempotent.
4. Duplicate webhook tidak boleh menggandakan credit/debit.
5. Transaction provider harus dapat direconcile.
6. Debit user untuk PPOB harus memiliki reservation/atomicity yang aman.
7. Provider failure harus dapat me-release reservation.
8. Reversal dilakukan melalui transaction baru, bukan mutation posting lama.
9. Provider-specific code berada di adapter layer.
10. Business logic harus tetap provider-agnostic.
11. Feature yang belum memiliki provider capability harus ditandai `COMING_SOON`, bukan dipalsukan seolah sudah tersedia.

## 13. Roadmap Ringkas

### Phase A — Wallet Core
- Wallet/account.
- Ledger.
- Balance.
- P2P.
- Transaction history.
- Idempotency.

### Phase B — Collection
- Midtrans bank transfer/VA.
- Midtrans dynamic QRIS.
- Supported e-wallet top up.
- Webhook.
- Reconciliation.

### Phase C — Wallet Utility
- PPOB abstraction.
- Product catalog.
- Inquiry.
- Purchase.
- Status.
- Pulsa/data.
- PLN token/tagihan.
- PPOB categories lainnya sesuai provider.

### Phase D — External Money Movement
- Unique VA per user.
- Bank payout.
- E-wallet payout.
- Withdrawal.

### Phase E — Merchant Payment
- Scan QRIS.
- QRIS merchant payment.
- Merchant onboarding.
- Merchant settlement.
- DesKaCash merchant ecosystem.

---

## 14. MVP Principle

DesKaCash tidak perlu menunggu seluruh kemampuan financial infrastructure tersedia untuk mulai menjadi wallet yang berguna.

Target v0.1:

```
TOP UP
  |
  v
WALLET BALANCE
  |
  +--> P2P
  |
  +--> PULSA
  |
  +--> DATA
  |
  +--> PLN
  |
  +--> PPOB
  |
  +--> voucher/digital products
  |
  v
Future:
BANK / E-WALLET / QRIS MERCHANT
```

Dengan pendekatan ini, provider dapat diganti/ditambah tanpa mengubah core ledger DesKaCash.

## 15. IndoChain & dIDR Integration Direction

DesKaCash dirancang agar pada tahap lanjutan dapat menggunakan **IndoChain** sebagai blockchain layer untuk aset on-chain **dIDR**. Integrasi ini tidak menggantikan ledger fiat DesKaCash pada v0.1.

### Pemisahan Source of Truth

`text
                    DesKaEcosystem
                           │
             ┌─────────────┴─────────────┐
             │                           │
        DesKaCash                    IndoChain
       Fiat Ledger                 Blockchain State
             │                           │
          IDR Fiat                      dIDR
             │                           │
             └────────── Conversion ─────┘
`

Prinsipnya:
- PostgreSQL double-entry ledger tetap menjadi source of truth untuk saldo fiat IDR DesKaCash.
- IndoChain menjadi source of truth untuk saldo dan state dIDR on-chain.
- DesKaCash tidak perlu menduplikasi saldo blockchain sebagai canonical balance.
- Database DesKaCash cukup menyimpan mapping seperti `user_id`, `blockchain_address`, `chain_id`, wallet type, dan status.
- Perubahan saldo blockchain harus berasal dari state/transaction IndoChain yang dapat diverifikasi.
- Konversi IDR ↔ dIDR merupakan proses settlement/conversion tersendiri.

### User Wallet Mapping

`text
DesKaCash User
│
├── user_id
├── fiat wallet/account
│      └── IDR ledger balance
│
└── IndoChain wallet
       ├── native account
       ├── EVM account (future)
       └── dIDR balance
`

Detail address/account binding mengikuti spesifikasi IndoChain. DesKaCash tidak menentukan format address protocol secara mandiri.

### dIDR sebagai Native Asset IndoChain

`dIDR` adalah native asset IndoChain. Pada tahap integrasi blockchain, DesKaCash dapat menggunakan dIDR untuk transfer on-chain, settlement berbasis blockchain, integrasi smart contract, dan kebutuhan ecosystem IndoChain.

DesKaCash harus membedakan dengan tegas antara **IDR fiat** dan **dIDR on-chain**, walaupun keduanya memiliki hubungan denominasi/konversi yang ditentukan oleh desain ekonomi IndoChain.

### EVM Smart Contract Integration

`text
DesKaCash Service
      │
      ├── Native IndoChain RPC
      │
      └── EVM JSON-RPC
               │
               ↓
          IndoChain EVM
               │
        Smart Contracts
`

Target developer workflow:

`text
Solidity
   ↓
Hardhat / Foundry / Remix
   ↓
IndoChain JSON-RPC
   ↓
Deploy / Call / Transaction
`

EVM merupakan execution layer IndoChain; DesKaCash tidak menjadi execution environment smart contract.

### Fee Sponsorship

IndoChain dirancang memiliki native **Fee Sponsorship** sehingga aplikasi seperti DesKaCash secara konseptual dapat mensponsori fee transaksi user apabila protocol dan policy ecosystem mengizinkannya.

`text
DesKaCash User
      │
      │ signed transaction
      ↓
IndoChain
      │
      ├── User authorization
      ├── Fee calculation
      └── Fee Sponsor
              ↓
          Transaction fee
`

Implementasi final fee payer/sponsor, authorization, limits, replay protection, dan settlement fee mengikuti spesifikasi protocol IndoChain. DesKaCash tidak boleh mengasumsikan mekanisme ERC-4337/Paymaster tertentu.

### Boundary DesKaCash ↔ IndoChain

`text
DesKaCash
├── Fiat ledger
├── Top up
├── P2P internal
├── PPOB
├── Provider integration
└── Blockchain integration service
             │
             ├── Read on-chain balance
             ├── Submit/track transaction
             ├── Verify transaction/finality
             └── IDR ↔ dIDR conversion
                          │
                          ↓
                      IndoChain
                      ├── Native state
                      ├── dIDR
                      ├── Consensus
                      ├── EVM
                      └── Smart contracts
`

Integrasi blockchain tidak boleh membuat provider payment, PPOB, atau database DesKaCash menjadi sumber kebenaran untuk state IndoChain.

### Roadmap Integrasi

#### v0.1 — Fiat Wallet
- IDR wallet ledger.
- Top up.
- P2P internal.
- PPOB.
- Provider abstraction.

#### v0.x — IndoChain Connectivity
- Blockchain address mapping.
- Native RPC client.
- Read dIDR balance langsung dari IndoChain.
- Transaction submission/tracking.
- Confirmation/finality verification.

#### v1.x — dIDR & EVM Ecosystem
- IDR ↔ dIDR conversion/settlement.
- dIDR transfer.
- EVM smart contract interaction.
- Developer/API integration.
- Native IndoChain fee sponsorship integration.

Fitur blockchain tetap dipisahkan dari MVP fiat wallet sampai interface protocol IndoChain cukup stabil untuk digunakan secara aman.
