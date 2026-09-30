package consensus

import (
	"errors"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/crypto"
)

var (
	ErrNilProposalSigner = errors.New("nil proposal signer")
	ErrInvalidProposalCandidate = errors.New("invalid proposal candidate")
)

// BuildSignedProposalMessage validates a deterministic block candidate,
// derives its consensus payload, constructs the exact current-round proposal
// message, and signs it. Candidate dissemination remains separate from this
// consensus evidence message because canonical block serialization is not yet
// frozen in v0.1.
func BuildSignedProposalMessage(
	ctx BlockProductionContext,
	candidate block.Block,
	signer crypto.Signer,
) (Message, BlockProposal, error) {
	if signer == nil {
		return Message{}, BlockProposal{}, ErrNilProposalSigner
	}
	proposal, err := NewBlockProposal(ctx, candidate)
	if err != nil {
		return Message{}, BlockProposal{}, err
	}
	msg := Message{
		ProtocolVersion: ctx.State.ProtocolVersion,
		ChainID: ctx.State.ChainID,
		Epoch: ctx.State.Epoch,
		Height: ctx.State.Height,
		Round: ctx.State.Round,
		Sender: append([]byte(nil), ctx.Proposer...),
		Type: MessageTypeProposal,
		Payload: proposal.MessagePayload(),
	}
	signed, err := msg.Sign(signer)
	if err != nil {
		return Message{}, BlockProposal{}, err
	}
	return signed, proposal, nil
}
