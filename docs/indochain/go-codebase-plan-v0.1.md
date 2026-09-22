# IndoChain Go Codebase Plan v0.1

> Status: Implementation Plan
> Target branch: `dev/indochain-v0.1`

## 1. Goal

Build the first IndoChain reference node in Go while keeping protocol-critical behavior modular, deterministic, testable, and independent from the Flutter wallet.

## 2. Proposed Repository Layout

~~~text
cmd/
└── indochain/
    └── main.go

internal/
├── core/
│   ├── types/
│   ├── transaction/
│   ├── block/
│   └── state/
├── crypto/
├── encoding/
├── storage/
├── mempool/
├── consensus/
│   ├── pos/
│   ├── bft/
│   └── pow/
├── p2p/
├── sync/
├── vm/
├── node/
├── rpc/
└── config/

pkg/
└── protocol/

genesis/
└── devnet/

testdata/
└── vectors/

docs/
```

## 3. Dependency Direction

Core protocol types should sit below higher-level services.

~~~text
core/types
   ↓
transaction / block / state
   ↓
consensus / mempool / storage / p2p
   ↓
node / rpc
~~~

The VM, wallet integration, and UI must not introduce dependencies into consensus-critical primitive types.

## 4. Initial Packages

### core/types

Primitive protocol types such as hashes, identifiers, heights, nonces, addresses, and protocol versions.

### crypto

Interfaces and implementations for hashing, signing, verification, and key/address operations after the crypto freeze.

### encoding

Canonical serialization and decoding.

### core/transaction

Transaction validation, hashing, signing payload construction, and execution-facing representation.

### core/block

Block/header construction, validation, hashing, and transaction commitments.

### core/state

Deterministic state transition interfaces and state commitments.

### storage

Node storage interfaces. The initial implementation should keep the backend replaceable.

### mempool

Admission, nonce handling, ordering, replacement, eviction, and resource limits.

### consensus

Interfaces shared by PoS, BFT, and optional PoW profiles.

### p2p

Peer lifecycle, transport, framing, gossip, and synchronization transport.

### node

Process lifecycle and orchestration of protocol subsystems.

### rpc

JSON-RPC and WebSocket server interfaces.

## 5. Devnet First

The first executable network should be Devnet.

Devnet must have:

- deterministic genesis fixture;
- development configuration;
- local node startup;
- local RPC endpoint;
- test accounts where appropriate;
- faucet integration only for Devnet/Testnet profiles;
- automated smoke tests.

## 6. Testing Strategy

Every consensus-critical package should have:

- unit tests;
- table-driven tests;
- malformed-input tests;
- deterministic test vectors;
- fuzz tests where appropriate.

Integration tests should eventually start multiple nodes and verify synchronization and canonical-chain agreement.

## 7. Coding Rule

Do not implement frozen-looking behavior that is still marked TBD in the protocol specification. Use interfaces/configuration boundaries until the protocol decision is explicit.

## 8. First Coding Milestone

The first implementation milestone should establish:

1. Go module;
2. protocol primitive types;
3. canonical encoding interface;
4. crypto interface;
5. transaction model;
6. block model;
7. genesis model;
8. deterministic unit tests.

Networking and consensus should be added after these primitives have stable tests.

## 9. Branch Workflow

All implementation work starts from `dev/indochain-v0.1` or a component branch created from it.

Never commit experimental implementation directly to `main`.
