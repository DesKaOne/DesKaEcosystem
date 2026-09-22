# DesKaEcosystem

**DesKa Ecosystem** adalah ekosistem teknologi blockchain dan finansial yang dibangun dengan **IndoChain** sebagai fondasi blockchain dan ledger utama.

Ekosistem ini dirancang untuk menghubungkan teknologi blockchain dengan berbagai layanan finansial berbasis Rupiah, mulai dari **native blockchain wallet** hingga layanan seperti **DesKaCash**, **DesKaBank**, **DesKaCard**, dan berbagai layanan **DesKa*** lainnya di masa depan.

```text
                         DESKA ECOSYSTEM
                                │
                                ▼
                    ┌─────────────────────┐
                    │      INDOCHAIN      │
                    │ Blockchain / Ledger │
                    │        dIDR         │
                    └──────────┬──────────┘
                               │
              ┌────────────────┼────────────────┐
              │                │                │
              ▼                ▼                ▼
       IndoChainWallet    Smart Contract     Token
       Native Wallet      Infrastructure    Infrastructure
              │
              │
      ┌───────┼────────────────────────────────┐
      │       │                │               │
      ▼       ▼                ▼               ▼
 DesKaCash DesKaBank       DesKaCard       DesKa*
      │       │                │               │
      └───────┴────────────────┴───────────────┘
                              │
                              ▼
                    Financial Services
                      Berbasis Rupiah
```

---

## Ecosystem Architecture

DesKa Ecosystem menggunakan pemisahan antara **blockchain infrastructure** dan **financial application layer**.

### IndoChain

**IndoChain** adalah blockchain utama yang menjadi fondasi DesKa Ecosystem.

IndoChain menyediakan infrastruktur blockchain seperti:

* Blockchain ledger
* Block
* Transaction
* Account state
* Native asset
* dIDR
* Cryptography
* Mempool
* Transaction execution
* State management
* Smart contract
* Token infrastructure
* Consensus
* P2P networking
* Blockchain protocol

IndoChain menjadi **underlying ledger** yang dapat digunakan oleh berbagai layanan dalam DesKa Ecosystem.

---

## dIDR

IndoChain memiliki native asset bernama:

> **dIDR — decentralized Indonesian Rupiah**

dIDR merupakan native asset yang digunakan di dalam blockchain IndoChain.

Denominasi dIDR dalam DesKa Ecosystem:

|         dIDR | Nilai Rupiah |
| -----------: | -----------: |
|     `1 dIDR` |      Rp1.000 |
|   `0.1 dIDR` |        Rp100 |
|  `0.01 dIDR` |         Rp10 |
| `0.001 dIDR` |          Rp1 |

Sehingga:

```text
1 dIDR = Rp1.000
1 IDR  = 0.001 dIDR
```

Contoh:

```text
Rp1.000      → 1 dIDR
Rp10.000     → 10 dIDR
Rp100.000    → 100 dIDR
Rp1.000.000  → 1.000 dIDR
```

> **Catatan:** `1 dIDR = Rp1.000` merupakan denominasi yang digunakan dalam desain IndoChain dan DesKa Ecosystem. Denominasi tersebut tidak dengan sendirinya menetapkan mekanisme backing, reserve, redemption, ataupun status hukum/keuangan dari dIDR.

---

# IndoChainWallet

**IndoChainWallet** adalah native blockchain wallet untuk berinteraksi secara langsung dengan IndoChain.

Berbeda dengan layanan finansial DesKa*, IndoChainWallet ditujukan untuk pengguna yang membutuhkan akses langsung terhadap aset dan identitas blockchain.

```text
IndoChainWallet
      │
      ├── Address
      ├── Public Key
      ├── Private Key
      ├── dIDR
      ├── Transaction
      └── Blockchain Interaction
```

Alur sederhananya:

```text
User
 │
 ▼
IndoChainWallet
 │
 ├── Address
 ├── Private Key
 └── Transaction Signing
 │
 ▼
IndoChain
 │
 ▼
Blockchain Ledger
```

Private key merupakan bagian dari wallet dan digunakan untuk melakukan signing transaksi.

Backend wallet dapat menerima **signed transaction**, tetapi private key tidak perlu dikirim ke backend.

---

# DesKa Financial Services

Di atas IndoChain terdapat berbagai layanan finansial dalam DesKa Ecosystem.

Contohnya:

* **DesKaCash**
* **DesKaBank**
* **DesKaCard**
* **DesKaPay**
* **DesKaMerchant**
* dan berbagai layanan **DesKa*** lainnya.

Layanan tersebut memiliki pendekatan berbeda dari IndoChainWallet.

Pengguna tidak harus berinteraksi langsung dengan detail blockchain seperti:

* Private key
* Public key
* Blockchain address
* Transaction hash
* Block
* Mempool
* dIDR denomination

Sebaliknya, pengguna berinteraksi menggunakan konsep finansial yang lebih familiar, yaitu **Rupiah**.

Detail blockchain dapat dikelola oleh backend dan infrastructure layer DesKa.

---

# DesKaCash

**DesKaCash** adalah layanan financial wallet dalam DesKa Ecosystem.

DesKaCash menggunakan **Rupiah sebagai denominasi utama pada user interface**, sementara IndoChain digunakan sebagai underlying blockchain ledger.

Contoh:

```text
User
 │
 │ Rp10.000
 ▼
DesKaCash
 │
 ▼
DesKaCash Backend
 │
 │  Rp10.000 → 10 dIDR
 ▼
IndoChain RPC
 │
 ▼
IndoChain
 │
 ▼
Blockchain Ledger
```

Pengguna dapat melihat:

```text
Saldo: Rp100.000
```

sementara layer blockchain merepresentasikan nilai tersebut sebagai:

```text
100 dIDR
```

karena:

```text
1 dIDR = Rp1.000
```

Dengan pendekatan ini, pengguna DesKaCash tidak harus mengetahui atau mengelola private key maupun blockchain address secara langsung.

---

# Backend-to-IndoChain

Layanan finansial DesKa dapat berkomunikasi dengan IndoChain melalui backend.

Secara konseptual:

```text
┌──────────────┐
│     User     │
└──────┬───────┘
       │
       │ Rupiah
       ▼
┌──────────────┐
│  DesKaCash   │
│      UI      │
└──────┬───────┘
       │
       ▼
┌──────────────┐
│   Backend    │
│ DesKaCash    │
└──────┬───────┘
       │
       │ dIDR transaction
       ▼
┌──────────────┐
│ IndoChain RPC│
└──────┬───────┘
       │
       ▼
┌──────────────┐
│  IndoChain   │
│    Ledger    │
└──────────────┘
```

Dengan demikian, aplikasi finansial tidak harus mengekspos detail blockchain kepada pengguna.

Backend dapat menangani integrasi seperti:

* Transaction creation
* Transaction validation
* Transaction signing sesuai model custody yang digunakan
* RPC communication
* Transaction submission
* Transaction status
* Balance synchronization
* Ledger reconciliation

Implementasi detail setiap layanan akan mengikuti kebutuhan dan model keamanan masing-masing project.

---

# DesKaBank

**DesKaBank** direncanakan sebagai layanan banking berbasis Rupiah dalam DesKa Ecosystem.

DesKaBank dapat menggunakan IndoChain sebagai underlying ledger untuk aktivitas yang relevan dengan blockchain infrastructure.

```text
DesKaBank
    │
    ▼
Financial Service Layer
    │
    ▼
IndoChain Integration
    │
    ▼
IndoChain
    │
    ▼
Blockchain Ledger
```

Detail fitur dan arsitektur DesKaBank akan dikembangkan sebagai project tersendiri.

---

# DesKaCard

**DesKaCard** direncanakan sebagai layanan card dan payment dalam DesKa Ecosystem.

Secara konseptual:

```text
DesKaCard
    │
    ▼
DesKa Financial Infrastructure
    │
    ▼
IndoChain Integration
    │
    ▼
IndoChain
```

DesKaCard dapat terintegrasi dengan layanan finansial lainnya dalam ecosystem sesuai kebutuhan implementasi.

---

# Shared Ledger

Salah satu prinsip utama DesKa Ecosystem adalah penggunaan **IndoChain sebagai shared blockchain ledger**.

Layanan DesKa tidak perlu memiliki blockchain sendiri-sendiri.

```text
                 ┌──────────────┐
                 │  DesKaCash   │
                 └──────┬───────┘
                        │
                 ┌──────▼───────┐
                 │  DesKaBank   │
                 └──────┬───────┘
                        │
                 ┌──────▼───────┐
                 │  DesKaCard   │
                 └──────┬───────┘
                        │
                        ▼
                 ┌──────────────┐
                 │   IndoChain  │
                 │ Shared Ledger│
                 └──────────────┘
```

Dengan pendekatan ini, IndoChain menjadi lapisan blockchain bersama yang dapat digunakan oleh berbagai produk dan layanan dalam DesKa Ecosystem.

---

# Dua Model Interaksi

DesKa Ecosystem memiliki dua model utama dalam berinteraksi dengan IndoChain.

## 1. Direct Blockchain Access

Digunakan oleh **IndoChainWallet**.

```text
User
 │
 ▼
IndoChainWallet
 │
 ├── Address
 ├── Private Key
 ├── Sign Transaction
 └── dIDR
 │
 ▼
IndoChain
```

Pada model ini pengguna memang berinteraksi langsung dengan konsep blockchain.

---

## 2. Financial Service Access

Digunakan oleh **DesKaCash**, **DesKaBank**, **DesKaCard**, dan layanan DesKa lainnya.

```text
User
 │
 ▼
DesKa Application
 │
 │ Rupiah
 ▼
DesKa Backend
 │
 │ dIDR
 ▼
IndoChain
 │
 ▼
Blockchain Ledger
```

Pada model ini blockchain menjadi **underlying infrastructure**, sedangkan aplikasi memberikan pengalaman finansial berbasis Rupiah.

---

# Rupiah ↔ dIDR

Mapping denominasi antara financial layer dan blockchain layer:

| Financial Layer | Blockchain Layer |
| --------------: | ---------------: |
|             Rp1 |     `0.001 dIDR` |
|            Rp10 |      `0.01 dIDR` |
|           Rp100 |       `0.1 dIDR` |
|         Rp1.000 |         `1 dIDR` |
|        Rp10.000 |        `10 dIDR` |
|       Rp100.000 |       `100 dIDR` |
|     Rp1.000.000 |     `1.000 dIDR` |

Secara konseptual:

```text
┌──────────────────────────┐
│   Financial Layer        │
│                          │
│   Rp1.000.000            │
└────────────┬─────────────┘
             │
             │ denomination mapping
             ▼
┌──────────────────────────┐
│   Blockchain Layer       │
│                          │
│   1.000 dIDR              │
└────────────┬─────────────┘
             │
             ▼
┌──────────────────────────┐
│       IndoChain          │
│                          │
│        Ledger            │
└──────────────────────────┘
```

Implementasi protocol sebaiknya menggunakan integer/base unit untuk menghindari masalah floating-point dalam perhitungan nilai.

---

# Project Structure

Repository ini merupakan root dari DesKa Ecosystem.

Struktur project dapat berkembang seiring bertambahnya layanan:

```text
DesKaEcosystem/
│
├── IndoChain/
│   └── Blockchain infrastructure
│
├── IndoChainWallet/
│   └── Native blockchain wallet
│
├── DesKaCash/
│   └── Financial wallet / payment service
│
├── DesKaBank/
│   └── Banking service
│
├── DesKaCard/
│   └── Card / payment service
│
└── ...
```

Setiap project memiliki tanggung jawab masing-masing dan dapat berkembang secara independen selama tetap mengikuti protocol dan integration contract yang ditetapkan ecosystem.

---

# Architecture Principles

### 1. IndoChain sebagai Blockchain Infrastructure

IndoChain bertanggung jawab terhadap blockchain, protocol, state, transaction, ledger, dan native asset.

### 2. IndoChainWallet sebagai Native Blockchain Wallet

IndoChainWallet menyediakan akses langsung terhadap blockchain dan identitas blockchain pengguna.

### 3. DesKa* sebagai Financial Services

DesKaCash, DesKaBank, DesKaCard, dan layanan lainnya memberikan pengalaman finansial berbasis Rupiah.

### 4. Shared Ledger

Berbagai layanan dapat menggunakan IndoChain sebagai underlying ledger.

### 5. Blockchain Abstraction

Layanan finansial tidak harus mengekspos detail blockchain kepada pengguna.

### 6. Clear Separation of Responsibility

```text
IndoChain
    ↓
Blockchain Infrastructure

IndoChainWallet
    ↓
Direct Blockchain Access

DesKaCash / DesKaBank / DesKaCard / ...
    ↓
Financial Services
```

---

# Roadmap

## Phase 1 — IndoChain Core

* [ ] Blockchain core
* [ ] Block
* [ ] Transaction
* [ ] State
* [ ] Ledger
* [ ] Native dIDR
* [ ] Cryptography
* [ ] Mempool
* [ ] Genesis

## Phase 2 — IndoChain Network

* [ ] Persistent blockchain storage
* [ ] Node lifecycle
* [ ] P2P networking
* [ ] Consensus
* [ ] Mainnet infrastructure
* [ ] RPC infrastructure

## Phase 3 — IndoChainWallet

* [ ] Wallet generation
* [ ] Address management
* [ ] Private key management
* [ ] dIDR balance
* [ ] Transaction signing
* [ ] Transaction broadcasting
* [ ] Transaction history
* [ ] Blockchain explorer integration

## Phase 4 — DesKa Financial Services

* [ ] DesKaCash
* [ ] DesKaBank
* [ ] DesKaCard
* [ ] DesKaPay
* [ ] DesKaMerchant
* [ ] Other DesKa* services

## Phase 5 — Ecosystem Infrastructure

* [ ] Smart contract
* [ ] Token infrastructure
* [ ] Developer SDK
* [ ] API infrastructure
* [ ] Service integration
* [ ] Ecosystem-wide identity
* [ ] Additional financial infrastructure

---

# Development Status

DesKa Ecosystem masih berada dalam tahap pengembangan aktif.

Beberapa komponen dapat berada pada tahap:

* Architecture
* Protocol design
* Core implementation
* Research
* Testing
* Integration

Protocol, API, storage, consensus, smart contract, networking, dan arsitektur financial services dapat berubah selama proses pengembangan.

Fitur yang belum selesai atau belum stabil tidak dianggap sebagai production infrastructure sampai implementasinya dinyatakan siap.

---

# Vision

DesKa Ecosystem dibangun untuk menggabungkan:

```text
Blockchain
     +
Digital Asset
     +
Financial Services
     +
Rupiah Experience
     +
Developer Infrastructure
```

dengan:

```text
                 ┌───────────────────┐
                 │    DesKa Ecosystem│
                 └─────────┬─────────┘
                           │
                 ┌─────────▼─────────┐
                 │     IndoChain     │
                 │ Blockchain Ledger │
                 └─────────┬─────────┘
                           │
            ┌──────────────┼──────────────┐
            │              │              │
            ▼              ▼              ▼
     IndoChainWallet   DesKaCash      DesKaBank
                                     
                           │
                           ▼
                       DesKaCard
                           │
                           ▼
                        DesKa*
```

**IndoChain** menjadi fondasi blockchain.

**IndoChainWallet** menjadi akses langsung ke blockchain.

**DesKaCash**, **DesKaBank**, **DesKaCard**, dan berbagai layanan **DesKa*** menjadi financial application layer yang memanfaatkan infrastructure tersebut.

Dengan pemisahan ini, DesKa Ecosystem dapat berkembang menjadi kumpulan layanan yang berbeda tanpa harus membangun blockchain terpisah untuk setiap produk.

---

## Projects

| Project             | Peran                              | Layer       |
| ------------------- | ---------------------------------- | ----------- |
| **IndoChain**       | Blockchain dan shared ledger       | Blockchain  |
| **IndoChainWallet** | Native blockchain wallet           | Wallet      |
| **DesKaCash**       | Digital wallet / financial service | Financial   |
| **DesKaBank**       | Banking service                    | Financial   |
| **DesKaCard**       | Card / payment service             | Financial   |
| **DesKa***          | Layanan ecosystem lainnya          | Application |

---

## License

License dan ketentuan penggunaan masing-masing project akan ditentukan sesuai tahap pengembangan dan kebutuhan DesKa Ecosystem.
