# IndoChain Wallet Key Management

> Status: Draft / Key Management Baseline

## 1. Purpose

Key management protects user-controlled signing material.

## 2. Key Lifecycle

~~~text
Generate / Import
      ↓
Secure Storage
      ↓
Use for Signing
      ↓
Backup / Recovery
      ↓
Rotate / Remove
~~~

## 3. Private Keys

Private keys must remain local by default, never be sent to RPC, never be included in transaction submission, never be written to normal logs, never be embedded in source code, and be protected by platform secure storage where available.

## 4. Mnemonic

A mnemonic-based recovery system may be supported. If adopted, the specification must define the mnemonic standard, derivation path, passphrase behavior, account index, and network/key-type separation.

## 5. Derivation

The wallet should support deterministic account derivation if a hierarchical key scheme is selected. Derivation paths must be documented and tested across Go and Dart implementations.

## 6. Backup

Backup UX must clearly distinguish recovery material from ordinary wallet metadata.

## 7. Hardware Wallet

Hardware-wallet integration remains optional. The signing protocol must preserve canonical transaction bytes and domain separation.

## 8. Multisignature

Multisig support may be added at the protocol or contract layer. The exact authorization model is not frozen.

## 9. Testing

Test corrupted key material, wrong derivation path, wrong network, malformed mnemonic, signing cancellation, secure-storage failure, backup/restore, and duplicate account derivation.
