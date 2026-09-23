package consensus

import (
	"errors"
	"testing"
)

func runtimeFixture(t *testing.T) (*ValidatorRuntime, RoundState, ValidatorSet, VotingPowerSet) {
	t.Helper()
	state, err := NewRoundState(1, 1001, 1, 8)
	if err != nil {
		t.Fatal(err)
	}
	validators, err := NewValidatorSet([][]byte{[]byte("validator-a"), []byte("validator-b"), []byte("validator-c")})
	if err != nil {
		t.Fatal(err)
	}
	power, err := NewVotingPowerSet([]ValidatorVotingPower{
		{ValidatorID: []byte("validator-a"), Power: 4},
		{ValidatorID: []byte("validator-b"), Power: 3},
		{ValidatorID: []byte("validator-c"), Power: 3},
	})
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := NewValidatorRuntime(RuntimeConfig{
		Rules: ValidationRules{
			ProtocolVersion: state.ProtocolVersion,
			ChainID: state.ChainID,
			RequireSender: true,
		},
		State: state, Validators: validators, VotingPower: power,
		Threshold: QuorumThreshold{Numerator: 2, Denominator: 3},
		Proposer: RoundRobinProposer{},
	})
	if err != nil {
		t.Fatal(err)
	}
	return runtime, state, validators, power
}

func runtimeMessage(state RoundState, sender string, kind MessageType, payload string) Message {
	return Message{
		ProtocolVersion: state.ProtocolVersion, ChainID: state.ChainID,
		Epoch: state.Epoch, Height: state.Height, Round: state.Round,
		Sender: []byte(sender), Type: kind, Payload: []byte(payload),
	}
}

func TestValidatorRuntimeAcceptsExpectedProposer(t *testing.T) {
	runtime, state, _, _ := runtimeFixture(t)
	if err := runtime.AcceptProposal(runtimeMessage(state, "validator-a", MessageTypeProposal, "block-8")); err != nil {
		t.Fatal(err)
	}
	if runtime.State().Phase != PhasePrevote {
		t.Fatalf("expected prevote phase, got %v", runtime.State().Phase)
	}
}

func TestValidatorRuntimeRejectsUnexpectedProposer(t *testing.T) {
	runtime, state, _, _ := runtimeFixture(t)
	err := runtime.AcceptProposal(runtimeMessage(state, "validator-b", MessageTypeProposal, "block-8"))
	if !errors.Is(err, ErrUnexpectedProposer) {
		t.Fatalf("expected unexpected proposer error, got %v", err)
	}
	if runtime.State().Phase != PhaseProposal {
		t.Fatalf("runtime phase changed after rejected proposal")
	}
}

func TestValidatorRuntimeAdvancesAndFinalizesAfterQuorum(t *testing.T) {
	runtime, state, _, _ := runtimeFixture(t)
	if err := runtime.AcceptProposal(runtimeMessage(state, "validator-a", MessageTypeProposal, "block-8")); err != nil {
		t.Fatal(err)
	}
	if err := runtime.AddVote(runtimeMessage(state, "validator-a", MessageTypeVote, "block-8")); err != nil {
		t.Fatal(err)
	}
	if runtime.State().Phase != PhasePrevote {
		t.Fatalf("expected prevote before quorum, got %v", runtime.State().Phase)
	}
	if err := runtime.AddVote(runtimeMessage(state, "validator-b", MessageTypeVote, "block-8")); err != nil {
		t.Fatal(err)
	}
	if runtime.State().Phase != PhasePrecommit {
		t.Fatalf("expected precommit after quorum, got %v", runtime.State().Phase)
	}

	certificate, err := runtime.FinalizeProposal()
	if err != nil {
		t.Fatal(err)
	}
	if runtime.State().Phase != PhaseFinalized {
		t.Fatalf("expected finalized phase, got %v", runtime.State().Phase)
	}
	if string(certificate.Payload) != "block-8" {
		t.Fatalf("unexpected certificate payload %q", certificate.Payload)
	}
}

func TestValidatorRuntimeDoesNotFinalizeWithoutQuorum(t *testing.T) {
	runtime, state, _, _ := runtimeFixture(t)
	if err := runtime.AcceptProposal(runtimeMessage(state, "validator-a", MessageTypeProposal, "block-8")); err != nil {
		t.Fatal(err)
	}
	if err := runtime.AddVote(runtimeMessage(state, "validator-a", MessageTypeVote, "block-8")); err != nil {
		t.Fatal(err)
	}
	_, err := runtime.FinalizeProposal()
	if !errors.Is(err, ErrInvalidRuntimePhase) {
		t.Fatalf("expected invalid runtime phase, got %v", err)
	}
}
