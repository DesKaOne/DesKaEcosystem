# IndoChain Blocks

> Status: Draft / Block Design Baseline

## 1. Purpose

Blocks provide the ordered, authenticated container for transactions and state transitions.

## 2. Proposed Structure

~~~text
Block
├── Header
│   ├── Height
│   ├── Timestamp
│   ├── PreviousHash
│   ├── MerkleRoot
│   ├── StateRoot
│   ├── TransactionsRoot
│   ├── Validator
│   └── Signature
└── Transactions[]
~~~

The exact serialization and whether some roots are redundant will be resolved during protocol design.

## 3. Header Fields

### Height

Monotonically identifies the block position in the canonical chain.

### Timestamp

Represents protocol block time. Validation must define acceptable clock drift and must not make consensus dependent on exact local wall-clock agreement.

### PreviousHash

Commits the block to its parent.

### MerkleRoot / TransactionsRoot

Provides commitment to transaction data. The final tree construction must be specified once.

### StateRoot

Commits to the resulting canonical state after block execution.

### Validator

Identifies the validator/proposer associated with the block.

### Signature

Authenticates the block according to consensus rules.

## 4. Block Validation

Suggested checks:

1. Header decoding.
2. Protocol version.
3. Height and parent relationship.
4. Timestamp constraints.
5. Parent hash.
6. Validator authorization.
7. Header signature.
8. Transaction structure.
9. Transaction execution.
10. State root.
11. Transaction commitment.
12. Consensus/finality rules.

## 5. Block Production

The proposer workflow is conceptually:

~~~text
Select valid transactions
        ↓
Build candidate block
        ↓
Execute transactions
        ↓
Calculate roots
        ↓
Sign proposal
        ↓
Broadcast proposal
        ↓
Collect consensus votes
        ↓
Finalize / commit
~~~

Exact proposal and voting semantics belong in the consensus specification.

## 6. Block Size and Limits

The protocol must define deterministic limits for:

- maximum serialized block size;
- maximum transaction count;
- maximum execution/gas budget;
- maximum individual transaction size;
- maximum evidence payload.

Limits must account for bandwidth, CPU, memory, and storage constraints.

## 7. Fork and Reorganization

The node must detect competing branches and apply the chosen consensus/finality rules. With finalized BFT-style blocks, reorganization behavior should be tightly constrained by finality semantics.

## 8. Genesis

The genesis block is a special protocol root and must define:

- chain identity;
- initial state;
- initial validator configuration;
- initial protocol parameters;
- genesis timestamp;
- any initial allocation.

Genesis format belongs in a dedicated specification and must be reproducible from a deterministic configuration.

## 9. Test Vectors

Before implementation freeze, provide fixtures for:

- genesis block;
- empty block;
- single-transaction block;
- multiple-transaction block;
- invalid parent;
- invalid timestamp;
- invalid signature;
- invalid transaction root;
- invalid state root.
