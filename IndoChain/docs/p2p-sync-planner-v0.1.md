# P2P Sync Planner v0.1

## Purpose

Define the bounded request planning step between a local canonical head and a remote advertised height.

## Rules

- if the remote height is not ahead of the local height, no request is produced;
- otherwise the next request starts at `localHeight + 1`;
- the requested batch is capped by `MaxBatch`;
- a zero `MaxBatch` is invalid.

## Boundary

The planner does not trust or import remote blocks. It only produces a bounded `BlockRequest`. Response validation remains the responsibility of the sync range reader/coordinator, and canonical import remains outside this planning layer.
