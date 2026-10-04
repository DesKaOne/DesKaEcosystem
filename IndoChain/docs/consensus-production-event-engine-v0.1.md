# IndoChain v0.1 — Production Consensus Event Engine

## Purpose

This document defines the first production-oriented orchestration capability above the existing deterministic \`ValidatorRuntime\` and \`RoundDriver\`.

The engine owns local consensus event lifecycle and timeout-token validity. It does not own canonical blockchain storage, P2P transport, peer discovery, wall-clock scheduling, transaction-selection policy, or application logic.

## State ownership

\`\`\`
ConsensusEngine
    |
    +-- ValidatorRuntime   consensus state machine
    +-- RoundDriver        authenticated event routing
    +-- timeout token      local timer identity
    |
    +-- external clock/scheduler
    +-- external P2P publisher
    +-- Node canonical commit
\`\`\`

Canonical block/state ownership remains in \`Node\`. Consensus finality remains an explicit handoff to the node execution/commit boundary.

## Timeout semantics

The current v0.1 runtime has three operational phase timeouts:

1. Proposal
2. Prevote
3. Precommit

\`TimeoutPolicy\` supplies deterministic durations for these phases. The engine does not read wall-clock time itself.

The external scheduler arms a timer using the duration returned by \`ArmTimeout\`. When that timer fires, it must submit the returned \`TimeoutToken\` to \`HandleTimeout\`.

A timeout token is valid only when all of these match the current runtime:

- height
- round
- phase
- generation

Any mismatch is rejected as stale. Arming a new timeout increments generation and invalidates every previous token. Finalized runtimes cannot accept timeout events.

## Timeout evidence

\`HandleTimeout\` creates authenticated timeout evidence for the local validator and routes it through the existing authenticated \`RoundDriver\`.

The local timeout does **not** advance the round by itself.

Round advancement requires quorum-backed timeout evidence from the validator set. The caller disseminates the returned timeout message through the production P2P layer. Received timeout evidence is accumulated by \`RoundDriver\`.

\`TryAdvanceRound\` attempts the quorum transition. If quorum is not reached, the queued evidence remains available for later retry. If quorum is reached:

\`\`\`
timeout quorum
    ↓
validated TimeoutCertificate
    ↓
round += 1
phase = Proposal
    ↓
new proposal timeout armed
\`\`\`

The runtime's locked proposal/lock proof is preserved only when authenticated timeout evidence carries a valid lock proof according to the existing runtime rules.

## Event routing

The engine routes proposal, prevote, precommit, and timeout messages through \`RoundDriver\`.

For proposal/vote events, a phase or round transition causes the current timeout generation to be invalidated and a timeout for the new phase to be armed.

Invalid authenticated messages never mutate runtime state.

## Crash/restart rule

Timeout tokens are operational state and are not canonical consensus evidence.

A process restart must discard all previous timeout tokens and create a new token from the reconstructed runtime context. A stale pre-restart token must never mutate the recovered runtime.

Durable consensus evidence remains governed by the existing evidence/recovery boundaries.

## Non-goals

This milestone does not claim completion of:

- wall-clock scheduler integration;
- production P2P broadcast/retransmission;
- durable validator-set/epoch lifecycle;
- canonical protocol serialization freeze;
- economic gas/fee semantics;
- EVM execution;
- full multi-process crash/restart orchestration.

Those remain explicit integration gates.

## Tests

The engine tests cover:

- phase-specific timeout duration selection;
- timeout generation invalidation;
- stale timeout rejection after a phase transition;
- authenticated local timeout creation;
- quorum-backed round advancement;
- automatic re-arming for the next proposal phase;
- deterministic timeout payload equality for equivalent validators.

## Invariants

1. A timeout cannot mutate consensus state unless its height, round, phase, and generation exactly match the currently armed timeout.
2. Finalized consensus state rejects timeout mutation.
3. Local timeout evidence is authenticated before entering the runtime.
4. A single timeout never advances the round without quorum.
5. Failed quorum attempts preserve queued timeout evidence for retry.
6. A successful round change resets phase to \`Proposal\`.
7. A successful round change invalidates the previous timeout generation.
8. The engine never commits canonical block/state.
9. The engine never bypasses validator membership, voting power, signature verification, or quorum validation.
10. Wall-clock scheduling remains outside the deterministic consensus state machine.
