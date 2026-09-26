package consensus

import (
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

func TestValidateProducedBlockReturnsDeterministicProposalPayload(t *testing.T) {
	state := RoundState{
		ProtocolVersion: 1,
		ChainID:         1001,
		Epoch:           1,
		Height:          7,
		Round:           2,
		Phase:           PhaseProposal,
	}
	proposer := []byte("validator-a")
	ctx := BlockProductionContext{
		State:        state,
		PreviousHash: types.Hash{1},
		Proposer:     proposer,
	}
	candidate := block.Block{
		Header: block.Header{
			Version:      1,
			ChainID:      1001,
			Height:       8,
			PreviousHash: ctx.PreviousHash,
			Proposer:     append([]byte(nil), proposer...),
		},
	}

	first, err := ValidateProducedBlock(ctx, candidate)
	if err != nil {
		t.Fatalf("ValidateProducedBlock() error = %v", err)
	}
	second, err := ValidateProducedBlock(ctx, candidate)
	if err != nil {
		t.Fatalf("second ValidateProducedBlock() error = %v", err)
	}
	if first != second {
		t.Fatalf("proposal payload changed: first=%x second=%x", first, second)
	}
}

func TestValidateProducedBlockRejectsContextMismatch(t *testing.T) {
	state := RoundState{
		ProtocolVersion: 1,
		ChainID:         1001,
		Epoch:           1,
		Height:          7,
		Round:           2,
		Phase:           PhaseProposal,
	}
	ctx := BlockProductionContext{
		State:        state,
		PreviousHash: types.Hash{1},
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

	if _, err := ValidateProducedBlock(ctx, candidate); err == nil {
		t.Fatal("ValidateProducedBlock() error = nil, want context mismatch")
	}
}

func TestValidateProducedBlockRejectsProposerMismatch(t *testing.T) {
	state := RoundState{
		ProtocolVersion: 1,
		ChainID:         1001,
		Epoch:           1,
		Height:          7,
		Round:           2,
		Phase:           PhaseProposal,
	}
	ctx := BlockProductionContext{
		State:        state,
		PreviousHash: types.Hash{1},
		Proposer:     []byte("validator-a"),
	}
	candidate := block.Block{
		Header: block.Header{
			Version:      1,
			ChainID:      1001,
			Height:       8,
			PreviousHash: ctx.PreviousHash,
			Proposer:     []byte("validator-b"),
		},
	}

	if _, err := ValidateProducedBlock(ctx, candidate); err == nil {
		t.Fatal("ValidateProducedBlock() error = nil, want proposer mismatch")
	}
}
