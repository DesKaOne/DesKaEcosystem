# IndoChain Block Specification v0.1

> Status: Draft Specification
> Scope: Devnet protocol baseline

## 1. Logical Structure

~~~text
Block
├── Header
│   ├── Version
│   ├── ChainID / Network Identity
│   ├── Height
│   ├── Timestamp
│   ├── PreviousHash
│   ├── TransactionsRoot
│   ├── StateRoot
│   ├── Validator / Proposer
│   └── ConsensusEvidence / Signature
└── Transactions[]
~~~

The exact binary layout is not yet frozen.

## 2. Height

Height identifies the block position in the canonical chain. Genesis occupies the protocol-defined initial height.

## 3. Timestamp

Timestamp is consensus-relevant metadata and must obey protocol validation rules. Exact allowed clock drift and block-time policy remain TBD.

## 4. Previous Hash

Every non-genesis block references the canonical hash of its predecessor. The node must reject an invalid linkage.

## 5. Transaction Commitment

Transactions must have a deterministic commitment, currently represented conceptually as TransactionsRoot.

The exact Merkle/tree construction remains TBD.

## 6. State Commitment

StateRoot commits to the resulting canonical state after block execution.

The exact state commitment structure remains TBD.

## 7. Proposer / Validator

The block identifies the proposer/validator according to the active consensus profile.

## 8. Consensus Evidence

PoS+BFT profiles may require voting/finality evidence or a protocol-defined signature set. PoW profiles may use mining-specific proof fields.

Consensus-specific fields must not be silently mixed between profiles.

## 9. Block Validation

Conceptual validation:

1. chain identity;
2. version;
3. height;
4. previous hash;
5. timestamp rules;
6. transaction commitment;
7. transaction validity/execution;
8. state commitment;
9. proposer/validator authorization;
10. consensus evidence.

## 10. Genesis

Genesis has no ordinary predecessor. Its identity must be reproducible from the genesis configuration and protocol-defined canonical encoding.

## 11. Test Vectors

Provide vectors for genesis, one valid child block, invalid previous hash, invalid transaction root, invalid state root, invalid proposer authorization, and invalid consensus evidence.

## 12. Open Items

- exact header field widths;
- canonical encoding;
- transaction-root algorithm;
- state-root algorithm;
- timestamp limits;
- consensus evidence encoding;
- block-size/transaction-count limits.
