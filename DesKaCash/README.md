# DesKaCash

**DesKaCash** adalah platform **digital wallet (e-wallet)** yang merupakan bagian dari **DesKaEcosystem**.

DesKaCash dirancang untuk menyediakan berbagai layanan transaksi keuangan digital seperti pembayaran, transfer dana, top-up, withdrawal, pembayaran tagihan, pembayaran QR, serta integrasi dengan bank, payment gateway, dan layanan keuangan pihak ketiga.

Yang membedakan DesKaCash dari e-wallet konvensional adalah **arsitektur ledger transaksi internalnya yang dirancang terintegrasi dengan IndoChain**, yaitu blockchain yang dikembangkan sebagai bagian dari DesKaEcosystem.

Dengan pendekatan ini, DesKaCash tidak hanya berfungsi sebagai aplikasi e-wallet, tetapi juga menjadi salah satu **financial application layer** yang memanfaatkan IndoChain sebagai bagian dari infrastruktur pencatatan dan validasi transaksi.

> **DesKaCash = Digital Wallet + Financial API Integration + IndoChain Ledger**

---

## 🚀 Konsep DesKaCash

DesKaCash dirancang dengan konsep yang serupa dengan platform e-wallet modern seperti DANA, OVO, GoPay, dan platform pembayaran digital lainnya dalam hal pengalaman pengguna dan layanan transaksi.

Namun, arsitektur internal DesKaCash memiliki pendekatan yang berbeda.

Pada sistem DesKaCash:

```text
                   ┌──────────────────────┐
                   │      DesKaCash       │
                   │    Mobile / Client   │
                   └──────────┬───────────┘
                              │
                              ▼
                   ┌──────────────────────┐
                   │   DesKaCash Backend  │
                   │      API / Core      │
                   └──────────┬───────────┘
                              │
             ┌────────────────┼────────────────┐
             │                │                │
             ▼                ▼                ▼
       ┌───────────┐    ┌───────────┐    ┌──────────────┐
       │ IndoChain │    │ PostgreSQL│    │    Redis     │
       │   Ledger  │    │   Data    │    │ Cache / Queue│
       └───────────┘    └───────────┘    └──────────────┘
             │
             ▼
       Transaction Ledger
```

IndoChain digunakan sebagai bagian dari infrastruktur ledger untuk mencatat dan memvalidasi aktivitas transaksi sesuai dengan desain sistem DesKaCash.

Sementara itu, data operasional seperti profil pengguna, konfigurasi, metadata, session, dan kebutuhan aplikasi lainnya tetap dapat disimpan pada database tradisional seperti PostgreSQL.

---

# ⛓️ IndoChain Integration

Salah satu komponen utama DesKaCash adalah integrasi dengan **IndoChain**.

IndoChain dikembangkan sebagai blockchain internal dalam DesKaEcosystem yang dapat digunakan untuk mendukung pencatatan dan validasi transaksi pada berbagai aplikasi dalam ekosistem.

Dalam DesKaCash, IndoChain dirancang untuk menjadi bagian dari **transaction ledger layer**.

Contoh alur transaksi:

```text
User
 │
 │ Transfer Rp100.000
 ▼
DesKaCash API
 │
 ├── Validate User
 ├── Validate Balance
 ├── Validate Transaction
 │
 ▼
IndoChain
 │
 ├── Create Transaction
 ├── Validate Transaction
 ├── Record Ledger
 │
 ▼
Transaction Confirmed
 │
 ▼
DesKaCash
 │
 └── Update Transaction Status
```

Pendekatan ini memungkinkan sistem memiliki pemisahan antara:

* **Application Data**
* **Financial Transaction**
* **Transaction Ledger**
* **External Financial Integration**

---

# 💳 Fitur DesKaCash

DesKaCash dirancang untuk menyediakan fitur-fitur utama yang umum tersedia pada platform e-wallet modern.

### Pembayaran

* Pembayaran menggunakan QR Code.
* Pembayaran di merchant.
* Pembayaran online.
* Pembayaran melalui merchant yang terintegrasi dengan DesKaEcosystem.
* Integrasi dengan payment gateway.
* Integrasi dengan layanan pembayaran pihak ketiga.

### Transfer

* Transfer antar pengguna DesKaCash.
* Transfer ke rekening bank.
* Transfer ke e-wallet lainnya.
* Transfer melalui layanan finansial pihak ketiga.
* Penerimaan dana dari pengguna lain.
* Transaction tracking.

### Top Up

Pengguna dapat melakukan top-up melalui berbagai metode yang tersedia pada sistem, termasuk:

* Transfer bank.
* Virtual Account.
* Payment Gateway.
* Kartu pembayaran.
* E-wallet lain.
* Merchant yang mendukung top-up.
* Integrasi layanan finansial pihak ketiga.

Metode top-up akan bergantung pada provider dan integrasi yang tersedia.

### Withdrawal

DesKaCash dirancang untuk mendukung penarikan dana melalui:

* Rekening bank.
* ATM melalui provider yang mendukung.
* Merchant atau agen yang bekerja sama.
* Layanan finansial pihak ketiga.

### QR Payment

DesKaCash akan mendukung sistem pembayaran berbasis QR untuk memudahkan transaksi antara pengguna dan merchant.

Contoh:

```text
User
 │
 │ Scan QR
 ▼
DesKaCash
 │
 ▼
Validate Payment
 │
 ▼
IndoChain Ledger
 │
 ▼
Transaction Confirmed
 │
 ▼
Merchant
```

---

# 🧾 Transaction History

DesKaCash menyediakan riwayat transaksi untuk membantu pengguna melihat aktivitas keuangan mereka.

Informasi transaksi dapat mencakup:

* ID transaksi.
* Jenis transaksi.
* Nominal.
* Waktu transaksi.
* Pengirim.
* Penerima.
* Merchant.
* Status transaksi.
* Reference ID.
* Transaction hash / blockchain reference.
* Provider reference.

Contoh:

```text
Transaction ID
     │
     ├── DesKaCash Transaction ID
     ├── IndoChain Transaction ID
     ├── External Provider Reference
     └── Transaction Status
```

---

# 🔔 Notification

DesKaCash akan menyediakan sistem notifikasi untuk memberikan informasi kepada pengguna mengenai aktivitas akun dan transaksi.

Notifikasi dapat digunakan untuk:

* Transfer masuk.
* Transfer keluar.
* Pembayaran berhasil.
* Pembayaran gagal.
* Top-up berhasil.
* Withdrawal.
* Perubahan status transaksi.
* Promo.
* Informasi keamanan.
* Aktivitas akun.

---

# 🧾 Pembayaran Tagihan

DesKaCash dirancang untuk mendukung pembayaran berbagai kebutuhan rutin melalui integrasi provider.

Contohnya:

* Listrik.
* Air.
* Internet.
* Telepon.
* TV berlangganan.
* Asuransi.
* Cicilan.
* Produk digital.
* Layanan berlangganan lainnya.

Ketersediaan layanan bergantung pada provider yang terintegrasi dengan DesKaCash.

---

# 🏛️ Integrasi Bank & Financial Provider

DesKaCash dirancang agar dapat berkomunikasi dengan berbagai pihak ketiga melalui API.

Contohnya:

```text
                    DesKaCash
                        │
          ┌─────────────┼─────────────┐
          │             │             │
          ▼             ▼             ▼
        Bank      Payment Gateway   Provider
          │             │             │
          └─────────────┼─────────────┘
                        │
                        ▼
                   DesKaCash API
                        │
                        ▼
                    IndoChain
```

Integrasi dapat mencakup:

* Bank API.
* Payment Gateway.
* QR Payment Provider.
* Virtual Account Provider.
* E-wallet Provider.
* Bill Payment Provider.
* Financial Service Provider.
* API pihak ketiga lainnya.

DesKaCash tidak bergantung pada satu provider tertentu sehingga arsitektur backend dirancang agar integrasi provider dapat ditambahkan secara modular.

---

# 🔐 Security

Keamanan merupakan salah satu bagian penting dalam desain DesKaCash.

Sistem dirancang untuk mendukung:

* HTTPS/TLS.
* JWT Authentication.
* Refresh Token.
* Two-Factor Authentication (2FA).
* Secure Storage.
* Encryption.
* API Authentication.
* Rate Limiting.
* Transaction Validation.
* Permission & Role Management.
* Audit Log.
* Device Management.
* Session Management.
* Fraud Detection.
* Transaction Monitoring.

Keamanan pada level blockchain juga menjadi bagian dari desain IndoChain dan mekanisme validasi transaksi.

---

# 🏦 Financial Ledger Architecture

Salah satu konsep utama DesKaCash adalah pemisahan antara **application database** dan **transaction ledger**.

### PostgreSQL

PostgreSQL digunakan untuk menyimpan data aplikasi seperti:

* User.
* Profile.
* Account.
* Merchant.
* Provider.
* Configuration.
* Session.
* Notification.
* Metadata.
* Operational data.

### IndoChain

IndoChain digunakan sebagai bagian dari ledger transaksi untuk:

* Transaction recording.
* Transaction validation.
* Ledger verification.
* Transaction reference.
* Auditability.
* Interoperability dengan aplikasi DesKaEcosystem.

Secara konseptual:

```text
PostgreSQL
    │
    ├── User
    ├── Account
    ├── Merchant
    ├── Provider
    └── Application Data

IndoChain
    │
    ├── Transaction
    ├── Ledger
    ├── Validation
    └── Transaction Reference
```

Dengan demikian, database aplikasi dan ledger transaksi tidak harus memiliki fungsi yang sama.

---

# ⚙️ Backend DesKaCash

Backend DesKaCash menggunakan **Go** sebagai bahasa utama untuk membangun backend dan API.

Komponen backend yang direncanakan meliputi:

* Go.
* Gin.
* PostgreSQL.
* Redis.
* IndoChain.
* JWT.
* Docker.
* GitHub Actions.
* Prometheus.
* Grafana.
* Swagger / OpenAPI.
* API integration layer.
* RPC service.

### Go

Go digunakan sebagai bahasa utama backend karena sesuai untuk membangun service yang membutuhkan performa tinggi, concurrency, networking, dan integrasi API.

### Gin

Gin digunakan sebagai HTTP framework untuk membangun REST API DesKaCash.

### PostgreSQL

PostgreSQL digunakan sebagai database utama untuk application data dan operational data.

### Redis

Redis digunakan untuk kebutuhan seperti:

* Cache.
* Session.
* Rate limiting.
* Temporary data.
* Queue / asynchronous processing sesuai kebutuhan arsitektur.

### Docker

Docker digunakan untuk membantu proses:

* Development.
* Testing.
* Deployment.
* Environment isolation.
* Service management.

### GitHub Actions

GitHub Actions digunakan untuk menjalankan proses CI/CD, termasuk:

* Automated testing.
* `go test`.
* `go vet`.
* Build.
* Deployment pipeline.

### Monitoring

DesKaCash dirancang untuk menggunakan monitoring seperti:

* Prometheus.
* Grafana.

Monitoring digunakan untuk mengamati:

* API performance.
* Request rate.
* Error rate.
* Database performance.
* Service health.
* Transaction processing.
* Resource usage.

### API Documentation

API DesKaCash akan menggunakan standar dokumentasi seperti:

* Swagger.
* OpenAPI.

Dokumentasi API ditujukan untuk mempermudah integrasi internal maupun integrasi dengan service pihak ketiga.

---

# 🔌 Internal Service Communication

Selain REST API, service dalam DesKaEcosystem dapat berkomunikasi menggunakan **RPC**.

Contoh:

```text
DesKaCash
    │
    ├── REST API
    │
    ├── RPC
    │
    └── IndoChain
            │
            ├── Transaction Service
            ├── Ledger Service
            └── Blockchain Service
```

RPC digunakan untuk komunikasi internal antar service yang membutuhkan komunikasi langsung dan efisien.

---

# 📱 Frontend DesKaCash

Frontend DesKaCash dirancang menggunakan:

* Dart.
* Flutter.

Aplikasi ditargetkan untuk:

* Android.
* iOS.

Flutter digunakan agar satu codebase dapat digunakan untuk beberapa platform.

### State Management

State management dapat menggunakan pendekatan seperti:

* Riverpod.
* BLoC.

Pemilihan state management akan mengikuti kebutuhan arsitektur aplikasi.

### HTTP Client

Komunikasi dengan backend menggunakan HTTP client seperti:

* Dio.

### Local Storage

Data lokal yang tidak sensitif dapat menggunakan storage seperti:

* Shared Preferences.

Data sensitif dapat menggunakan:

* Flutter Secure Storage.

### Push Notification

Layanan seperti Firebase dapat digunakan untuk kebutuhan:

* Push Notification.
* Analytics.
* Authentication jika diperlukan.

### UI & Animation

Flutter package seperti Lottie dapat digunakan untuk animasi dan meningkatkan pengalaman pengguna.

### Localization

Internationalization dan localization dapat menggunakan:

* Flutter Intl.
* Sistem localization bawaan Flutter.

---

# 💰 Layanan Keuangan

DesKaCash dirancang untuk menyediakan berbagai layanan finansial digital melalui integrasi dengan provider yang tersedia.

### Pembayaran Tagihan

* PLN.
* PDAM.
* Internet.
* Telepon.
* TV.
* Asuransi.
* Layanan berlangganan.

### Cicilan

* Cicilan kredit.
* Cicilan belanja.
* Cicilan layanan.
* Produk finansial lainnya yang tersedia melalui provider.

### Pajak

Integrasi pembayaran pajak dapat dikembangkan sesuai dengan provider dan layanan resmi yang tersedia.

Contohnya:

* PBB.
* Pajak kendaraan.
* Pajak lainnya.

### Donasi

DesKaCash dapat menyediakan pembayaran donasi melalui organisasi atau provider yang telah terintegrasi.

---

# 🎁 Promo & Loyalty

DesKaCash dapat dikembangkan dengan sistem:

* Promo.
* Voucher.
* Cashback.
* Discount.
* Reward.
* Loyalty point.

Sistem tersebut dapat dikembangkan sebagai bagian dari ekosistem DesKaCash dan DesKaEcosystem.

---

# 🏪 Merchant

DesKaCash dirancang tidak hanya untuk pengguna individu tetapi juga merchant.

Merchant dapat memiliki kemampuan untuk:

* Menerima pembayaran.
* Membuat QR pembayaran.
* Melihat transaksi.
* Melihat settlement.
* Melihat laporan.
* Mengelola produk.
* Mengelola staff.
* Mengelola rekening settlement.
* Mengakses API merchant.

Arsitektur merchant dapat dikembangkan menjadi bagian tersendiri dari DesKaEcosystem.

---

# 🌐 DesKaEcosystem

DesKaCash merupakan salah satu komponen dalam DesKaEcosystem.

Secara konseptual:

```text
                       DesKaEcosystem
                              │
        ┌─────────────────────┼─────────────────────┐
        │                     │                     │
        ▼                     ▼                     ▼
    DesKaCash             IndoChain             Other Apps
        │                     │                     │
        │                     │                     │
        ├───────────────┬─────┴───────────────┬─────┤
        │               │                     │
        ▼               ▼                     ▼
      Bank          Financial API          Internal API
        │               │                     │
        └───────────────┴─────────────────────┘
```

Dengan arsitektur tersebut, aplikasi dalam DesKaEcosystem dapat saling berkomunikasi dan memanfaatkan service yang tersedia.

---

# 🗺️ Roadmap

Pengembangan DesKaCash dilakukan secara bertahap.

### Phase 1 — Core Platform

* User authentication.
* User account.
* Wallet.
* Balance.
* Transaction.
* Transaction history.
* Basic API.
* PostgreSQL.
* Redis.
* IndoChain integration.

### Phase 2 — Payment

* QR Payment.
* Merchant.
* Top-up.
* Withdrawal.
* Payment Gateway integration.

### Phase 3 — Financial Integration

* Bank API.
* Virtual Account.
* E-wallet integration.
* Bill payment.
* Financial provider integration.

### Phase 4 — Ecosystem

* Merchant platform.
* Developer API.
* Webhook.
* RPC service.
* Settlement.
* Reporting.
* Analytics.

### Phase 5 — Advanced Financial Infrastructure

* Advanced transaction monitoring.
* Fraud detection.
* Automated reconciliation.
* Multi-provider routing.
* Advanced ledger management.
* Extended IndoChain integration.

---

# ⚠️ Development Status

> **DesKaCash saat ini merupakan proyek yang sedang dalam tahap pengembangan.**

Fitur, integrasi, arsitektur, provider, dan teknologi yang tercantum dalam README ini dapat berubah selama proses pengembangan.

Integrasi dengan bank, payment gateway, e-wallet, layanan finansial, dan layanan pihak ketiga lainnya bergantung pada ketersediaan API, persyaratan teknis, serta ketentuan masing-masing provider.

Penggunaan layanan keuangan secara publik juga akan mengikuti persyaratan hukum, regulasi, perizinan, dan ketentuan yang berlaku pada yurisdiksi terkait.

---

# 🏗️ DesKaEcosystem

DesKaCash dikembangkan sebagai bagian dari visi **DesKaEcosystem**, yaitu ekosistem teknologi yang menghubungkan berbagai aplikasi, service, API, dan infrastructure dalam satu platform.

Komponen utama yang menjadi bagian dari ekosistem ini antara lain:

* **DesKaCash** — Digital Wallet & Financial Platform.
* **IndoChain** — Blockchain & Transaction Ledger Infrastructure.
* **DesKaEcosystem API** — Integrasi antar aplikasi dan service.
* **Financial Integration Layer** — Integrasi dengan bank dan financial provider.
* **Merchant Platform** — Infrastruktur untuk merchant dan bisnis.

---

## License

License proyek akan ditentukan sesuai kebijakan dan tahap pengembangan DesKaEcosystem.

---

## v0.1 Development Branch

This branch is the dedicated development track for the DesKaCash v0.1 implementation.

- Backend: Go
- Frontend: Flutter / Dart
- Integration boundary: IndoChain RPC/API
- External payment providers: application-layer integrations

DesKaCash remains independently developed from IndoChain. IndoChain source code is not a dependency of this project; integration occurs through defined RPC/API contracts.
