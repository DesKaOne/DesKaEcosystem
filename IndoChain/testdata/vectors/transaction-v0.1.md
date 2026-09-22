# Transaction Test Vector v0.1 (Development)

> This fixture documents the current development encoding only. It is not a frozen wire-format vector.

## Input

~~~text
Version: 1
ChainID: 1001
Nonce: 7
Sender: 010203
Recipient: 040506
Value: 42
GasLimit: 21000
Data: 696e646f636861696e
Signature: 090909
~~~

## Required Properties

- Signing bytes exclude the signature field.
- Signed bytes include the signature field.
- Field ordering is deterministic.
- Integer encoding is big-endian in the current development codec.
- Variable byte fields use a 32-bit length prefix in the current development codec.

The exact canonical serialization and hash/domain-separation rules remain subject to the serialization and crypto specifications.