# IndoChain Gas & Fees

> Status: Draft / Gas Design Baseline

## 1. Purpose

Gas limits computational/resource consumption and provides a fee mechanism for transaction execution.

## 2. Core Concepts

The design references:

- GasLimit;
- GasUsed;
- GasPrice;
- BaseFee;
- PriorityFee.

## 3. Fee Model

Conceptually:

~~~text
Total Fee ≈ GasUsed × Effective Gas Price
~~~

Exact arithmetic, rounding, caps, and fee distribution remain to be specified.

## 4. Base Fee

A dynamic base fee may respond to network congestion.

If adopted, the protocol must define:

- target block capacity;
- adjustment interval;
- adjustment formula;
- minimum/maximum bounds;
- activation rules.

## 5. Priority Fee

A priority component may compensate block proposers/validators for transaction inclusion.

Distribution must be defined by economics.

## 6. Burn

The architecture allows optional fee burning.

If enabled, burn semantics must be deterministic and reflected in state transition rules.

## 7. Gas Schedule

The gas schedule must assign deterministic costs to:

- native transfers;
- signature/verification work where charged;
- state reads;
- state writes;
- contract execution;
- contract deployment;
- storage growth;
- event/log output.

No production gas numbers are frozen in this document.

## 8. Limits

Gas design must constrain:

- transaction execution;
- block execution;
- memory;
- storage growth;
- event output.

## 9. Estimation

RPC may expose gas estimation based on canonical execution rules.

Wallets must treat estimates as estimates and handle changed network state.

## 10. Testing

Required tests include:

- exact fee calculation;
- underpriced transaction;
- out-of-gas execution;
- fee overflow;
- rounding;
- block gas limit;
- base-fee changes;
- fee burn if enabled.
