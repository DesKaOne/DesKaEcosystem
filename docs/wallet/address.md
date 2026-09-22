# IndoChain Wallet Addresses

> Status: Draft / Address Design Baseline

## 1. Current Direction

The roadmap references an IndoChain address beginning with `iND1...` and a Base58-oriented address representation. This is a design direction, not a frozen encoding specification.

## 2. Address Components

The final format should define version, network, key/account type, payload, checksum, and canonical text encoding.

## 3. Network Separation

Devnet, Testnet, and Mainnet addresses must not be ambiguous. Wallet and node implementations should reject wrong-network addresses where the protocol requires network-specific encoding.

## 4. EVM Compatibility

If EVM support is enabled, a `0x...` representation may coexist with native IndoChain addresses. The mapping must be deterministic and explicitly specified.

## 5. Validation

Reject malformed encoding, invalid checksum, unsupported version, invalid payload length, and wrong network where applicable.

## 6. Test Vectors

Provide valid Devnet, Testnet, and Mainnet addresses plus invalid checksum, invalid version, malformed payload, and cross-network rejection cases.
