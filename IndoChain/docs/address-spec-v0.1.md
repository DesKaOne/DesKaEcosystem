# IndoChain Address Specification v0.1

## Status

Development specification. Address format is not protocol-frozen.

## Design direction

IndoChain permits compatibility with several address families, including EVM-style, UTXO-style, TVM-style, and Solana-style designs.

The current preferred direction is a human-readable Base58-oriented address.

Candidate display prefixes:

- `IND`
- `iND`
- `dIDR`

No candidate is final yet.

## Logical structure

The address is conceptually:

~~~text
Address
├── Network / Version
├── Address Type
├── Account Identifier / Key Hash
├── Checksum
└── Human-readable Encoding
~~~

The exact byte layout remains TBD.

## Requirements

A final address format should provide:

1. Network separation.
2. Address-type separation.
3. Deterministic encoding and decoding.
4. Corruption detection through checksum.
5. Clear validation rules.
6. Stable Go and Dart interoperability.
7. No ambiguity between account, contract, and other address types.
8. A migration/version mechanism if the format ever changes.

## Compatibility principle

The internal transaction model uses `types.Address`, not a hard-coded string or Base58 type.

This allows the protocol model to remain independent from the human-readable representation.

The final canonical address encoding must be frozen together with the crypto and serialization specifications.

## Current development codec

The development implementation currently supports:

~~~text
Prefix + Base58(Version + Payload + 4-byte Checksum)
~~~

The checksum currently uses double SHA-256.

This is a development implementation only and is not a commitment to the final protocol.

## Open decisions

- Prefix: `IND`, `iND`, `dIDR`, or another value.
- Base58 alphabet.
- Network/version layout.
- Address type encoding.
- Public-key vs key-hash payload.
- Checksum algorithm and length.
- Account/contract address separation.
- Mainnet/testnet/devnet prefixes.
- Future address-version migration rules.
