# IndoChain Development Workflow v0.1

> Status: Development Policy

## 1. Branch Protection Convention

`main` is the stable integration branch.

Implementation must happen on development branches.

## 2. Branch Naming

Use descriptive prefixes:

~~~text
feature/indochain-<component>
dev/indochain-v0.1
fix/indochain-<issue>
refactor/indochain-<component>
test/indochain-<scope>
~~~

## 3. Pull Requests

Changes should enter `main` through a pull request after tests and review.

A pull request should explain:

- what changed;
- why it changed;
- protocol implications;
- tests executed;
- compatibility impact;
- whether documentation/test vectors changed.

## 4. Commit Scope

Prefer focused commits such as:

~~~text
feat(core): add transaction primitives
feat(crypto): add hash interface
feat(block): add block header model
test(protocol): add transaction vectors
docs(protocol): clarify nonce rules
~~~

## 5. Protocol-Critical Changes

Changes affecting canonical bytes, hashes, signatures, state transition, consensus, genesis, or P2P compatibility require documentation and test-vector updates in the same development cycle.

## 6. Testing Gate

Before merge, run the applicable:

- unit tests;
- integration tests;
- fuzz/property tests;
- protocol-vector tests;
- static analysis;
- formatting.

## 7. Main Branch Policy

Do not use `main` as a scratch branch. If an experiment is uncertain, create a branch.

## 8. Release Direction

Devnet releases may move faster than Testnet/Mainnet releases. Production promotion requires protocol compatibility, security review, observability, recovery procedures, and documented upgrade behavior.
