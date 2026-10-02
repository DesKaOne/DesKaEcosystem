package consensus

import (
    "github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
)

var ErrInvalidConsensusProposal = ErrInvalidConsensusEngine

// BuildProposalMessage creates the authenticated proposal event for the
// runtime's current height/round. Candidate validation remains at the node
// integration boundary because canonical state and execution rules are not
// owned by the consensus engine.
func (e *ConsensusEngine) BuildProposalMessage(candidate block.Block) (Message, error) {
    if e == nil || e.runtime == nil || e.signer == nil {
        return Message{}, ErrNilConsensusEngine
    }
    state := e.runtime.State()
    ctx := BlockProductionContext{
        State:        state,
        PreviousHash: candidate.Header.PreviousHash,
        Proposer:     append([]byte(nil), candidate.Header.Proposer...),
    }
    proposal, err := NewBlockProposal(ctx, candidate)
    if err != nil {
        return Message{}, err
    }
    msg := Message{
        ProtocolVersion: state.ProtocolVersion,
        ChainID: state.ChainID,
        Epoch: state.Epoch,
        Height: state.Height,
        Round: state.Round,
        Sender: append([]byte(nil), candidate.Header.Proposer...),
        Type: MessageTypeProposal,
        Payload: proposal.MessagePayload(),
    }
    return msg.Sign(e.signer)
}
