# IndoChain Transaction Signing

> Status: Draft / Wallet Signing Baseline

## 1. Signing Flow

~~~text
Transaction Intent
      ↓
Canonical Transaction
      ↓
Canonical Signing Bytes
      ↓
Domain / Chain ID
      ↓
Local Signature
      ↓
Signed Transaction
      ↓
RPC Submission
~~~

## 2. Human-Readable Review

Before signing, the wallet should display network, sender, recipient, asset, amount, fee/gas, and contract action where applicable.

## 3. Canonical Bytes

The wallet must sign protocol-defined canonical bytes. UI JSON, display strings, or arbitrary maps must not become the signing format unless explicitly specified by protocol.

## 4. Replay Protection

Signing must include the required chain/network/domain information and nonce semantics.

## 5. Error Handling

Signing should fail safely for wrong network, invalid transaction, missing key, unsupported key type, invalid nonce, invalid fee, and user cancellation.

## 6. Test Vectors

Go and Dart implementations must share transaction bytes, signing bytes, signatures, transaction hashes, invalid-signature cases, and wrong-chain cases.
