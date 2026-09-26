package consensus

import (
	"bytes"
	"errors"
	"testing"
)

func precommitTestState(t *testing.T) RoundState {
	t.Helper()
	state, err := NewRoundState(1, 1001, 2, 7)
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func precommitTestValidators(t *testing.T) (ValidatorSet, VotingPowerSet) {
	t.Helper()
	validators, err := NewValidatorSet([][]byte{
		[]byte("validator-a"),
		[]byte("validator-b"),
		[]byte("validator-c"),
	})
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
	return validators, power
}

func precommitTestVote(state RoundState, sender string, payload []byte) Message {
	return Message{
		ProtocolVersion: state.ProtocolVersion,
		ChainID:         state.ChainID,
		Epoch:           state.Epoch,
		Height:          state.Height,
		Round:           state.Round,
		Sender:          []byte(sender),
		Type:            MessageTypePrecommit,
		Payload:         append([]byte(nil), payload...),
	}
}

func TestNewPrecommitCertificateRequiresExplicitPrecommitVotes(t *testing.T) {
	state := precommitTestState(t)
	validators, power := precommitTestValidators(t)
	payload := []byte("block-7")

	_, err := NewPrecommitCertificate(
		state, validators, power,
		QuorumThreshold{Numerator: 2, Denominator: 3},
		payload,
		[]Message{{
			ProtocolVersion: state.ProtocolVersion,
			ChainID:         state.ChainID,
			Epoch:           state.Epoch,
			Height:          state.Height,
			Round:           state.Round,
			Sender:          []byte("validator-a"),
			Type:            MessageTypeVote,
			Payload:         append([]byte(nil), payload...),
		}},
	)
	if !errors.Is(err, ErrInvalidPrecommitCertificate) {
		t.Fatalf("expected explicit precommit rejection, got %v", err)
	}
}

func TestPrecommitCertificateCanonicalizesAndValidates(t *testing.T) {
	state := precommitTestState(t)
	validators, power := precommitTestValidators(t)
	payload := []byte("block-7")

	certificate, err := NewPrecommitCertificate(
		state, validators, power,
		QuorumThreshold{Numerator: 2, Denominator: 3},
		payload,
		[]Message{
			precommitTestVote(state, "validator-b", payload),
			precommitTestVote(state, "validator-a", payload),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if string(certificate.Votes[0].Sender) != "validator-a" ||
		string(certificate.Votes[1].Sender) != "validator-b" {
		t.Fatalf("precommit votes are not canonical: %+v", certificate.Votes)
	}
	if err := ValidatePrecommitCertificate(certificate, state, validators, power); err != nil {
		t.Fatalf("expected certificate validation success, got %v", err)
	}
}

func TestValidatePrecommitCertificateRejectsNonCanonicalOrder(t *testing.T) {
	state := precommitTestState(t)
	validators, power := precommitTestValidators(t)
	payload := []byte("block-7")
	certificate := PrecommitCertificate{
		ProtocolVersion: state.ProtocolVersion,
		ChainID:         state.ChainID,
		Epoch:           state.Epoch,
		Height:          state.Height,
		Round:           state.Round,
		Payload:         append([]byte(nil), payload...),
		Threshold:       QuorumThreshold{Numerator: 2, Denominator: 3},
		Votes: []Message{
			precommitTestVote(state, "validator-b", payload),
			precommitTestVote(state, "validator-a", payload),
		},
	}

	if err := ValidatePrecommitCertificate(certificate, state, validators, power); !errors.Is(err, ErrInvalidPrecommitCertificate) {
		t.Fatalf("expected canonical-order rejection, got %v", err)
	}
}

func TestLockProofBindsProposalAndRoundAndClones(t *testing.T) {
	state := precommitTestState(t)
	validators, power := precommitTestValidators(t)
	payload := []byte("block-7")
	certificate, err := NewPrecommitCertificate(
		state, validators, power,
		QuorumThreshold{Numerator: 2, Denominator: 3},
		payload,
		[]Message{
			precommitTestVote(state, "validator-a", payload),
			precommitTestVote(state, "validator-b", payload),
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	proof, err := NewLockProof(state.Round, payload, certificate)
	if err != nil {
		t.Fatal(err)
	}
	payload[0] = 'X'
	certificate.Votes[0].Sender[0] = 'X'

	if !bytes.Equal(proof.Proposal, []byte("block-7")) {
		t.Fatal("lock proof proposal was not cloned")
	}
	if string(proof.Certificate.Votes[0].Sender) != "validator-a" {
		t.Fatal("lock proof certificate votes were not cloned")
	}
	if err := ValidateLockProof(proof, state, validators, power); err != nil {
		t.Fatalf("expected lock proof validation success, got %v", err)
	}
}

func TestNewLockProofRejectsMismatchedRoundOrProposal(t *testing.T) {
	state := precommitTestState(t)
	validators, power := precommitTestValidators(t)
	payload := []byte("block-7")
	certificate, err := NewPrecommitCertificate(
		state, validators, power,
		QuorumThreshold{Numerator: 2, Denominator: 3},
		payload,
		[]Message{
			precommitTestVote(state, "validator-a", payload),
			precommitTestVote(state, "validator-b", payload),
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := NewLockProof(state.Round+1, payload, certificate); !errors.Is(err, ErrInvalidLockProof) {
		t.Fatalf("expected round mismatch rejection, got %v", err)
	}
	if _, err := NewLockProof(state.Round, []byte("other-block"), certificate); !errors.Is(err, ErrInvalidLockProof) {
		t.Fatalf("expected proposal mismatch rejection, got %v", err)
	}
}
