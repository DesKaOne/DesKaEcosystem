package consensus

import (
	"bytes"
	"crypto/ed25519"
	"errors"
	"testing"
)

type timeoutRuntimeAuthorityResolver struct {
	keys map[string]ed25519.PublicKey
}

func (r timeoutRuntimeAuthorityResolver) PublicKeyForValidator(validatorID []byte) ([]byte, error) {
	key, ok := r.keys[string(validatorID)]
	if !ok {
		return nil, errors.New("unknown validator")
	}
	return append([]byte(nil), key...), nil
}

func TestValidatorRuntimeAdvancesRoundWithSignedTimeoutEvidence(t *testing.T) {
	runtime, state, validators, power := runtimeFixture(t)
	signerA, publicA := newTimeoutTestSigner(t)
	signerB, publicB := newTimeoutTestSigner(t)

	resolver := timeoutRuntimeAuthorityResolver{keys: map[string]ed25519.PublicKey{
		"validator-a": publicA,
		"validator-b": publicB,
	}}

	msgA, err := NewTimeoutMessage(state, []byte("validator-a"), state.Round+1, signerA)
	if err != nil {
		t.Fatal(err)
	}
	msgB, err := NewTimeoutMessage(state, []byte("validator-b"), state.Round+1, signerB)
	if err != nil {
		t.Fatal(err)
	}

	certificate, err := runtime.AdvanceRoundWithTimeoutEvidence(
		[]Message{msgB, msgA},
		resolver,
	)
	if err != nil {
		t.Fatal(err)
	}
	if certificate.NextRound != state.Round+1 {
		t.Fatalf("unexpected timeout target: %d", certificate.NextRound)
	}
	if runtime.state.Round != state.Round+1 {
		t.Fatalf("runtime did not advance: %d", runtime.state.Round)
	}
	if runtime.state.Phase != PhaseProposal {
		t.Fatalf("runtime phase was not reset: %v", runtime.state.Phase)
	}
	if runtime.proposal != nil || runtime.certificate != nil {
		t.Fatal("round-local proposal/certificate state was not cleared")
	}
	if len(runtime.lockedProposal) != 0 {
		t.Fatal("unexpected lock in timeout fixture")
	}

	_ = validators
	_ = power
}

func TestValidatorRuntimeRejectsInsufficientTimeoutEvidenceWithoutMutation(t *testing.T) {
	runtime, state, validators, power := runtimeFixture(t)
	signer, publicKey := newTimeoutTestSigner(t)
	resolver := timeoutRuntimeAuthorityResolver{keys: map[string]ed25519.PublicKey{
		"validator-a": publicKey,
	}}

	msg, err := NewTimeoutMessage(state, []byte("validator-a"), state.Round+1, signer)
	if err != nil {
		t.Fatal(err)
	}

	before := runtime.state
	_, err = runtime.AdvanceRoundWithTimeoutEvidence([]Message{msg}, resolver)
	if !errors.Is(err, ErrTimeoutQuorumNotReached) {
		t.Fatalf("expected timeout quorum error, got %v", err)
	}
	if runtime.state != before {
		t.Fatal("runtime state mutated after rejected timeout evidence")
	}
}

func TestValidatorRuntimeRejectsTamperedTimeoutEvidenceWithoutMutation(t *testing.T) {
	runtime, state, validators, power := runtimeFixture(t)
	signer, publicKey := newTimeoutTestSigner(t)
	resolver := timeoutRuntimeAuthorityResolver{keys: map[string]ed25519.PublicKey{
		"validator-a": publicKey,
	}}

	msg, err := NewTimeoutMessage(state, []byte("validator-a"), state.Round+1, signer)
	if err != nil {
		t.Fatal(err)
	}
	msg.Payload[7]++

	before := runtime.state
	_, err = runtime.AdvanceRoundWithTimeoutEvidence([]Message{msg}, resolver)
	if !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("expected invalid signature error, got %v", err)
	}
	if runtime.state != before {
		t.Fatal("runtime state mutated after invalid timeout signature")
	}
}


func timeoutLockProofAtRound(t *testing.T, state RoundState, validators ValidatorSet, power VotingPowerSet, lockedRound uint64, proposal string) LockProof {
	t.Helper()
	lockState := state
	lockState.Round = lockedRound
	votes := []Message{
		runtimeMessage(lockState, "validator-a", MessageTypePrecommit, proposal),
		runtimeMessage(lockState, "validator-b", MessageTypePrecommit, proposal),
	}
	certificate, err := NewPrecommitCertificate(lockState, validators, power, QuorumThreshold{Numerator: 2, Denominator: 3}, []byte(proposal), votes)
	if err != nil { t.Fatal(err) }
	proof, err := NewLockProof(lockedRound, []byte(proposal), certificate)
	if err != nil { t.Fatal(err) }
	return proof
}

func TestValidatorRuntimeAdoptsTimeoutLockEvidenceAtomically(t *testing.T) {
	runtime, state, validators, power := runtimeFixture(t)
	signerA, publicA := newTimeoutTestSigner(t)
	signerB, publicB := newTimeoutTestSigner(t)
	resolver := timeoutRuntimeAuthorityResolver{keys: map[string]ed25519.PublicKey{
		"validator-a": publicA,
		"validator-b": publicB,
	}}
	proof := timeoutLockProof(t, state, validators, power, "locked-proposal")
	locked := []byte("locked-proposal")
	msgA, err := NewTimeoutMessageWithLockProof(state, []byte("validator-a"), state.Round+1, proof, signerA)
	if err != nil { t.Fatal(err) }
	msgB, err := NewTimeoutMessageWithLockProof(state, []byte("validator-b"), state.Round+1, proof, signerB)
	if err != nil { t.Fatal(err) }

	certificate, err := runtime.AdvanceRoundWithTimeoutEvidence([]Message{msgA, msgB}, resolver)
	if err != nil { t.Fatal(err) }
	if !bytes.Equal(certificate.LockedProposal, locked) { t.Fatalf("certificate lock mismatch: %q", certificate.LockedProposal) }
	if !bytes.Equal(runtime.lockedProposal, locked) { t.Fatalf("runtime did not adopt lock: %q", runtime.lockedProposal) }
}

func TestValidatorRuntimeRejectsTimeoutLockConflictWithoutMutation(t *testing.T) {
	runtime, state, validators, power := runtimeFixture(t)
	runtime.lockedProposal = []byte("local-lock")
	signerA, publicA := newTimeoutTestSigner(t)
	signerB, publicB := newTimeoutTestSigner(t)
	resolver := timeoutRuntimeAuthorityResolver{keys: map[string]ed25519.PublicKey{
		"validator-a": publicA,
		"validator-b": publicB,
	}}
	proof := timeoutLockProof(t, state, validators, power, "remote-lock")
	msgA, err := NewTimeoutMessageWithLockProof(state, []byte("validator-a"), state.Round+1, proof, signerA)
	if err != nil { t.Fatal(err) }
	msgB, err := NewTimeoutMessageWithLockProof(state, []byte("validator-b"), state.Round+1, proof, signerB)
	if err != nil { t.Fatal(err) }

	beforeState := runtime.state
	beforeLock := append([]byte(nil), runtime.lockedProposal...)
	_, err = runtime.AdvanceRoundWithTimeoutEvidence([]Message{msgA, msgB}, resolver)
	if !errors.Is(err, ErrConflictingTimeoutLock) { t.Fatalf("expected lock conflict, got %v", err) }
	if runtime.state != beforeState { t.Fatal("runtime state mutated after timeout lock conflict") }
	if !bytes.Equal(runtime.lockedProposal, beforeLock) { t.Fatal("runtime lock mutated after timeout lock conflict") }
}


func TestValidatorRuntimeAdoptsHigherTimeoutLockRound(t *testing.T) {
	runtime, state, _, _ := runtimeFixture(t)
	state.Round = 2
	runtime.state.Round = 2
	runtime.lockedProposal = []byte("locked-proposal")
	runtime.lockedRound = state.Round - 2
	signerA, publicA := newTimeoutTestSigner(t)
	signerB, publicB := newTimeoutTestSigner(t)
	resolver := timeoutRuntimeAuthorityResolver{keys: map[string]ed25519.PublicKey{
		"validator-a": publicA,
		"validator-b": publicB,
	}}
	lockedRound := state.Round - 1
	proof := timeoutLockProofAtRound(t, state, validators, power, lockedRound, "locked-proposal")
	msgA, err := NewTimeoutMessageWithLockProof(state, []byte("validator-a"), state.Round+2, proof, signerA)
	if err != nil { t.Fatal(err) }
	msgB, err := NewTimeoutMessageWithLockProof(state, []byte("validator-b"), state.Round+2, proof, signerB)
	if err != nil { t.Fatal(err) }

	_, err = runtime.AdvanceRoundWithTimeoutEvidence([]Message{msgA, msgB}, resolver)
	if err != nil { t.Fatal(err) }
	if runtime.lockedRound != lockedRound {
		t.Fatalf("runtime did not adopt higher lock round: %d", runtime.lockedRound)
	}
}

func TestValidatorRuntimeRejectsLowerTimeoutLockRoundWithoutDowngrade(t *testing.T) {
	runtime, state, _, _ := runtimeFixture(t)
	state.Round = 2
	runtime.state.Round = 2
	runtime.lockedProposal = []byte("locked-proposal")
	runtime.lockedRound = state.Round - 1
	signerA, publicA := newTimeoutTestSigner(t)
	signerB, publicB := newTimeoutTestSigner(t)
	resolver := timeoutRuntimeAuthorityResolver{keys: map[string]ed25519.PublicKey{
		"validator-a": publicA,
		"validator-b": publicB,
	}}
	proof := timeoutLockProofAtRound(t, state, validators, power, state.Round-2, "locked-proposal")
	msgA, err := NewTimeoutMessageWithLockProof(state, []byte("validator-a"), state.Round+2, proof, signerA)
	if err != nil { t.Fatal(err) }
	msgB, err := NewTimeoutMessageWithLockProof(state, []byte("validator-b"), state.Round+2, proof, signerB)
	if err != nil { t.Fatal(err) }

	_, err = runtime.AdvanceRoundWithTimeoutEvidence([]Message{msgA, msgB}, resolver)
	if err != nil { t.Fatal(err) }
	if runtime.lockedRound != state.Round-1 {
		t.Fatalf("runtime lock round was downgraded: %d", runtime.lockedRound)
	}
}
