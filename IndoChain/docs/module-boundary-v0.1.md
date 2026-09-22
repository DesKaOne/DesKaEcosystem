# IndoChain Module Boundary v0.1

## Purpose
Define the public/internal boundary of the IndoChain Go module inside the larger DesKaEcosystem repository.

## Module

~~~text
module github.com/DesKaOne/DesKaEcosystem/IndoChain
~~~

## Directory Contract

~~~text
IndoChain/
├── cmd/
├── internal/
├── pkg/
├── genesis/
├── testdata/
└── go.mod
~~~

## Responsibilities

### cmd/
Executable entrypoints only. The node command may assemble internal components, but protocol logic should remain in reusable packages.

### internal/
Consensus-critical implementation and node internals.

Initial areas:
- core/types
- core/transaction
- core/block
- core/state
- crypto
- encoding
- storage
- mempool
- consensus
- p2p
- sync
- vm
- node
- rpc
- config

Code under `internal/` must not be treated as a stable external SDK surface.

### pkg/
Stable, intentionally public Go APIs.

Candidate contents:
- protocol-facing public types
- client interfaces
- SDK helpers
- RPC client types
- interoperability utilities

A package should enter `pkg/` only when its API is intentionally supported for external consumers.

### genesis/
Network profile and genesis configuration artifacts. Genesis creation must remain deterministic and must not depend on runtime secrets or machine-local state.

### testdata/
Protocol fixtures and deterministic interoperability vectors. Fixtures should be usable by Go tests and, where applicable, mirrored by Dart wallet tests.

## Dependency Direction

~~~text
cmd
 ↓
internal/node
 ↓
core / consensus / p2p / storage / vm / rpc
 ↓
core/types + encoding + crypto
~~~

`pkg/` may depend on stable internal implementation through carefully designed public boundaries, but consensus-critical internals must not depend on application projects.

Application projects such as `IndoChainWallet`, `IndoScan`, `DesKaCash`, and `DesKaWeb` integrate with IndoChain through documented APIs/protocols rather than importing node internals.

## Ecosystem Boundary

~~~text
DesKaEcosystem
│
├── IndoChain
│   └── blockchain protocol + node
├── IndoChainWallet
│   └── wallet/client
├── IndoScan
│   └── explorer/indexer
├── DesKaCash
│   └── application/financial layer
└── DesKaWeb
    └── web/application layer
~~~

## Rules

1. Do not import application modules into IndoChain core.
2. Do not expose consensus-critical internals as public SDK APIs without an explicit protocol decision.
3. Do not make the node depend on the explorer, wallet, or web application.
4. Protocol compatibility must be defined by specifications and test vectors.
5. Cross-project integration should prefer RPC, WebSocket, documented protocol types, or a deliberately versioned SDK boundary.
6. Moving a package from `internal/` to `pkg/` is an API decision and requires documentation/tests.