# IndoChain Wallet Architecture

> Status: Draft / Wallet Design Baseline

## 1. Purpose

The IndoChain wallet is the primary user-facing client for account creation, key management, transaction construction, signing, submission, and transaction tracking.

Primary UI/client stack: Flutter and Dart.

The wallet must remain independent from node consensus internals.

## 2. Architecture

~~~text
Flutter UI
   ↓
Wallet Application Layer
   ↓
Wallet Core / Protocol Library
   ├── Key Management
   ├── Address
   ├── Transaction Builder
   ├── Signing
   └── Network Client
   ↓
JSON-RPC / WebSocket
   ↓
IndoChain Node
~~~

## 3. Wallet Core

The wallet core should provide key generation/import, account derivation, address encoding/decoding, transaction construction, canonical serialization, signing, signature verification, RPC client, and WebSocket subscriptions.

## 4. Network Profiles

The wallet should support explicit Devnet, Testnet, and Mainnet profiles. Every transaction must be bound to the selected chain/network identity.

## 5. Security Boundary

UI should never directly manipulate raw private keys except through the wallet core/key-management boundary. Sensitive values must not appear in logs, analytics, crash reports, or exported diagnostics.

## 6. Platform

The same Dart wallet core should be reusable across Android, iOS, Windows, macOS, and Linux where supported by Flutter.

## 7. Open Decisions

Mnemonic standard, key type, secure storage implementation, hardware-wallet protocol, multisig UX, and watch-only account behavior remain open.
