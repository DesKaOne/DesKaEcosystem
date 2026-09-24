package consensus

import (
	"crypto/ed25519"
	"errors"
	"testing"
)

type timeoutTestSigner struct {
	privateKey ed25519.PrivateKey
}

func (s timeoutTestSigner) Sign(message []byte) ([]byte, error) {
	return ed25519.Sign(s.privateKey, message), nil
}

type timeoutTestAuthorityResolver struct {
	keys map[string]ed25519.PublicKey
}

func (r timeoutTestAuthorityResolver) PublicKeyForValidator(validatorID []byte) ([]byte, error) {
	key, ok := r.keys[string(validatorID)]
	if !ok {
		return nil, errors.New("unknown validator")
	}
	return append([]byte(nil), key...), nil
}

func newTimeoutTestSigner(t *testing.T) (timeoutTestSigner, ed25519.PublicKey) {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	return timeoutTestSigner{privateKey: privateKey}, publicKey
}

func timeoutMessageRules(state RoundState) ValidationRules {
	return ValidationRules{
		ProtocolVersion: state.ProtocolVersion,
		ChainID:         state.ChainID,
		MaxPayloadSize:  64,
	}
}

func TestTimeoutMessageAuthenticatesAndBuildsCertificate(t *testing.T) {
	_, state, validators, power := runtimeFixture(t)
	signerA, publicA := newTimeoutTestSigner(t)
	signerB, publicB := newTimeoutTestSigner(t)

	resolver := timeoutTestAuthorityResolver{keys: map[string]ed25519.PublicKey{
		"validator-a": publicA,
		"validator-b": publicB,
	}}
	rules := timeoutMessageRules(state)

	msgA, err := NewTimeoutMessage(state, []byte("validator-a"), state.Round+1, signerA)
	if err != nil {
		t.Fatal(err)
	}
	msgB, err := NewTimeoutMessage(state, []byte("validator-b"), state.Round+1, signerB)
	if err != nil {
		t.Fatal(err)
	}

	target, err := ValidateTimeoutMessage(msgA, state, validators, rules, resolver)
	if err != nil {
		t.Fatal(err)
	}
	if target != state.Round+1 {
		t.Fatalf("unexpected timeout target: %d", target)
	}

	certificate, err := NewTimeoutCertificateFromMessages(
		state,
		validators,
		power,
		QuorumThreshold{Numerator: 2, Denominator: 3},
		[]Message{msgB, msgA},
		rules,
		resolver,
	)
	if err != nil {
		t.Fatal(err)
	}
	if certificate.NextRound != state.Round+1 {
		t.Fatalf("unexpected certificate target round: %d", certificate.NextRound)
	}
	if len(certificate.Validators) != 2 {
		t.Fatalf("unexpected timeout validator count: %d", len(certificate.Validators))
	}
	if string(certificate.Validators[0]) != "validator-a" || string(certificate.Validators[1]) != "validator-b" {
		t.Fatalf("timeout validators are not canonical: %q", certificate.Validators)
	}
}

func TestTimeoutMessageRejectsTamperedPayloadBeforeCertificate(t *testing.T) {
	_, state, validators, power := runtimeFixture(t)
	signer, publicKey := newTimeoutTestSigner(t)
	resolver := timeoutTestAuthorityResolver{keys: map[string]ed25519.PublicKey{
		"validator-a": publicKey,
	}}
	rules := timeoutMessageRules(state)

	msg, err := NewTimeoutMessage(state, []byte("validator-a"), state.Round+1, signer)
	if err != nil {
		t.Fatal(err)
	}
	msg.Payload[7]++
	_, err = NewTimeoutCertificateFromMessages(
		state,
		validators,
		power,
		QuorumThreshold{Numerator: 2, Denominator: 3},
		[]Message{msg},
		rules,
		resolver,
	)
	if !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("expected invalid signature, got %v", err)
	}
}

func TestTimeoutMessageRejectsMismatchedTargetRound(t *testing.T) {
	_, state, validators, power := runtimeFixture(t)
	signerA, publicA := newTimeoutTestSigner(t)
	signerB, publicB := newTimeoutTestSigner(t)
	resolver := timeoutTestAuthorityResolver{keys: map[string]ed25519.PublicKey{
		"validator-a": publicA,
		"validator-b": publicB,
	}}
	rules := timeoutMessageRules(state)

	msgA, err := NewTimeoutMessage(state, []byte("validator-a"), state.Round+1, signerA)
	if err != nil {
		t.Fatal(err)
	}
	msgB, err := NewTimeoutMessage(state, []byte("validator-b"), state.Round+2, signerB)
	if err != nil {
		t.Fatal(err)
	}

	_, err = NewTimeoutCertificateFromMessages(
		state,
		validators,
		power,
		QuorumThreshold{Numerator: 2, Denominator: 3},
		[]Message{msgA, msgB},
		rules,
		resolver,
	)
	if !errors.Is(err, ErrTimeoutTargetRoundMismatch) {
		t.Fatalf("expected target round mismatch, got %v", err)
	}
}

func TestTimeoutMessageRejectsStaleTargetRound(t *testing.T) {
	_, state, validators, power := runtimeFixture(t)
	signer, publicKey := newTimeoutTestSigner(t)
	resolver := timeoutTestAuthorityResolver{keys: map[string]ed25519.PublicKey{
		"validator-a": publicKey,
	}}
	rules := timeoutMessageRules(state)

	msg := Message{
		ProtocolVersion: state.ProtocolVersion,
		ChainID:         state.ChainID,
		Epoch:           state.Epoch,
		Height:          state.Height,
		Round:           state.Round,
		Sender:          []byte("validator-a"),
		Type:            MessageTypeTimeout,
		Payload:         encodeTimeoutTargetRound(state.Round),
	}
	signed, err := msg.Sign(signer)
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewTimeoutCertificateFromMessages(
		state,
		validators,
		power,
		QuorumThreshold{Numerator: 2, Denominator: 3},
		[]Message{signed},
		rules,
		resolver,
	)
	if !errors.Is(err, ErrInvalidTimeoutRound) {
		t.Fatalf("expected stale timeout round error, got %v", err)
	}
}

