# IndoChain Crypto Test Vectors v0.1

## Status

Development interoperability fixture. This does not freeze final protocol cryptography.

## Development implementation

- Signature candidate: Ed25519
- Hash candidate: SHA-256
- Public key: raw 32-byte Ed25519 key
- Signature: raw 64-byte Ed25519 signature
- Address encoding: not frozen
- Domain separation: protocol version + domain + payload

## Address direction

IndoChain remains open to EVM-style, UTXO-style, TVM-style, and Solana-style address conventions.

Current preference is a Base58-oriented human-readable address, with possible prefixes:
- `IND...`
- `iND...`
- `dIDR...`

The exact prefix, checksum, payload layout, network/version byte, and Base58 boundary must be decided before address format freeze.

## Required future fixtures

1. Private-key seed
2. Public key
3. Domain-separated signing bytes
4. Signature
5. Address payload
6. Encoded address
7. Invalid/tampered cases

## Compatibility

Go and Dart must produce byte-identical signing payloads and address encodings after the corresponding protocol components are frozen.