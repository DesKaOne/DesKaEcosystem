# IndoChain Flutter Wallet

> Status: Draft / Flutter Implementation Baseline

## 1. Purpose

Flutter/Dart is the primary wallet application stack for mobile and desktop.

## 2. Layering

~~~text
Flutter UI
├── Screens
├── Widgets
└── Navigation
       ↓
Application State
       ↓
Wallet Core
├── Keys
├── Accounts
├── Address
├── Transactions
├── Signing
└── RPC/WS
       ↓
Platform Services
├── Secure Storage
├── Biometrics
└── Hardware Wallet (future)
~~~

## 3. Network Client

The wallet client should support JSON-RPC, WebSocket, configurable endpoints, request timeouts, safe retries, and chain identity verification.

## 4. UI Requirements

Core screens may include onboarding, wallet creation/import, account list, receive, send, transaction history, transaction details, network selector, and developer/testnet tools.

## 5. Developer Mode

Devnet/Testnet mode may expose faucet access, custom RPC endpoints, network diagnostics, test accounts, and raw transaction inspection. These controls must be clearly separated from Mainnet UX.

## 6. Testing

Required areas include widget tests, wallet-core tests, signing interoperability tests, RPC integration tests, network-switch tests, secure-storage tests, and end-to-end transaction tests.
