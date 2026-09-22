package consensus

import (
	"errors"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

func TestRoundStateTransitions(t *testing.T) {
	s, err := NewRoundState(1, 1001, 2, 7)
	if err != nil {
		t.Fatalf("new state: %v", err)
	}
	if s.Phase != PhaseProposal || s.Round != 0 {
		t.Fatalf("unexpected initial state: %+v", s)
	}

	s, err = s.AdvancePhase(PhasePrevote)
	if err != nil {
		t.Fatalf("advance prevote: %v", err)
	}
	s, err = s.AdvancePhase(PhasePrecommit)
	if err != nil {
		t.Fatalf("advance precommit: %v", err)
	}
	s, err = s.AdvanceRound(1)
	if err != nil {
		t.Fatalf("advance round: %v", err)
	}
	if s.Round != 1 || s.Phase != PhaseProposal {
		t.Fatalf("round transition did not reset phase: %+v", s)
	}
	s, err = s.AdvanceHeight(8)
	if err != nil {
		t.Fatalf("advance height: %v", err)
	}
	if s.Height != 8 || s.Round != 0 || s.Phase != PhaseProposal {
		t.Fatalf("height transition did not reset round state: %+v", s)
	}
}

func TestRoundStateRejectsRegression(t *testing.T) {
	s, _ := NewRoundState(1, 1001, 1, 5)
	s, _ = s.AdvanceRound(2)
	if _, err := s.AdvanceRound(1); !errors.Is(err, ErrRoundRegression) {
		t.Fatalf("expected round regression, got %v", err)
	}
	if _, err := s.AdvanceHeight(4); !errors.Is(err, ErrHeightRegression) {
		t.Fatalf("expected height regression, got %v", err)
	}
	s, _ = s.AdvancePhase(PhasePrecommit)
	if _, err := s.AdvancePhase(PhasePrevote); !errors.Is(err, ErrPhaseRegression) {
		t.Fatalf("expected phase regression, got %v", err)
	}
}

func TestRoundStateContext(t *testing.T) {
	a, _ := NewRoundState(1, 1001, 3, 10)
	b := a
	if !a.SameContext(b) {
		t.Fatal("identical contexts must match")
	}
	b.Epoch++
	if a.SameContext(b) {
		t.Fatal("different epochs must not match")
	}
	b = a
	b.ChainID = types.ChainID(1002)
	if a.SameContext(b) {
		t.Fatal("different chain ids must not match")
	}
}

func TestRoundStateValidation(t *testing.T) {
	if _, err := NewRoundState(0, 1001, 0, 0); !errors.Is(err, ErrInvalidConsensusState) {
		t.Fatalf("expected invalid state, got %v", err)
	}
	s := RoundState{ProtocolVersion: 1, ChainID: 1001, Phase: Phase(99)}
	if !errors.Is(s.Validate(), ErrInvalidConsensusPhase) {
		t.Fatalf("expected invalid phase, got %v", s.Validate())
	}
}
