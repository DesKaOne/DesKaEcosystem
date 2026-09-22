# IndoChain Mempool

> Status: Draft / Mempool Design Baseline

## 1. Purpose

The mempool stores transactions that have passed admission checks but have not yet been committed in a block.

## 2. Admission Pipeline

~~~text
Receive Transaction
       ↓
Decode
       ↓
Basic Validation
       ↓
Signature / Chain Validation
       ↓
Nonce / Fee Checks
       ↓
Capacity / Policy Checks
       ↓
Mempool
~~~

## 3. Admission Rules

The mempool should reject:

- malformed transactions;
- wrong chain ID;
- invalid signatures;
- impossible fee parameters;
- stale or invalid nonce states;
- oversized payloads;
- transactions violating local policy.

## 4. Ordering

Candidate ordering factors:

- effective fee;
- priority;
- nonce dependencies;
- transaction age;
- replacement rules.

Consensus must define any ordering that affects canonical execution. Local mempool ordering may differ when block validity does not depend on it.

## 5. Replacement

Replacement policy must prevent cheap repeated replacement attacks.

Potential requirements:

- same sender and nonce;
- sufficient fee increase;
- replacement cooldown/policy;
- bounded replacement history.

Exact values remain open.

## 6. Eviction

Eviction may occur because of:

- expiry;
- block inclusion;
- invalidated nonce;
- capacity pressure;
- insufficient fee;
- chain reorganization.

## 7. Persistence

The default should avoid making mempool contents canonical. Optional persistence may accelerate node restart but must never override canonical chain state.

## 8. DoS Protection

Mempool limits should cover:

- transaction count;
- total bytes;
- per-account count;
- per-peer admission rate;
- validation CPU;
- replacement frequency.

## 9. RPC

RPC should expose safe operations such as:

- submit transaction;
- query transaction status;
- estimate fee/gas where supported.

Administrative mempool controls should be restricted.
