package consensus

import (
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

func TestNewBlockProposalBridgesCandidateToRuntimePayload(t *testing.T) {
	ctx := BlockProductionContext{
		State: RoundState{
			ProtocolVersion: 1,
			ChainID:         1001,
			Epoch:           1,
			Height:          7,
			Round:           2,
			Phase:           PhaseProposal,
		},
		PreviousHash: types.Hash{4},
		Proposer:     []byte("validator-a"),
	}
	candidate := block.Block{
		Header: block.Header{
			Version:      1,
			ChainID:      1001,
			Height:       8,
			PreviousHash: ctx.PreviousHash,
			Proposer:     ctx.Proposer,
		},
	}

	proposal, err := NewBlockProposal(ctx, candidate)
	if err != nil {
		t.Fatalf("NewBlockProposal() error = %v", err)
	}
	expected, err := ValidateProducedBlock(ctx, candidate)
	if err != nil {
		t.Fatalf("ValidateProducedBlock() error = %v", err)
	}
	if proposal.Payload != expected {
		t.Fatalf("proposal payload does not match candidate hash")
	}
	if !proposal.SamePayload(proposal.MessagePayload()) {
		t.Fatalf("proposal payload did not round-trip")
	}
}

func TestNewBlockProposalRejectsInvalidCandidate(t *testing.T) {
	ctx := BlockProductionContext{
		State: RoundState{
			ProtocolVersion: 1,
			ChainID:         1001,
			Height:          7,
			Phase:           PhaseProposal,
		},
		PreviousHash: types.Hash{4},
		Proposer:     []byte("validator-a"),
	}
	candidate := block.Block{
		Header: block.Header{
			Version:      1,
			ChainID:      1001,
			Height:       9,
			PreviousHash: ctx.PreviousHash,
			Proposer:     ctx.Proposer,
		},
	}

	if _, err := NewBlockProposal(ctx, candidate); err == nil {
		t.Fatal("expected invalid candidate rejection")
	}
}

func TestBlockProposalMessagePayloadIsCloned(t *testing.T) {
	proposal := BlockProposal{Payload: types.Hash{1, 2, 3}}
	payload := proposal.MessagePayload()
	payload[0] = 9
	if proposal.Payload[0] != 1 {
		t.Fatal("message payload mutated proposal payload")
	}
}
