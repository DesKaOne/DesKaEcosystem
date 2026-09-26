package consensus

import (
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
	runtime, state, _, _ := runtimeFixture(t)
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
	runtime, state, _, _ := runtimeFixture(t)
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
