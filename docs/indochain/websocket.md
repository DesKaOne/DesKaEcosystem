# IndoChain WebSocket

> Status: Draft / WebSocket Design Baseline

## 1. Purpose

WebSocket provides low-latency event subscriptions for wallets, explorers, trading tools, and developer applications.

## 2. Candidate Events

- new block;
- finalized block;
- transaction submitted;
- transaction included;
- transaction finalized;
- transaction failed;
- account state changed;
- validator status changed.

## 3. Subscription Flow

~~~text
Connect
  ↓
Authenticate / Validate Network
  ↓
Subscribe
  ↓
Receive Events
  ↓
Handle Reconnect
  ↓
Resume / Resubscribe
~~~

## 4. Event Format

Every event should contain enough metadata for consumers to identify:

- event type;
- chain ID;
- protocol/API version;
- block height where applicable;
- transaction ID where applicable;
- event payload.

## 5. Reliability

Clients must assume connections can disconnect.

The server should support a bounded replay/resume mechanism where practical, or expose canonical RPC methods so clients can reconcile missed events.

## 6. Limits

Implement limits for:

- concurrent connections;
- subscriptions per connection;
- event rate;
- message size;
- connection lifetime where appropriate.

## 7. Security

Public WebSocket endpoints require:

- origin/request controls where applicable;
- rate limiting;
- subscription limits;
- authentication for private/admin streams.

Never expose validator private-key operations through WebSocket.
