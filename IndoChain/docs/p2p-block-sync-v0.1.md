# P2P Block Synchronization Boundary v0.1

## Purpose

Define the first request/response boundary for obtaining canonical blocks from a peer.

## Request

`BlockRequest` contains:

- `FromHeight` — starting canonical height;
- `Limit` — maximum number of blocks requested.

A request is accepted only when the limit is non-zero and does not exceed the configured maximum.

## Response

`BlockResponse` carries block objects. The response boundary does not itself import or commit them.

## Reader boundary

`SyncReader` exposes single-height canonical block reads using the existing node `ChainReader` shape. P2P synchronization therefore consumes a read-only boundary rather than accessing node storage directly.

## Safety boundary

This layer does not:

- mutate canonical state;
- commit received blocks;
- decide finality;
- choose the canonical fork;
- execute transactions.

A future sync coordinator may validate and import received blocks through the node's existing canonical import boundary.
