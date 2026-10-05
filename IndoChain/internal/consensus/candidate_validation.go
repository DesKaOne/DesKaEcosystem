package consensus

import (
    "bytes"
    "errors"
    "fmt"

    "github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
    "github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/state"
    "github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

var (
    ErrNilCandidateState = errors.New("nil candidate validation state")
    ErrCandidatePayloadMismatch = errors.New("candidate payload mismatch")
)

// ValidateBlockCandidateForConsensus performs the complete non-mutating
// validation required before a received candidate can be accepted as the
// authenticated consensus proposal. It validates consensus context, roots,
// deterministic hash, and execution against a canonical-state snapshot.
func ValidateBlockCandidateForConsensus(
    ctx BlockProductionContext,
    candidate block.Block,
    canonicalState *state.State,
    rules block.ExecutionRules,
) (types.Hash, error) {
    if canonicalState == nil {
        return types.Hash{}, ErrNilCandidateState
    }
    if rules.ChainID != ctx.State.ChainID || rules.ProtocolVersion != ctx.State.ProtocolVersion {
        return types.Hash{}, fmt.Errorf("%w: execution rules context mismatch", ErrBlockProductionContextMismatch)
    }

    payload, err := ValidateProducedBlock(ctx, candidate)
    if err != nil {
        return types.Hash{}, err
    }

    working := canonicalState.Snapshot()
    if err := block.ExecuteBlock(
        working,
        candidate,
        ctx.State.Height+1,
        ctx.PreviousHash,
        rules,
    ); err != nil {
        return types.Hash{}, err
    }

    // ExecuteBlock validates a non-zero StateRoot. Keep this explicit equality
    // check as the consensus boundary's final execution-result assertion.
    if candidate.Header.StateRoot != (types.Hash{}) && candidate.Header.StateRoot != working.Root() {
        return types.Hash{}, block.ErrStateRootMismatch
    }
    return payload, nil
}

// ValidateAuthenticatedBlockProposal performs signature/authority validation
// and candidate execution validation before mutating consensus phase state.
func (r *ValidatorRuntime) ValidateAuthenticatedBlockProposal(
    msg Message,
    candidate block.Block,
    ctx BlockProductionContext,
    canonicalState *state.State,
    executionRules block.ExecutionRules,
    resolver TimeoutAuthorityResolver,
) error {
    if r == nil {
        return ErrInvalidConsensusRuntime
    }
    if err := ValidateBlockCandidateContext(msg, ctx, candidate); err != nil {
        return err
    }
    payload, err := ValidateBlockCandidateForConsensus(ctx, candidate, canonicalState, executionRules)
    if err != nil {
        return err
    }
    if !bytes.Equal(payload[:], msg.Payload) {
        return ErrCandidatePayloadMismatch
    }
    return r.AcceptAuthenticatedProposal(msg, resolver)
}

// ValidateBlockCandidateContext checks that the authenticated proposal and
// supplied candidate describe exactly the same consensus context.
func ValidateBlockCandidateContext(
    msg Message,
    ctx BlockProductionContext,
    candidate block.Block,
) error {
    if msg.Type != MessageTypeProposal {
        return ErrInvalidConsensusMessage
    }
    if msg.ProtocolVersion != ctx.State.ProtocolVersion ||
        msg.ChainID != ctx.State.ChainID ||
        msg.Epoch != ctx.State.Epoch ||
        msg.Height != ctx.State.Height ||
        msg.Round != ctx.State.Round {
        return ErrStateContextMismatch
    }
    if candidate.Header.Height != ctx.State.Height+1 ||
        candidate.Header.Version != ctx.State.ProtocolVersion ||
        candidate.Header.ChainID != ctx.State.ChainID ||
        candidate.Header.PreviousHash != ctx.PreviousHash {
        return ErrBlockProductionContextMismatch
    }
    if !bytes.Equal(candidate.Header.Proposer, msg.Sender) {
        return ErrUnexpectedProposer
    }
    return nil
}
