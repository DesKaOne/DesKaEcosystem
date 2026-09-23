package consensus

import (
	"bytes"
	"errors"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/crypto"
)

func TestValidateConsensusMessageAcceptsValidMessage(t *testing.T) {
	state, err := NewRoundState(1, 1001, 2, 7)
	if err != nil {
		t.Fatal(err)
	}
	validators, err := NewValidatorSet([][]byte{[]byte("validator-a"), []byte("validator-b")})
	if err != nil {
		t.Fatal(err)
	}

	msg := Message{
		ProtocolVersion: 1,
		ChainID:        1001,
		Epoch:          2,
		Height:         7,
		Round:          0,
		Sender:         []byte("validator-a"),
		Type:           MessageTypeProposal,
		Payload:        []byte("proposal"),
		Signature:      []byte("development-signature"),
	}
	ctx := MessageValidationContext{
		Rules: ValidationRules{
			ProtocolVersion: 1,
			ChainID:         1001,
			MaxPayloadSize:  1024,
			RequireSender:   true,
			RequireSignature:true,
		},
		State:      state,
		Validators: validators,
	}

	if err := ValidateConsensusMessage(msg, ctx); err != nil {
		t.Fatalf("expected valid message, got %v", err)
	}
}

func TestValidateConsensusMessageRejectsContextMismatch(t *testing.T) {
	state, err := NewRoundState(1, 1001, 2, 7)
	if err != nil {
		t.Fatal(err)
	}
	validators, err := NewValidatorSet([][]byte{[]byte("validator-a")})
	if err != nil {
		t.Fatal(err)
	}

	msg := Message{
		ProtocolVersion: 1,
		ChainID:        1001,
		Epoch:          2,
		Height:         7,
		Round:          1,
		Sender:         []byte("validator-a"),
		Type:           MessageTypeVote,
		Signature:      []byte("sig"),
	}
	ctx := MessageValidationContext{
		Rules:      ValidationRules{ProtocolVersion: 1, ChainID: 1001, RequireSender: true, RequireSignature: true},
		State:      state,
		Validators: validators,
	}

	if err := ValidateConsensusMessage(msg, ctx); !errors.Is(err, ErrConsensusMessageContextMismatch) {
		t.Fatalf("expected context mismatch, got %v", err)
	}
}

func TestValidateConsensusMessageRejectsUnauthorizedSender(t *testing.T) {
	state, err := NewRoundState(1, 1001, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	validators, err := NewValidatorSet([][]byte{[]byte("validator-a")})
	if err != nil {
		t.Fatal(err)
	}

	msg := Message{
		ProtocolVersion: 1,
		ChainID:        1001,
		Sender:         []byte("validator-x"),
		Type:           MessageTypeVote,
		Signature:      []byte("sig"),
	}
	ctx := MessageValidationContext{
		Rules:      ValidationRules{ProtocolVersion: 1, ChainID: 1001, RequireSender: true, RequireSignature: true},
		State:      state,
		Validators: validators,
	}

	if err := ValidateConsensusMessage(msg, ctx); !errors.Is(err, ErrConsensusMessageUnauthorized) {
		t.Fatalf("expected unauthorized sender, got %v", err)
	}
}

func TestValidateConsensusMessagePreservesInput(t *testing.T) {
	state, err := NewRoundState(1, 1001, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	validators, err := NewValidatorSet([][]byte{[]byte("validator-a")})
	if err != nil {
		t.Fatal(err)
	}
	originalSender := []byte("validator-a")
	msg := Message{
		ProtocolVersion: 1,
		ChainID:        1001,
		Sender:         append([]byte(nil), originalSender...),
		Type:           MessageTypeVote,
		Signature:      []byte("sig"),
	}
	beforeSender := append([]byte(nil), msg.Sender...)
	ctx := MessageValidationContext{
		Rules:      ValidationRules{ProtocolVersion: 1, ChainID: 1001, RequireSender: true, RequireSignature: true},
		State:      state,
		Validators: validators,
	}

	if err := ValidateConsensusMessage(msg, ctx); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(msg.Sender, beforeSender) {
		t.Fatal("message sender mutated")
	}
	if !bytes.Equal(validators.Validators[0], originalSender) {
		t.Fatal("validator set mutated")
	}
}

func TestValidateConsensusMessageCanFollowWithSignatureVerification(t *testing.T) {
	keyPair, err := crypto.NewEd25519KeyPair(bytes.Repeat([]byte{0x42}, 32))
	if err != nil {
		t.Fatal(err)
	}
	signer, err := crypto.NewEd25519Signer(keyPair.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	state, err := NewRoundState(1, 1001, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	validators, err := NewValidatorSet([][]byte{keyPair.PublicKey})
	if err != nil {
		t.Fatal(err)
	}
	msg := Message{
		ProtocolVersion: 1,
		ChainID:        1001,
		Sender:         append([]byte(nil), keyPair.PublicKey...),
		Type:           MessageTypeProposal,
		Payload:        []byte("proposal"),
	}
	msg, err = msg.Sign(signer)
	if err != nil {
		t.Fatal(err)
	}
	ctx := MessageValidationContext{
		Rules: ValidationRules{
			ProtocolVersion: 1,
			ChainID:        1001,
			MaxPayloadSize: 1024,
			RequireSender: true,
			RequireSignature: true,
		},
		State: state,
		Validators: validators,
	}
	if err := ValidateConsensusMessage(msg, ctx); err != nil {
		t.Fatal(err)
	}
	if err := VerifyMessageSignature(msg, keyPair.PublicKey); err != nil {
		t.Fatalf("expected signature verification to remain a separate boundary: %v", err)
	}
}
