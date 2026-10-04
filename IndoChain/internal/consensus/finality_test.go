package consensus

import (
	"bytes"
	"errors"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

func testFinalityContext(t *testing.T) RoundState {
	t.Helper()
	state, err := NewRoundState(1, 1001, 2, 7)
	if err != nil {
		t.Fatal(err)
	}
	state, err = state.AdvanceRound(3)
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func testFinalityValidators(t *testing.T) (ValidatorSet, VotingPowerSet) {
	t.Helper()
	validators, err := NewValidatorSet([][]byte{[]byte("validator-a"), []byte("validator-b"), []byte("validator-c")})
	if err != nil {
		t.Fatal(err)
	}
	votingPower, err := NewVotingPowerSet([]ValidatorVotingPower{
		{ValidatorID: []byte("validator-a"), Power: 4},
		{ValidatorID: []byte("validator-b"), Power: 3},
		{ValidatorID: []byte("validator-c"), Power: 3},
	})
	if err != nil {
		t.Fatal(err)
	}
	return validators, votingPower
}

func testFinalityVote(state RoundState, sender string, payload []byte) Message {
	return Message{
		ProtocolVersion: state.ProtocolVersion,
		ChainID:         state.ChainID,
		Epoch:           state.Epoch,
		Height:          state.Height,
		Round:           state.Round,
		Sender:          []byte(sender),
		Type:            MessageTypeVote,
		Payload:         append([]byte(nil), payload...),
	}
}

func TestNewFinalityCertificateRequiresQuorum(t *testing.T) {
	state := testFinalityContext(t)
	validators, votingPower := testFinalityValidators(t)
	payload := []byte("block-7")
	votes := []Message{
		testFinalityVote(state, "validator-a", payload),
	}

	_, err := NewFinalityCertificate(
		state,
		validators,
		votingPower,
		QuorumThreshold{Numerator: 2, Denominator: 3},
		payload,
		votes,
	)
	if !errors.Is(err, ErrFinalityQuorumNotReached) {
		t.Fatalf("expected quorum error, got %v", err)
	}
}

func TestFinalityCertificateRoundTripsAndClonesInputs(t *testing.T) {
	state := testFinalityContext(t)
	validators, votingPower := testFinalityValidators(t)
	payload := []byte("block-7")
	votes := []Message{
		testFinalityVote(state, "validator-a", payload),
		testFinalityVote(state, "validator-b", payload),
	}
	certificate, err := NewFinalityCertificate(
		state,
		validators,
		votingPower,
		QuorumThreshold{Numerator: 2, Denominator: 3},
		payload,
		votes,
	)
	if err != nil {
		t.Fatal(err)
	}

	payload[0] = 'X'
	votes[0].Sender[0] = 'X'

	if !bytes.Equal(certificate.Payload, []byte("block-7")) {
		t.Fatalf("certificate payload was not cloned")
	}
	if string(certificate.Votes[0].Sender) != "validator-a" {
		t.Fatalf("certificate vote sender was not cloned")
	}
	if err := ValidateFinalityCertificate(certificate, state, validators, votingPower); err != nil {
		t.Fatalf("expected certificate validation success, got %v", err)
	}
}

func TestValidateFinalityCertificateRejectsContextMismatch(t *testing.T) {
	state := testFinalityContext(t)
	validators, votingPower := testFinalityValidators(t)
	payload := []byte("block-7")
	certificate, err := NewFinalityCertificate(
		state,
		validators,
		votingPower,
		QuorumThreshold{Numerator: 2, Denominator: 3},
		payload,
		[]Message{
			testFinalityVote(state, "validator-a", payload),
			testFinalityVote(state, "validator-b", payload),
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	otherState := state
	otherState.Round++
	if err := ValidateFinalityCertificate(certificate, otherState, validators, votingPower); !errors.Is(err, ErrStateContextMismatch) {
		t.Fatalf("expected context mismatch, got %v", err)
	}
}

func TestValidateFinalityCertificateRejectsMixedPayloadVotes(t *testing.T) {
	state := testFinalityContext(t)
	validators, votingPower := testFinalityValidators(t)
	payload := []byte("block-7")
	certificate := FinalityCertificate{
		ProtocolVersion: state.ProtocolVersion,
		ChainID:         state.ChainID,
		Epoch:           state.Epoch,
		Height:          state.Height,
		Round:           state.Round,
		Payload:         payload,
		Threshold:       QuorumThreshold{Numerator: 2, Denominator: 3},
		Votes: []Message{
			testFinalityVote(state, "validator-a", payload),
			testFinalityVote(state, "validator-b", []byte("other-block")),
		},
	}

	if err := ValidateFinalityCertificate(certificate, state, validators, votingPower); !errors.Is(err, ErrFinalityQuorumNotReached) {
		t.Fatalf("expected quorum error for mixed payload votes, got %v", err)
	}
}

func TestFinalityCertificateRejectsDuplicateSender(t *testing.T) {
	state := testFinalityContext(t)
	validators, votingPower := testFinalityValidators(t)
	payload := []byte("block-7")
	_, err := NewFinalityCertificate(
		state,
		validators,
		votingPower,
		QuorumThreshold{Numerator: 2, Denominator: 3},
		payload,
		[]Message{
			testFinalityVote(state, "validator-a", payload),
			testFinalityVote(state, "validator-a", payload),
			testFinalityVote(state, "validator-b", payload),
		},
	)
	if !errors.Is(err, ErrDuplicateVote) {
		t.Fatalf("expected duplicate vote error, got %v", err)
	}
}

func TestFinalityCertificateDoesNotAdvanceRoundState(t *testing.T) {
	state := testFinalityContext(t)
	validators, votingPower := testFinalityValidators(t)
	payload := []byte("block-7")
	certificate, err := NewFinalityCertificate(
		state,
		validators,
		votingPower,
		QuorumThreshold{Numerator: 2, Denominator: 3},
		payload,
		[]Message{
			testFinalityVote(state, "validator-a", payload),
			testFinalityVote(state, "validator-b", payload),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateFinalityCertificate(certificate, state, validators, votingPower); err != nil {
		t.Fatal(err)
	}
	if state.Round != 3 || state.Phase != PhaseProposal || state.Height != types.Height(7) {
		t.Fatalf("certificate validation mutated round state: %+v", state)
	}
}
