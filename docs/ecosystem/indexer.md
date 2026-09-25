# IndoChain Indexer

> Status: Draft / Indexer Design Baseline

## Purpose

The indexer transforms canonical blockchain data into query-friendly datasets for explorers, analytics, APIs, and ecosystem applications.

## Architecture

~~~text
Canonical Node Data
      ↓
Indexer Worker
      ↓
Normalized Data Store
      ↓
Query/API Layer
      ↓
Explorer / Analytics / Apps
~~~

## Responsibilities

- consume finalized or otherwise explicitly selected canonical chain data;
- index blocks and transactions;
- index account activity;
- index validator events;
- index token/contract events where supported;
- maintain reprocessing/reindex capability.

## Storage

PostgreSQL is the current candidate for indexed application data. Node canonical state remains under node storage and is not replaced by the indexer database.

## Reorg / Finality

The indexer must distinguish provisional data from finalized data according to the selected consensus/finality model.

## Reliability

Required capabilities include checkpoints, idempotent processing, retry handling, corruption detection, and full reindex procedures.

## Open Decisions

Schema, event model, queueing architecture, retention policy, and indexing framework remain open.
