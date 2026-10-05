# IndoChain v0.1 — Authenticated Round Driver Boundary

## Purpose

This document freezes the first executable orchestration layer above ValidatorRuntime.

The driver is intentionally event-driven and deterministic. It does not own a clock, peer discovery, retransmission, failure detection, or canonical blockchain state.

## Lifecycle

The authenticated driver routes:

Proposal
→ Prevote
→ Precommit
→ Finality

and separately collects:

Timeout messages
→ explicit timeout-evidence batch
→ round change

Every proposal, prevote, precommit, and timeout message is authenticated against the configured validator authority before it reaches the runtime.

## Locked invariants

1. Consensus messages must match the runtime protocol/chain/epoch/height/round context.
2. Sender must belong to the validator set.
3. Proposal/vote/timeout signatures must validate against the configured authority.
4. Proposal and vote rejection never mutates runtime state.
5. Timeout evidence is collected without advancing the round.
6. A round transition occurs only through AdvanceRoundWithTimeoutEvidence.
7. Timeout evidence is cleared only after a successful round transition.
8. Failed timeout batches remain available for caller-controlled recovery/discard.
9. Runtime remains the sole owner of lock, quorum, phase, and finality state.
10. The driver never commits canonical block/state storage.

## Production boundary

This is a production-oriented consensus ingress/orchestration boundary, not a complete production BFT scheduler.

Still external:

- timeout clock/scheduler;
- peer discovery and network-wide broadcast/retransmission;
- validator lifecycle and stake/delegation;
- production proposer selection;
- durable consensus WAL/snapshot restore;
- canonical block execution/commit;
- formal BFT safety/liveness proof.

The next integration target is to bind this driver to the existing P2P consensus transport and block-candidate production without moving authority out of the runtime.
