# IndoChain Virtual Machine

> Status: Draft / VM Design Baseline

## 1. Direction

The architecture keeps EVM and WASM as candidate execution environments.

The VM choice is intentionally not frozen until protocol requirements and interoperability tests are complete.

## 2. VM Responsibilities

The execution layer must provide:

- deterministic execution;
- gas accounting;
- state access;
- transaction context;
- contract deployment;
- contract invocation;
- execution receipts/results;
- failure/revert semantics.

## 3. VM Boundary

Conceptually:

~~~text
Transaction
    ↓
Execution Context
    ↓
VM
    ↓
State Interface
    ↓
State Changes / Revert
~~~

The VM must not directly control the canonical database.

## 4. Determinism

VM execution must not depend on local:

- filesystem;
- environment variables;
- wall-clock time outside protocol context;
- network access;
- random values outside deterministic protocol mechanisms.

## 5. Gas

Every execution path that consumes bounded computational resources should have deterministic gas accounting.

Gas schedule belongs in gas.md.

## 6. EVM Candidate

EVM would provide compatibility with established Ethereum-oriented tooling, wallets, and contract ecosystems.

Compatibility requirements would need explicit specification rather than assuming byte-for-byte equivalence.

## 7. WASM Candidate

WASM could provide a flexible runtime boundary and support multiple languages/toolchains.

A WASM profile would require strict deterministic execution rules and a defined host-function ABI.

## 8. Host Functions

Host functions should expose only protocol-approved operations such as:

- state reads/writes;
- caller/context information;
- block metadata;
- cryptographic primitives where required.

External network calls must not be part of canonical execution.

## 9. Receipts

Contract execution should produce deterministic receipts containing enough information for RPC and explorer consumers.

## 10. Open Decisions

- EVM vs WASM;
- bytecode format;
- host API;
- gas schedule;
- precompiles;
- contract storage model;
- execution limits;
- upgrade/versioning model.
