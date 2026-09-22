# IndoChain TransactionsRoot Implementation v0.1

> Status: Development implementation baseline

## Purpose

Connect the block transaction list to a deterministic development commitment.

## Current Development Algorithm

For each transaction, in block order:

1. encode the complete signed transaction with the current development encoding;
2. prefix the encoded transaction with its 4-byte big-endian length;
3. append the result to a byte stream;
4. hash the complete stream with SHA-256.

The resulting digest is the development TransactionsRoot.

~~~text
Transactions[]
      |
      v
SignedBytes(tx)
      |
      v
Length-prefix each tx
      |
      v
Ordered byte stream
      |
      v
SHA-256
      |
      v
TransactionsRoot
~~~

## Validation

During block execution, the node validates Header.TransactionsRoot before executing transactions.

A non-zero header root must match the computed root. A zero root remains accepted during development.

## Determinism

The root is sensitive to transaction ordering. Identical ordered transaction lists produce the same root.

## Deferred Protocol Decisions

This implementation is not the final wire/protocol specification. The following remain subject to protocol freeze:

- canonical transaction serialization;
- transaction hash algorithm;
- Merkle/tree versus flat commitment structure;
- empty-list root;
- domain separation;
- maximum transaction encoding size;
- final TransactionsRoot encoding.

The implementation must be replaced or aligned with the frozen serialization-spec-v0.1 and block-spec-v0.1 before production use.