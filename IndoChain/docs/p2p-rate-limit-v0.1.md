# P2P Rate-Limit Admission Boundary v0.1

## Purpose

Define the first per-peer admission guard against unbounded message processing.

## Policy

Each peer has a message allowance configured by `MaxMessages`.

A peer may consume its allowance through `Allow`. Once the allowance is exhausted, further admissions return a rate-limit error until the peer's counter is reset.

Counters are maintained independently per PeerID and protected for concurrent access.

## Boundary

This is an implementation boundary, not the final production rate-limit algorithm. It does not define time windows, token buckets, bandwidth quotas, message-class weighting, or peer penalties.

It also does not perform message decoding, transaction validation, block validation, consensus, or canonical state mutation.

A future production limiter can replace the counter implementation while preserving the admission interface.
