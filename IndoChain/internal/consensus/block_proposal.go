package consensus

import (
	"errors"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

var ErrInvalidBlockProposal = errors.New("invalid block proposal")

// BlockProposal is an in-memory bridge between a validated block candidate
// and the opaque proposal payload consumed by ValidatorRuntime.
//
// The payload is the current development block hash. This deliberately avoids
// defining canonical block serialization or a wire-level proposal format.
type BlockProposal struct {
	Candidate block.Block
	Payload   types.Hash
}

// NewBlockProposal validates a block candidate against the consensus
// block-production context and derives its deterministic development payload.
func NewBlockProposal(ctx BlockProductionContext, candidate block.Block) (BlockProposal, error) {
	payload, err := ValidateProducedBlock(ctx, candidate)
	if err != nil {
		return BlockProposal{}, err
	}

	return BlockProposal{
		Candidate: candidate,
		Payload:   payload,
	}, nil
}

// MessagePayload returns a cloned opaque payload suitable for a proposal
// Message or vote Message.
func (p BlockProposal) MessagePayload() []byte {
	out := make([]byte, len(p.Payload))
	copy(out, p.Payload[:])
	return out
}

// SamePayload reports whether the supplied payload identifies this candidate.
func (p BlockProposal) SamePayload(payload []byte) bool {
	return len(payload) == len(p.Payload) && string(payload) == string(p.Payload[:])
}
