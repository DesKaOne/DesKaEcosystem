# IndoChain Block Hash Implementation v0.1

> Status: Development implementation baseline

## Purpose

Provide a deterministic block identity derived from the block header commitments.

## Current Development Input

~~~text
Version
ChainID
Height
Timestamp
PreviousHash
TransactionsRoot
StateRoot
Proposer
ConsensusEvidence
~~~

Variable-length byte fields use a 4-byte big-endian length prefix in this development encoding.

The resulting header byte stream is hashed with SHA-256.

## Chain Relationship

Block N contains `PreviousHash` referencing the identity of the previous block.

~~~text
Block N-1 --Hash--> H(N-1)
                    ^
                    |
             PreviousHash
                    |
                 Block N
~~~

## Important Boundary

The raw transaction list is not hashed directly by this function. Transactions are committed through `TransactionsRoot`, which is then included in the header hash.

## Deferred

This is not the final protocol block hash. Canonical serialization, hash algorithm, domain separation, header field order, proposer encoding, consensus evidence encoding, and genesis/block identity rules remain subject to protocol freeze.