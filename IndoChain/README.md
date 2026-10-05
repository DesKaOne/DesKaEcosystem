# IndoChain

**IndoChain** adalah blockchain yang dikembangkan sebagai bagian utama dari **DesKaEcosystem**.

IndoChain dirancang sebagai infrastruktur blockchain yang dapat digunakan untuk mendukung transaksi digital, aplikasi terdesentralisasi, smart contract, serta berbagai layanan yang membutuhkan jaringan blockchain yang aman, terukur, dan dapat dikembangkan secara modular.

IndoChain juga menjadi fondasi blockchain bagi berbagai komponen dalam **DesKaEcosystem**, termasuk integrasi dengan **DesKaCash** sebagai platform digital wallet dan financial application.

> **IndoChain = Blockchain Infrastructure untuk DesKaEcosystem**

---

## 🎯 Tujuan Proyek

Tujuan utama IndoChain adalah membangun infrastruktur blockchain yang dapat digunakan oleh berbagai aplikasi, service, dan platform dalam DesKaEcosystem.

IndoChain dikembangkan dengan fokus pada:

* Infrastruktur blockchain yang terdesentralisasi.
* Transaksi digital yang aman dan dapat diverifikasi.
* Native asset yang menggunakan denominasi **dIDR**.
* Transaction ledger yang dapat digunakan oleh aplikasi dalam ekosistem.
* Dukungan terhadap smart contract.
* Dukungan terhadap berbagai token dan aset digital.
* Integrasi dengan aplikasi dan service dalam DesKaEcosystem.
* Arsitektur yang dapat dikembangkan secara modular.
* Infrastruktur yang dapat digunakan oleh aplikasi terdesentralisasi atau **dApps**.

---

# 🪙 dIDR — Native Coin IndoChain

IndoChain memiliki native coin bernama:

> **dIDR — denomination Indonesian Rupiah**

dIDR merupakan native asset yang digunakan di dalam jaringan IndoChain.

dIDR menggunakan skala denominasi yang berbeda dari Rupiah fiat untuk mempermudah representasi nilai pada blockchain.

### Denominasi dIDR

| Istilah        |       Nilai |
| -------------- | ----------: |
| **1 dIDR**     | **Rp1.000** |
| **0,1 dIDR**   |       Rp100 |
| **0,01 dIDR**  |        Rp10 |
| **0,001 dIDR** |         Rp1 |

Dengan demikian:

```text
1 dIDR = 1.000 IDR
```

atau:

```text
1 IDR = 0,001 dIDR
```

### Contoh Konversi

```text
Rp1
    ↓
0,001 dIDR

Rp10
    ↓
0,01 dIDR

Rp100
    ↓
0,1 dIDR

Rp1.000
    ↓
1 dIDR

Rp10.000
    ↓
10 dIDR
```

---

# 💰 dIDR dan DesKaCash

Salah satu penggunaan dIDR dalam DesKaEcosystem adalah integrasi dengan **DesKaCash**.

DesKaCash menggunakan representasi Rupiah pada sisi pengguna, sedangkan IndoChain menggunakan dIDR sebagai native asset.

Secara konseptual:

```text
                  DesKaCash
                      │
                      │
                  Rp10.000
                      │
                      ▼
              DesKaCash Backend
                      │
                      │
                  10 dIDR
                      │
                      ▼
                  IndoChain
                      │
                      │
                  10 dIDR
                      │
                      ▼
                 Blockchain
```

Ketika nilai ditampilkan kembali kepada pengguna:

```text
IndoChain
    │
    │ 10 dIDR
    ▼
DesKaCash Backend
    │
    │ × 1.000
    ▼
UI DesKaCash
    │
    ▼
Rp10.000
```

### Representasi Antar Layer

| Layer                 |         Representasi |
| --------------------- | -------------------: |
| **IndoChain**         |             `1 dIDR` |
| **DesKaCash Backend** | `1 dIDR = 1.000 IDR` |
| **Database / Ledger** |             `1 dIDR` |
| **UI DesKaCash**      |          **Rp1.000** |
| **User**              |          **Rp1.000** |

> **Catatan:** `1 dIDR = Rp1.000` merupakan aturan denominasi nilai yang digunakan dalam desain IndoChain dan integrasi DesKaEcosystem.

---

# ⛓️ Blockchain

IndoChain dikembangkan sebagai blockchain yang menyediakan infrastruktur untuk pencatatan dan validasi transaksi.

Komponen blockchain dapat mencakup:

* Block.
* Block Header.
* Transaction.
* Transaction Root.
* State Root.
* Account State.
* Transaction Execution.
* Transaction Validation.
* Mempool.
* Genesis.
* Cryptographic primitives.
* Chain State.
* Blockchain Storage.
* Network Communication.
* Consensus.
* Smart Contract infrastructure.

Arsitektur blockchain dikembangkan secara bertahap sehingga setiap komponen dapat diuji dan dikembangkan secara independen.

---

# 💸 Transaction

Transaction merupakan salah satu komponen utama IndoChain.

Secara konseptual sebuah transaction dapat berisi informasi seperti:

```text
Transaction
├── Version
├── Chain ID
├── Nonce
├── Sender
├── Recipient
├── Value
├── Gas Limit
├── Data
└── Signature
```

Transaction digunakan untuk melakukan perubahan state pada blockchain.

Contoh transaksi native:

```text
Alice
  │
  │ 10 dIDR
  ▼
IndoChain
  │
  ▼
Bob
```

State blockchain kemudian diperbarui berdasarkan hasil eksekusi transaksi.

---

# 🧾 Ledger & State

IndoChain menggunakan konsep **state** untuk merepresentasikan kondisi akun dan aset pada blockchain.

Secara konseptual:

```text
Blockchain
    │
    ▼
Block
    │
    ├── Transactions
    │
    └── State Root
             │
             ▼
          State
             │
       ┌─────┴─────┐
       │           │
     Account     Account
       │           │
    Balance      Balance
       │           │
     Nonce       Nonce
```

State digunakan untuk menentukan kondisi blockchain setelah transaksi dieksekusi.

---

# 🌳 State Root

State Root digunakan sebagai commitment terhadap kondisi state blockchain.

Secara konseptual:

```text
Account State
     │
     ├── Account A
     ├── Account B
     ├── Account C
     └── ...
            │
            ▼
       State Root
            │
            ▼
        Block Header
```

State Root memungkinkan state blockchain direpresentasikan secara deterministic dan dapat diverifikasi.

---

# 🌳 Transactions Root

Transactions Root digunakan sebagai commitment terhadap transaksi yang terdapat dalam sebuah block.

```text
Transactions
     │
     ├── Transaction 1
     ├── Transaction 2
     ├── Transaction 3
     └── ...
            │
            ▼
    Transactions Root
            │
            ▼
       Block Header
```

Dengan demikian block memiliki referensi terhadap kumpulan transaksi yang menjadi bagian dari block tersebut.

---

# 📦 Block

Block merupakan unit utama penyimpanan transaksi dalam blockchain.

Secara konseptual:

```text
Block
├── Header
│   ├── Version
│   ├── Chain ID
│   ├── Height
│   ├── Timestamp
│   ├── Previous Hash
│   ├── Transactions Root
│   ├── State Root
│   ├── Proposer
│   └── Consensus Evidence
│
└── Transactions
    ├── Transaction 1
    ├── Transaction 2
    └── Transaction N
```

Block terhubung dengan block sebelumnya melalui `Previous Hash`.

```text
Genesis
   │
   ▼
Block 1
   │
   ▼
Block 2
   │
   ▼
Block 3
   │
   ▼
Block N
```

---

# 🏁 Genesis

Genesis merupakan state awal jaringan IndoChain.

Genesis mendefinisikan konfigurasi awal blockchain seperti:

* Chain ID.
* Protocol version.
* Network profile.
* Initial timestamp.
* Initial blockchain state.
* Genesis block.
* Genesis hash.

Contoh environment pengembangan:

```text
Network Profile : devnet
Chain ID        : 1001
Protocol        : v1
```

Konfigurasi genesis dapat berbeda berdasarkan network yang digunakan.

---

# 🧠 Mempool

Mempool digunakan untuk menyimpan transaksi yang belum dimasukkan ke dalam block.

Secara konseptual:

```text
User
 │
 │ Transaction
 ▼
Node
 │
 ▼
Mempool
 │
 ├── Transaction A
 ├── Transaction B
 ├── Transaction C
 └── ...
        │
        ▼
   Block Builder
        │
        ▼
      Block
```

Mempool juga dapat digunakan untuk melakukan validasi awal dan pengelolaan transaksi sebelum proses block production.

---

# 🔐 Cryptography

IndoChain menggunakan primitive kriptografi untuk mendukung keamanan transaksi dan identitas akun.

Komponen kriptografi dapat mencakup:

* Digital signature.
* Public key.
* Private key.
* Address generation.
* Hashing.
* Encoding.
* Cryptographic verification.

Implementasi kriptografi harus menghasilkan output yang deterministic sehingga dapat diverifikasi oleh node lain dalam jaringan.

---

# 📜 Smart Contract

IndoChain dirancang untuk mendukung **Smart Contract**.

Smart Contract memungkinkan developer membangun aplikasi dan logic yang berjalan pada blockchain.

Contoh penggunaan:

* Token.
* Digital asset.
* DeFi application.
* Decentralized application.
* Automated transaction logic.
* Ecosystem services.

Arsitektur Smart Contract akan dikembangkan secara bertahap mengikuti perkembangan protocol IndoChain.

---

# 🪙 Token

Selain native coin **dIDR**, IndoChain juga dirancang untuk mendukung berbagai token dan aset digital.

Secara konseptual:

```text
IndoChain
    │
    ├── Native Asset
    │      └── dIDR
    │
    └── Smart Contract Assets
           ├── Token A
           ├── Token B
           └── Token N
```

Native dIDR merupakan bagian dari protocol blockchain, sedangkan token lain dapat dibangun menggunakan infrastructure Smart Contract yang tersedia.

---

# 🌐 DesKaEcosystem

IndoChain merupakan salah satu komponen fundamental dalam **DesKaEcosystem**.

Secara konseptual:

```text
                    DesKaEcosystem
                          │
            ┌─────────────┼─────────────┐
            │             │             │
            ▼             ▼             ▼
        IndoChain     DesKaCash      Other Apps
            │             │             │
            │             │             │
            └─────────────┼─────────────┘
                          │
                    Ecosystem API
```

IndoChain berfungsi sebagai blockchain infrastructure, sementara aplikasi lain dapat menggunakan service dan infrastructure blockchain sesuai kebutuhan.

---

# 💳 DesKaCash Integration

DesKaCash merupakan salah satu aplikasi yang dirancang untuk terintegrasi dengan IndoChain.

Hubungan keduanya:

```text
                    DesKaEcosystem
                          │
             ┌────────────┴────────────┐
             │                         │
             ▼                         ▼
         IndoChain                 DesKaCash
             │                         │
         Native dIDR              Financial App
             │                         │
             └────────────┬────────────┘
                          │
                   Transaction Layer
```

DesKaCash dapat menggunakan IndoChain sebagai bagian dari infrastructure ledger dan transaction processing.

---

# 🏗️ Architecture

Arsitektur IndoChain dikembangkan secara modular.

Secara konseptual:

```text
                         IndoChain
                            │
        ┌───────────────────┼───────────────────┐
        │                   │                   │
        ▼                   ▼                   ▼
     Network             Consensus           Execution
        │                   │                   │
        │                   │           ┌───────┴────────┐
        │                   │           │                │
        ▼                   ▼           ▼                ▼
      P2P              Validators   Transaction       State
                                      Execution       Management
                                           │
                                           ▼
                                       Blockchain
                                          State
```

Komponen dapat dikembangkan secara independen selama tetap mengikuti protocol dan deterministic execution rules IndoChain.

---

# 🛠️ Development

IndoChain saat ini dikembangkan secara bertahap.

Development difokuskan pada pembangunan fundamental blockchain terlebih dahulu sebelum masuk ke fitur tingkat lanjut.

Tahapan pengembangan dapat mencakup:

```text
Core Types
    │
    ▼
Transaction
    │
    ▼
State
    │
    ▼
Execution
    │
    ▼
Block
    │
    ▼
Genesis
    │
    ▼
Mempool
    │
    ▼
Blockchain Storage
    │
    ▼
Node
    │
    ▼
Networking
    │
    ▼
Consensus
    │
    ▼
Smart Contract
    │
    ▼
Production Network
```

---

# 🗺️ Roadmap

Roadmap IndoChain akan dikembangkan secara bertahap.

### Phase 1 — Blockchain Core

* Core types.
* Transaction.
* Account state.
* State transition.
* Block.
* Block hashing.
* State root.
* Transactions root.
* Genesis.
* Cryptographic primitives.
* Mempool.

### Phase 2 — Blockchain Node

* Persistent blockchain storage.
* Chain management.
* Node lifecycle.
* Block synchronization.
* Transaction pool management.
* RPC/API.

### Phase 3 — Network

* Peer-to-peer networking.
* Peer discovery.
* Block propagation.
* Transaction propagation.
* Network synchronization.

### Phase 4 — Consensus

* Consensus protocol.
* Block production.
* Validator infrastructure.
* Consensus verification.
* Finality mechanism.

### Phase 5 — Smart Contract

* Smart contract execution.
* Contract state.
* Contract deployment.
* Contract invocation.
* Token infrastructure.

### Phase 6 — DesKaEcosystem Integration

* DesKaCash integration.
* Ecosystem API.
* Financial transaction integration.
* Cross-service communication.
* Developer infrastructure.

---

# ⚠️ Development Status

> **IndoChain saat ini masih dalam tahap pengembangan.**

Protocol, architecture, transaction format, consensus, smart contract infrastructure, storage, networking, dan komponen lainnya dapat berubah selama proses pengembangan.

Fitur yang tercantum dalam README ini tidak selalu berarti seluruhnya telah tersedia atau aktif pada versi saat ini.

Dokumentasi akan diperbarui mengikuti perkembangan implementasi IndoChain.

---

# 📚 Project Structure

Struktur project IndoChain dikembangkan secara modular.

Contoh struktur:

```text
IndoChain/
├── cmd/
│   └── indochain/
│
├── internal/
│   ├── core/
│   │   ├── block/
│   │   ├── state/
│   │   ├── transaction/
│   │   └── types/
│   │
│   ├── crypto/
│   ├── encoding/
│   └── mempool/
│
├── genesis/
│   └── devnet/
│
├── pkg/
│   └── protocol/
│
├── docs/
│
└── testdata/
```

Struktur tersebut dapat berubah seiring perkembangan protocol dan kebutuhan implementasi.

---

# 🌏 DesKaEcosystem

IndoChain dikembangkan sebagai bagian dari visi **DesKaEcosystem**, yaitu ekosistem teknologi yang menghubungkan blockchain, aplikasi, service, API, dan berbagai infrastructure dalam satu platform.

Komponen utama ekosistem meliputi:

* **IndoChain** — Blockchain & native dIDR infrastructure.
* **DesKaCash** — Digital Wallet & Financial Platform.
* **DesKaEcosystem API** — Integrasi antar aplikasi dan service.
* **Financial Integration Layer** — Integrasi dengan financial provider.
* **Merchant Platform** — Infrastructure untuk merchant dan bisnis.

---

# 📄 License

License IndoChain akan ditentukan sesuai kebijakan dan tahap pengembangan **DesKaEcosystem**.
