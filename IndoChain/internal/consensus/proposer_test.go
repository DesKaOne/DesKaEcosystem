package consensus

import (
	"bytes"
	"testing"
)

func TestRoundRobinProposerUsesDeterministicValidatorOrder(t *testing.T) {
	validators, err := NewValidatorSet([][]byte{
		[]byte("validator-b"),
		[]byte("validator-a"),
		[]byte("validator-c"),
	})
	if err != nil {
		t.Fatal(err)
	}
	state, err := NewRoundState(1, 1001, 0, 1)
	if err != nil {
		t.Fatal(err)
	}

	selector := RoundRobinProposer{}
	for round, want := range []string{"validator-a", "validator-b", "validator-c", "validator-a"} {
		current, err := state.AdvanceRound(uint64(round))
		if err != nil {
			t.Fatal(err)
		}
		got, err := selector.Proposer(current, validators)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != want {
			t.Fatalf("round %d proposer = %q, want %q", round, got, want)
		}
	}
}

func TestRoundRobinProposerClonesResult(t *testing.T) {
	validators, err := NewValidatorSet([][]byte{[]byte("validator-a")})
	if err != nil {
		t.Fatal(err)
	}
	state, err := NewRoundState(1, 1001, 0, 1)
	if err != nil {
		t.Fatal(err)
	}

	got, err := (RoundRobinProposer{}).Proposer(state, validators)
	if err != nil {
		t.Fatal(err)
	}
	got[0] = 'X'
	if bytes.Equal(got, validators.Validators[0]) {
		t.Fatal("proposer result aliases validator set")
	}
}

func TestRoundRobinProposerRejectsEmptySet(t *testing.T) {
	state, err := NewRoundState(1, 1001, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (RoundRobinProposer{}).Proposer(state, ValidatorSet{}); err != ErrNoValidators {
		t.Fatalf("error = %v, want %v", err, ErrNoValidators)
	}
}

func TestRoundRobinProposerRejectsInvalidState(t *testing.T) {
	validators, err := NewValidatorSet([][]byte{[]byte("validator-a")})
	if err != nil {
		t.Fatal(err)
	}
	state := RoundState{ProtocolVersion: 1, ChainID: 1001, Phase: 0}
	if _, err := (RoundRobinProposer{}).Proposer(state, validators); err != ErrInvalidConsensusPhase {
		t.Fatalf("error = %v, want %v", err, ErrInvalidConsensusPhase)
	}
}
