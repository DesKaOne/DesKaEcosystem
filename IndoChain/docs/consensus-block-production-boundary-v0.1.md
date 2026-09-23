# IndoChain v0.1 — Consensus Block Production Boundary

## Purpose

This document defines the development boundary between the consensus runtime and block construction.

The boundary is intentionally small: consensus supplies the current round context and expected proposer; a block producer constructs a candidate block; validation checks that the candidate belongs to the expected chain context and next height.

## Implemented boundary

Implementation:

- `IndoChain/internal/consensus/block_production.go`
- `BlockProductionContext`
- `BlockProducer`
- `ValidateProducedBlock`

The context contains:

- current `RoundState`
- previous block hash
- proposer identifier

The producer interface is:

`ProduceBlock(ctx BlockProductionContext) (block.Block, error)`

The interface does not prescribe how transactions are selected or ordered.

## Candidate validation

`ValidateProducedBlock` verifies:

1. round state is valid;
2. proposer context is present;
3. protocol version matches;
4. chain ID matches;
5. candidate height is current height + 1;
6. previous block hash matches;
7. candidate proposer matches the supplied proposer;
8. the development transaction root is valid.

It then returns the deterministic development block hash as an opaque proposal payload.

## Relationship to consensus runtime

The returned block hash can be used as the opaque payload of the existing consensus proposal/vote boundaries.

Conceptually:

```text
RoundState + previous hash + proposer
                |
                v
         BlockProducer
                |
                v
          candidate block
                |
                v
      ValidateProducedBlock
                |
                v
       development block hash
                |
                v
      Consensus Proposal/Vote
```

This does not make the block hash a final canonical proposal encoding. The current block hash implementation explicitly remains a development identifier until serialization is frozen.

## Deliberate non-goals

This milestone does not implement:

- transaction selection policy;
- deterministic mempool ordering policy;
- fee/gas accounting;
- execution or state-root calculation;
- block persistence;
- canonical serialization;
- proposer scheduling beyond the existing selector;
- timeout or round-change logic;
- production BFT algorithm;
- validator-set lifecycle;
- signature authority registry;
- P2P proposal transport;
- automatic block commit after finality.

## Next integration direction

The next production-oriented integration should connect a concrete block producer to node state/mempool and block execution without bypassing the existing execution and storage boundaries.

That integration must preserve deterministic candidate construction and must not silently treat the development block hash as the final protocol serialization.
