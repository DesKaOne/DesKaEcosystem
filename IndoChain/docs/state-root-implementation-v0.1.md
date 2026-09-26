# IndoChain State Root Implementation v0.1

> Status: Development implementation baseline

## Purpose

Provide a deterministic development commitment for the current in-memory account state.

## Current Algorithm

The implementation:

1. collects all account keys;
2. sorts keys lexicographically by their raw byte-preserving string representation;
3. serializes each account as:
   - key length: uint32 big-endian;
   - raw key bytes;
   - balance: uint64 big-endian;
   - nonce: uint64 big-endian;
4. hashes the resulting byte stream with SHA-256.

## Guarantees

For the current implementation:

- insertion order does not affect the root;
- identical states produce identical roots;
- changing an account changes the root;
- an empty state has a deterministic root.

## Protocol Warning

This is a development commitment only.

It does not freeze:

- final state tree/trie structure;
- canonical state serialization;
- production hash algorithm;
- proof format;
- account schema;
- storage layout;
- StateRoot rules for the final protocol.

The final StateRoot must be aligned with the serialization and state-transition specifications before protocol freeze.