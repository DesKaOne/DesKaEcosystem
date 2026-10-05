# IndoChain Integration Contract v0.1

## Purpose
Define how sibling DesKaEcosystem projects integrate with IndoChain without coupling to node internals.

## Primary Integration Paths
1. JSON-RPC for general application/client access.
2. WebSocket for subscriptions and event streams.
3. Versioned public Go APIs under pkg/ where Go consumers genuinely need them.
4. Protocol test vectors for byte-level interoperability.

## Sibling Projects
- IndoChainWallet: wallet/client integration.
- IndoScan: explorer/indexer integration.
- DesKaCash: application/financial integration.
- DesKaWeb: web/application integration.

## Boundary Rule
Sibling projects must not import IndoChain internal packages.

Consensus-critical compatibility is defined by protocol specifications, canonical serialization, and test vectors rather than by copying implementation details.

## API Stability
Anything under pkg/ is a deliberate public API decision. Until a package is explicitly promoted, consumers should prefer RPC/WebSocket and protocol documentation.

## Future SDK Direction
A versioned IndoChain SDK may be introduced after transaction, address, signing, RPC, and serialization contracts are sufficiently frozen.