# IndoChain Address Test Vectors v0.1

## Status

Development fixture. Values are intended to validate the codec implementation, not freeze the final address protocol.

## Coverage

Each implementation should eventually cover:

- raw payload
- version
- network
- address type
- checksum
- encoded address
- decode result
- invalid checksum
- invalid prefix
- invalid version
- invalid Base58 characters

## Interoperability

Go and Flutter/Dart implementations must produce identical bytes and encoded strings after the address format is frozen.

## Current development shape

~~~text
prefix + Base58(version + payload + checksum)
~~~

The current development codec uses:

- prefix: configurable
- version: configurable one-byte value
- payload: arbitrary bytes
- checksum: first 4 bytes of double SHA-256
