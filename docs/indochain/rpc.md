# IndoChain RPC

> Status: Draft / RPC Design Baseline

## 1. Purpose

RPC provides the application-facing interface for wallets, SDKs, explorers, tooling, and operators.

## 2. Transport

Primary direction:

- JSON-RPC over HTTP;
- WebSocket for subscriptions.

Additional gRPC interfaces may be considered for internal/service tooling.

## 3. RPC Categories

Candidate categories:

~~~text
chain
├── block
├── transaction
├── state
├── account
├── network
├── validator
├── consensus
└── gas/fee

submit
├── transaction
└── raw transaction

admin
├── node status
├── peers
└── operational controls
~~~

Administrative APIs must not be exposed publicly by default.

## 4. Read APIs

Read operations should be deterministic views over canonical node state.

Examples:

- get block;
- get transaction;
- get account;
- get balance;
- get nonce;
- get state;
- get network information;
- estimate gas.

## 5. Write APIs

Write APIs may include:

- submit transaction;
- submit raw signed transaction.

RPC must not bypass transaction validation.

## 6. Error Model

Errors should be structured and stable enough for SDK consumers.

At minimum distinguish:

- malformed request;
- unsupported method;
- invalid parameters;
- rejected transaction;
- unavailable data;
- internal node error;
- rate limit.

## 7. Versioning

RPC methods should be versioned or compatibility-managed so protocol upgrades do not silently change semantics.

## 8. Security

Production RPC should support:

- authentication where needed;
- TLS at deployment boundary;
- rate limiting;
- request-size limits;
- method allowlists;
- separate public and admin endpoints.

## 9. Observability

Record metrics for:

- request count;
- latency;
- error rate;
- method usage;
- rate-limit events.

Do not log private keys or sensitive signing material.
