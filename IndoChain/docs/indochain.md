# IndoChain

IndoChain adalah rancangan blockchain utama dalam DesKa Ecosystem.

Dokumen ini menjadi baseline arsitektur dan roadmap fitur IndoChain. Implementasi blockchain menggunakan **Go (Golang) sebagai bahasa utama**, sedangkan aplikasi wallet mobile/desktop menggunakan **Flutter (Dart)**.

> Status: Architecture & Roadmap Draft  
> Repository: `DesKaOne/DesKaEcosystem`

## 🧭 Technology Direction

### Core Blockchain
- **Language:** Go
- **Runtime:** Go CLI / node daemon
- **Target:** Linux amd64/arm64/armv7 dan platform lain yang relevan
- **Architecture:** Modular blockchain node
- **State model:** Account Model
- **Consensus direction:** PoS + BFT with modular consensus engine
- **Smart contract direction:** EVM and/or WASM, finalized during implementation
- **Internal storage:** Embedded KV database untuk chain/state data
- **External/indexing database:** PostgreSQL bila diperlukan untuk service/indexing layer

### Wallet
- **Framework:** Flutter
- **Language:** Dart
- **Targets:** Android, iOS, Windows, Linux, macOS
- **Features:** wallet, transactions, token/NFT, staking, swap, QR, history dan ecosystem integrations

---

# 🧱 1. Core Blockchain

Fondasi wajib:
- ⛓️ Block structure
- 🔐 Cryptographic hash — SHA-256/Keccak
- 🔏 Digital signature — secp256k1/Ed25519
- 🧮 Transaction
- 🧾 Merkle Tree
- 🔢 Nonce & block difficulty/consensus parameters
- ⏱️ Block timestamp
- 🔗 Previous block hash
- 🧱 Genesis block
- 🛡️ Chain validation
- 🔄 Fork detection
- 🧹 Reorganization / chain reorg

Contoh struktur:

```text
Block
 ├── Height
 ├── Timestamp
 ├── PreviousHash
 ├── MerkleRoot
 ├── StateRoot
 ├── TransactionsRoot
 ├── Validator
 ├── Signature
 └── Transactions[]
```

---

# ⚡ 2. Consensus

Consensus dibuat modular:

```text
ConsensusEngine
 ├── Proof of Work
 ├── Proof of Stake
 ├── Delegated Proof of Stake
 ├── Proof of Authority
 └── BFT / PBFT
```

Arah utama IndoChain:

```text
PoS
 +
BFT
 +
Validator Set
```

Fitur:
- Validator registration
- Staking
- Delegation
- Slashing
- Reward
- Validator rotation
- Epoch
- Voting
- Finality
- Double-sign protection

---

# 🌐 3. P2P Network

Minimal:

```text
Node
 │
 ├── Peer discovery
 ├── TCP/QUIC
 ├── Handshake
 ├── Peer authentication
 ├── Message protocol
 ├── Block propagation
 ├── Transaction propagation
 └── Peer reputation
```

Fitur lanjut:
- DHT
- Peer scoring
- Connection limits
- Ban list
- Rate limiting
- Gossip protocol
- Peer synchronization
- Fast sync
- Snapshot sync
- State sync

---

# 💰 4. Native Coin

Aset native utama rancangan:

```text
dIDR
```

dIDR adalah nama rancangan native asset yang terinspirasi oleh denominasi Rupiah.

Digunakan untuk:
- Transaction fee
- Staking
- Validator reward
- Governance
- Smart contract execution
- Storage/resource fee

Supply tracking:

```text
Total Supply
Circulating Supply
Locked Supply
Staked Supply
Burned Supply
```

> Tokenomics, monetary policy, initial supply dan mekanisme emisi harus ditentukan dalam desain ekonomi/protokol tersendiri sebelum mainnet.

---

# ⛽ 5. Gas & Fee System

Parameter:

```text
GasLimit
GasUsed
GasPrice
BaseFee
PriorityFee
```

Model:

```text
Transaction Fee =
Base Fee
+
Priority Fee
```

Fitur:
- Dynamic fee
- Congestion pricing
- Fee burning
- Minimum gas price
- Fee market

---

# 👛 6. Wallet System

Fitur:
- HD wallet
- Mnemonic
- Private key
- Public key
- Address (base58)
- Multi-account
- Multi-signature
- Hardware wallet support
- Watch-only wallet

Format address rancangan:

```text
iND1xxxxxxxxxxxxxxxx
```

Format `0x...` dapat dipertimbangkan bila EVM dipilih.

---

# 📦 7. State Model — Account Model

IndoChain menggunakan **Account Model** sebagai arah utama karena target ecosystem mencakup smart contract dan DeFi.

```text
Address
 ├── Balance
 ├── Nonce
 └── Storage
```

Kegunaan:
- Smart contract
- DeFi
- Token
- EVM-like ecosystem

UTXO dipertimbangkan sebagai referensi desain, bukan model state utama.

---

# 🧠 8. Smart Contract

Execution environment:

```text
SmartContractVM
```

Pilihan:
- WASM
- EVM
- Custom VM

Arah desain:
- **EVM** untuk kompatibilitas tooling Ethereum
- **WASM** untuk fleksibilitas runtime dan bahasa
- Pilihan final ditentukan sebelum fase implementasi VM

Fitur:
- Contract deployment
- Contract execution
- Contract storage
- Gas metering
- Contract events
- ABI
- Contract upgrade
- Contract permission

---

# 🪙 9. Token System

Token menjadi asset di atas IndoChain.

Rancangan standard:
```text
IND-20
IND-721
IND-1155
```

Fungible:
```text
dIDR
USDT
USDC
```

NFT:
```text
NFT
 ├── Token ID
 ├── Owner
 ├── Metadata
 └── URI
```

---

# 🔄 10. DEX

```text
dIDR
   ↓
DEX
   ↓
Liquidity Pool
   ↓
Token Swap
```

Fitur:
- AMM
- Liquidity pool
- Swap
- LP token
- Fee
- Slippage
- Routing
- Price oracle

Contoh pair:
```text
dIDR/USDT
dIDR/USDC
dIDR/BTC
```

---

# 🏦 11. DeFi

- Lending
- Borrowing
- Staking
- Farming
- Liquidity mining
- Vault
- Stablecoin
- Yield system

---

# 🗳️ 12. Governance

```text
Governance
 ├── Proposal
 ├── Voting
 ├── Quorum
 ├── Voting Power
 └── Execution
```

Contoh:
```text
Proposal #001

Increase block gas limit

YES
NO
ABSTAIN
```

---

# 🔐 13. Security Layer

Security adalah requirement inti.

Fitur:
- Replay protection
- Double-spend protection
- Reentrancy protection
- Signature verification
- Integer overflow protection
- Rate limiting
- Peer authentication
- Sybil resistance
- DDoS protection
- Slashing
- Fraud detection

Quality & security engineering:
```text
Security Audit
+
Fuzz Testing
+
Property Testing
+
Chaos Testing
```

---

# 📊 14. Blockchain Explorer

Menampilkan:
```text
Blocks
Transactions
Addresses
Tokens
Contracts
Validators
Staking
```

Search:
```text
Block Hash
Tx Hash
Address
Contract
Token
```

Domain rancangan:
```text
explorer.indochain.network
```

---

# 🔌 15. RPC API

```text
JSON-RPC
REST API
WebSocket
gRPC
```

Contoh bila EVM-compatible:
```text
eth_blockNumber
eth_getBalance
eth_sendRawTransaction
eth_getTransactionReceipt
```

API native IndoChain juga dapat menyediakan namespace khusus protocol.

---

# 📡 16. WebSocket

Realtime events:
```text
newBlock
newTransaction
pendingTransaction
contractEvent
validatorUpdate
```

Contoh:
```text
WebSocket
    ↓
Flutter Wallet
    ↓
Realtime wallet balance
```

---

# 🗄️ 17. State Database

Penyimpanan:
```text
Block Store
State Store
Transaction Index
Receipt Store
```

Kandidat embedded database:
```text
RocksDB
Pebble
Badger
LevelDB
```

Prinsip:
- Embedded KV database untuk hot state dan canonical chain storage
- PostgreSQL untuk indexer, explorer, analytics dan service eksternal bila diperlukan
- PostgreSQL bukan canonical state store node secara default

---

# 🚀 18. Fast Sync

Node baru tidak harus mengulang seluruh proses dari genesis.

```text
Snapshot
  ↓
Verify State Root
  ↓
Continue Sync
```

---

# 📸 19. Snapshot

```text
snapshot-1000000.tar.zst
```

Isi:
```text
State
Accounts
Contracts
Validators
```

Restore:
```text
Download snapshot
→ Verify hash
→ Restore
→ Sync recent blocks
```

---

# 🌉 20. Bridge

```text
IndoChain
     ↕
Ethereum
     ↕
BSC
     ↕
Polygon
```

Fitur:
- Lock
- Mint
- Burn
- Release
- Cross-chain message

Bridge termasuk infrastruktur high-risk dan membutuhkan threat model, audit, monitoring, pause controls dan recovery procedures sebelum digunakan dengan aset bernilai.

---

# 🧩 21. Oracle

Contoh data:
```text
BTC/USD
ETH/USD
USD/IDR
Gold
Weather
API data
```

Arsitektur:
```text
Oracle Providers
       ↓
Oracle Aggregator
       ↓
Blockchain
       ↓
Smart Contract
```

---

# 🪪 22. Identity

```text
did:ind:xxxxx
```

Alur:
```text
DID
 ↓
Credential
 ↓
Verification
```

---

# 📁 23. Decentralized Storage

```text
Blockchain
     ↓
CID
     ↓
IPFS / Storage Network
```

NFT example:
```text
Token #100

metadata:
{
  "name": "Indo NFT",
  "image": "ipfs://..."
}
```

---

# 🧪 24. Developer Tools

CLI utama:
```text
indochain-cli
```

Contoh:
```bash
indochain wallet create
indochain wallet balance
indochain transaction send
indochain node start
indochain node sync
indochain validator stake
indochain contract deploy
```

SDK:
```text
indochain-sdk-go
indochain-sdk-dart
indochain-sdk-python
indochain-sdk-js
```

Go menjadi SDK/tooling prioritas internal; Dart menjadi prioritas integrasi Flutter.

---

# 📱 25. Mobile/Desktop Wallet

Implementasi:

```text
Flutter + Dart
```

Target:
- Android
- iOS
- Windows
- Linux
- macOS

Fitur:
- Send
- Receive
- QR
- Token
- NFT
- Staking
- Swap
- Transaction history
- WalletConnect-like protocol

---

# 🖥️ 26. Node Dashboard

Dashboard:
```text
IndoChain Node
```

Monitoring:
```text
Block Height
Peers
TPS
Memory
CPU
Disk
Mempool
Validator
Sync %
```

---

# 📈 27. Mempool

Alur:
```text
Wallet
  ↓
RPC
  ↓
Mempool
  ↓
Validator
  ↓
Block
```

Mempool:
- Priority queue
- Fee sorting
- Nonce management
- Duplicate detection
- Expiration
- Replacement transaction

---

# ⚡ 28. Parallel Transaction Processing

Target:

```text
Transaction
Transaction
Transaction
Transaction
      ↓
Dependency Analysis
      ↓
Parallel Execution
```

Transaksi tanpa konflik state dapat dieksekusi paralel.

---

# 🧮 29. MEV Protection

Untuk ecosystem DEX:
- Transaction ordering rules
- Commit-reveal
- Private transaction
- Batch auction
- MEV-aware validator logic

---

# 🔥 30. Burn Mechanism

```text
Transaction Fee
      ↓
Base Fee
      ↓
Burn
```

Burn policy harus menjadi bagian dari tokenomics dan governance; burn bukan jaminan kenaikan nilai aset.

---

# 🏗️ 31. Modular Architecture

Struktur rancangan:

```text
indochain/
│
├── core/
│   ├── block
│   ├── transaction
│   ├── state
│   └── crypto
│
├── consensus/
│   ├── pos
│   ├── pow
│   └── bft
│
├── network/
│   ├── p2p
│   ├── gossip
│   └── discovery
│
├── mempool/
│
├── vm/
│
├── storage/
│
├── rpc/
│
├── wallet/
│
├── validator/
│
├── governance/
│
└── explorer/
```

Core protocol harus independen dari wallet, explorer dan service bisnis.

---

# 🧠 32. Governance Upgrade

```text
Proposal
  ↓
Vote
  ↓
Timelock
  ↓
Activation
```

Contoh:
```text
Protocol v1
      ↓
Governance
      ↓
Protocol v2
```

---

# 🌍 33. Multi-Network

```text
IndoChain
├── Mainnet
├── Testnet
├── Devnet
└── Localnet
```

Chain ID rancangan:
```text
indochain-mainnet = 1001
indochain-testnet = 1002
indochain-devnet  = 1003
```

Nilai chain ID dapat berubah sebelum mainnet.

---

# 🧰 34. Observability

```text
Prometheus
Grafana
OpenTelemetry
Structured Logging
Metrics
Tracing
```

Metrics:
```text
block_time
tx_count
tps
gas_used
peer_count
mempool_size
validator_uptime
state_size
```

---

# 🤖 35. Automation

```text
Node Watchdog
Validator Watchdog
Auto Restart
Auto Backup
Auto Snapshot
Auto Update
Health Check
```

Contoh:
```text
Node DOWN
  ↓
Watchdog
  ↓
Restart
  ↓
Resync
  ↓
Healthy
```

---

# 🧬 Arsitektur Besar

```text
                    ┌─────────────────────┐
                    │   IndoChain Wallet  │
                    │   Flutter / Dart    │
                    └──────────┬──────────┘
                               │
                         JSON-RPC / WS
                               │
                    ┌──────────▼──────────┐
                    │      RPC Node       │
                    │        Go           │
                    └──────────┬──────────┘
                               │
              ┌────────────────▼────────────────┐
              │             Mempool              │
              └────────────────┬────────────────┘
                               │
                    ┌──────────▼──────────┐
                    │     Consensus       │
                    │      PoS + BFT      │
                    └──────────┬──────────┘
                               │
                    ┌──────────▼──────────┐
                    │     Execution VM    │
                    │    EVM / WASM       │
                    └──────────┬──────────┘
                               │
              ┌────────────────▼────────────────┐
              │           State DB              │
              └────────────────┬────────────────┘
                               │
                    ┌──────────▼──────────┐
                    │     Block Store     │
                    └─────────────────────┘

       ↕ P2P / Gossip / DHT / Peer Discovery

 ┌─────────────┐ ┌─────────────┐ ┌─────────────┐
 │ Validator 1 │ │ Validator 2 │ │ Validator N │
 └─────────────┘ └─────────────┘ └─────────────┘
```

---

# 🔥 Roadmap

## Phase 1 — Core
`Block → Transaction → Crypto → State → Chain`

## Phase 2 — P2P
`Peer → Gossip → Sync → Mempool`

## Phase 3 — Consensus
`Validator → PoS → BFT → Finality`

## Phase 4 — Economy
`Native Coin → Gas → Staking → Reward → Slashing`

## Phase 5 — Smart Contract
`VM → Contract → Token → NFT`

## Phase 6 — Ecosystem
`RPC → SDK → Wallet → Explorer`

## Phase 7 — DeFi
`DEX → Oracle → Lending → Farming`

## Phase 8 — Advanced
`Parallel Execution → MEV Protection → ZK → Cross-chain`

## Phase 9 — Production
`Monitoring → Snapshot → Fast Sync → Backup → Security Audit`

---

# 🚀 Official Technology Stack Direction

| Layer | Technology |
|---|---|
| Blockchain Core | **Go** |
| Node / CLI | **Go** |
| Consensus | Go |
| P2P | Go |
| Execution VM | Go integration / selected VM |
| State / Block DB | Embedded KV DB |
| RPC | Go |
| WebSocket | Go |
| SDK Primary | Go |
| SDK Mobile | Dart |
| Wallet | **Flutter / Dart** |
| Explorer | Flutter Web / Next.js |
| Indexer / Analytics | PostgreSQL where appropriate |
| Infrastructure | Docker, Nginx, Cloudflare, Tailscale, VPS |
| Testing | Unit, Integration, Fuzz, Property, Chaos |

---

# 🎯 Design Principles

1. **Go adalah bahasa utama protocol dan node IndoChain.**
2. **Flutter/Dart adalah stack utama wallet mobile/desktop.**
3. Core blockchain tidak boleh bergantung pada wallet atau UI.
4. Consensus, P2P, storage dan VM harus modular.
5. Canonical blockchain state disimpan pada storage node khusus; PostgreSQL digunakan untuk indexing/service layer bila diperlukan.
6. Semua parameter protocol penting harus deterministic dan dapat diverifikasi semua node.
7. Security dan testability diprioritaskan sejak awal.
8. Mainnet hanya setelah testnet, tooling, observability, snapshot/sync strategy, backup dan security review memadai.
9. DEX, bridge, DeFi, oracle dan governance dikembangkan di atas core protocol yang stabil.

---

## 📝 Status

Dokumen ini adalah **master roadmap dan architecture baseline** untuk IndoChain di dalam DesKa Ecosystem.

Detail seperti final consensus implementation, VM, tokenomics, cryptographic suite, wire protocol, block timing, gas schedule dan genesis allocation akan ditetapkan pada dokumen desain teknis masing-masing sebelum implementasi production.


---

# EVM Execution & Developer Ecosystem

IndoChain tetap merupakan blockchain native milik sendiri. EVM digunakan sebagai execution layer agar developer dapat menggunakan ekosistem smart contract yang sudah dikenal, tanpa menjadikan IndoChain sebagai Ethereum clone.

```text
                    IndoChain
                        │
        ┌───────────────┼────────────────┐
        │               │                │
   Consensus Layer  Native Asset     EVM Layer
        │               │                │
      PoS+BFT           dIDR         Solidity
      Validator         Native       Smart Contract
      P2P/Block         Asset         ABI
                                      JSON-RPC
                                      Tooling
                        │                │
                        └───────┬────────┘
                                │
                         Fee Sponsorship
```

## Layering

### Native Blockchain Layer

Fondasi IndoChain tetap mencakup consensus, validator, P2P, block production, mempool, state, storage, dan finality. Core protocol menggunakan Go sebagai bahasa utama.

### Native Asset Layer

`dIDR` adalah native asset IndoChain. Detail tokenomics, monetary policy, supply, emission, dan genesis allocation ditetapkan pada spesifikasi ekonomi/protocol tersendiri sebelum mainnet.

### EVM Execution Layer

Target developer workflow:

```text
Solidity
   ↓
Hardhat / Foundry / Remix
   ↓
IndoChain JSON-RPC
   ↓
EVM Execution
   ↓
IndoChain State
```

Compatibility yang ditargetkan mencakup Solidity, EVM bytecode, ABI, JSON-RPC, contract deployment/execution, events, gas metering, dan tooling EVM yang relevan.

**EVM adalah execution layer IndoChain, bukan pengganti native blockchain layer.**

## Native Account dan EVM Account

Secara konseptual wallet dapat mengelola:

```text
IndoChain User
│
├── Native Account
│      └── native dIDR balance
│
└── EVM Account
       └── smart contract / token interaction
```

Binding identity/user, native account, dan EVM account harus ditentukan pada spesifikasi account/address. Format address final belum dianggap production specification.

## Fee Sponsorship

Fee sponsorship dirancang sebagai fitur native IndoChain yang dapat berlaku untuk transaksi native maupun EVM.

```text
                 Transaction
                      │
              ┌───────┴───────┐
              │               │
          Native Tx        EVM Tx
              │               │
              └───────┬───────┘
                      │
                 Fee System
                      │
              ┌───────┴───────┐
              │               │
          User Pays       Sponsored
                              │
                        Fee Sponsor
```

Desain ini tidak dikunci pada ERC-4337/Paymaster. Karena IndoChain memiliki protocol sendiri, sponsorship dapat diimplementasikan langsung pada transaction format, validation rules, dan execution pipeline.

Konsep transaction:

```text
Transaction
├── from
├── to
├── amount
├── nonce
├── fee
├── signature
└── optional fee_payer / sponsor
```

Authorization sponsor, signature, limits, replay protection, refund, dan anti-abuse ditetapkan pada spesifikasi fee sponsorship.

## DesKaCash Integration

```text
                         DesKaEcosystem
                               │
              ┌────────────────┴────────────────┐
              │                                 │
        DesKaCash                           IndoChain
        Fiat Ledger                       Blockchain
              │                                 │
       Top Up / PPOB / P2P                  dIDR
              │                                 │
              └──────────────┬──────────────────┘
                             │
                         Conversion
```

PostgreSQL ledger DesKaCash menjadi source of truth untuk saldo fiat DesKaCash, sedangkan IndoChain menjadi source of truth untuk state dan saldo on-chain. DesKaCash cukup menyimpan mapping seperti `user_id`, `blockchain_address`, `chain_id`, wallet type, dan status; saldo blockchain tidak perlu diduplikasi sebagai canonical balance.

Konversi fiat IDR ↔ dIDR merupakan settlement/conversion process tersendiri.

## Developer Ecosystem

Target IndoChain adalah memungkinkan developer membawa skill dan tooling EVM yang sudah mereka miliki:

```text
Developer
   ↓
Solidity
   ↓
Hardhat / Foundry / Remix
   ↓
IndoChain RPC
   ↓
Deploy / Call / Transaction
   ↓
IndoChain EVM
```

Tooling native yang direncanakan:

```text
indochain-cli
indochain-sdk-go
indochain-sdk-dart
indochain-sdk-python
indochain-sdk-js
```

EVM compatibility menjadi developer-facing boundary, sedangkan core blockchain tetap modular dan dapat berkembang secara independen.

## Architectural Principles

1. IndoChain tetap blockchain native milik sendiri.
2. EVM menjadi execution layer untuk smart contract compatibility.
3. Consensus, P2P, native state, dan block layer tidak bergantung pada application tooling EVM.
4. dIDR merupakan native asset IndoChain.
5. Fee sponsorship dirancang pada level protocol IndoChain.
6. DesKaCash bukan canonical source untuk blockchain state.
7. EVM interfaces diperlakukan sebagai compatibility boundary untuk developer.
8. Native protocol dan EVM execution memiliki batas tanggung jawab yang jelas.
9. Detail yang belum ditetapkan secara teknis tidak dianggap sebagai production specification.
