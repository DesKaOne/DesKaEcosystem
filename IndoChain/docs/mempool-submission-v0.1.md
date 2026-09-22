# Mempool Submission Boundary v0.1

## Purpose

Define the node-facing write boundary between signed transactions and the local mempool.

Submission validates the transaction against the current node execution rules and a snapshot of canonical state before admitting it to the mempool.

## Guarantees

A successful submission:

- validates protocol version and chain ID;
- verifies the transaction signature using the supplied public key;
- checks the transaction can execute against the current canonical state;
- adds the transaction to the local mempool.

Submission does not:

- mutate canonical state;
- create a block;
- finalize or confirm the transaction;
- write a block to chain storage.

## Current API

NodeMempool.SubmitTransaction(tx, publicKey)

The public key is an explicit parameter because the v0.1 transaction model does not yet contain a canonical sender public-key field.

## Boundary

The mempool remains separate from canonical chain state. Consensus/block production will consume eligible mempool transactions later. Rejection, duplicate handling, and capacity errors remain local admission outcomes until protocol-level semantics are frozen.

## DesKaCash boundary

A financial application may submit signed settlement transactions through a public node/RPC adapter. Its application ledger remains separate from the chain mempool and canonical state.
