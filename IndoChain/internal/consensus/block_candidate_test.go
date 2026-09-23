package consensus

import (
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/state"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

func TestBuildBlockCandidateBuildsDeterministicEmptyBlock(t *testing.T) {
	s := state.New()
	s.Set(types.Address{1}, state.Account{Balance: 100})

	ctx := BlockProductionContext{
		State: RoundState{
			ProtocolVersion: 1,
			ChainID:         1001,
			Height:          7,
			Round:           2,
			Phase:           PhaseProposal,
		},
		PreviousHash: types.Hash{9},
		Proposer:     []byte{3},
	}
	rules := block.ExecutionRules{ChainID: 1001, ProtocolVersion: 1}

	first, err := BuildBlockCandidate(BlockCandidateInput{
		Context:   ctx,
		Timestamp: 100,
		Rules:     rules,
	}, s)
	if err != nil {
		t.Fatalf("build first candidate: %v", err)
	}
	second, err := BuildBlockCandidate(BlockCandidateInput{
		Context:   ctx,
		Timestamp: 100,
		Rules:     rules,
	}, s)
	if err != nil {
		t.Fatalf("build second candidate: %v", err)
	}

	firstHash, err := block.Hash(first)
	if err != nil {
		t.Fatalf("hash first: %v", err)
	}
	secondHash, err := block.Hash(second)
	if err != nil {
		t.Fatalf("hash second: %v", err)
	}
	if firstHash != secondHash {
		t.Fatalf("candidate hash is not deterministic")
	}
	if first.Header.Height != 8 {
		t.Fatalf("expected height 8, got %d", first.Header.Height)
	}
	if first.Header.StateRoot != s.Root() {
		t.Fatalf("empty candidate changed state root")
	}
	if s.Root() != second.Header.StateRoot {
		t.Fatalf("canonical state was mutated")
	}
}

func TestBuildBlockCandidateRejectsExecutionContextMismatch(t *testing.T) {
	s := state.New()
	ctx := BlockProductionContext{
		State: RoundState{
			ProtocolVersion: 1,
			ChainID:         1001,
			Height:          0,
			Phase:           PhaseProposal,
		},
		Proposer: []byte{1},
	}

	_, err := BuildBlockCandidate(BlockCandidateInput{
		Context: ctx,
		Rules: block.ExecutionRules{
			ChainID:         1002,
			ProtocolVersion: 1,
		},
	}, s)
	if err == nil {
		t.Fatal("expected execution context mismatch")
	}
}

func TestBuildBlockCandidateRejectsNilState(t *testing.T) {
	_, err := BuildBlockCandidate(BlockCandidateInput{}, nil)
	if err != ErrNilBlockProductionState {
		t.Fatalf("expected nil state error, got %v", err)
	}
}
