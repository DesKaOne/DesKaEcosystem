package consensus

import (
	"bytes"
	"crypto/ed25519"
	"errors"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/crypto"
)

type evidenceAuthorityResolver struct {
	keys map[string][]byte
}

func (r evidenceAuthorityResolver) PublicKeyForValidator(validatorID []byte) ([]byte, error) {
	key, ok := r.keys[string(validatorID)]
	if !ok {
		return nil, errors.New("unknown validator")
	}
	return append([]byte(nil), key...), nil
}

func signedEvidenceVote(t *testing.T, state RoundState, validator string, seed byte, typ MessageType, payload []byte) (Message, []byte) {
	t.Helper()
	keyPair, err := crypto.NewEd25519KeyPair(bytes.Repeat([]byte{seed}, ed25519.SeedSize))
	if err != nil {
		t.Fatal(err)
	}
	msg := Message{
		ProtocolVersion: state.ProtocolVersion,
		ChainID:         state.ChainID,
		Epoch:           state.Epoch,
		Height:          state.Height,
		Round:           state.Round,
		Sender:          []byte(validator),
		Type:            typ,
		Payload:         append([]byte(nil), payload...),
	}
	signed, err := msg.Sign(keyPair)
	if err != nil {
		t.Fatal(err)
	}
	return signed, keyPair.PublicKey
}

func TestValidatePrecommitCertificateWithAuthority(t *testing.T) {
	runtime, state, validators, power := runtimeFixture(t)
	_ = runtime
	proposal := []byte("authenticated-proposal")
	voteA, publicA := signedEvidenceVote(t, state, "validator-a", 1, MessageTypePrecommit, proposal)
	voteB, publicB := signedEvidenceVote(t, state, "validator-b", 2, MessageTypePrecommit, proposal)
	certificate, err := NewPrecommitCertificate(
		state, validators, power, QuorumThreshold{Numerator: 2, Denominator: 3},
		proposal, []Message{voteB, voteA},
	)
	if err != nil {
		t.Fatal(err)
	}
	resolver := evidenceAuthorityResolver{keys: map[string][]byte{
		"validator-a": publicA,
		"validator-b": publicB,
	}}
	if err := ValidatePrecommitCertificateWithAuthority(certificate, state, validators, power, resolver); err != nil {
		t.Fatalf("authenticated precommit certificate rejected: %v", err)
	}

	certificate.Votes[0].Signature[0] ^= 0xff
	if err := ValidatePrecommitCertificateWithAuthority(certificate, state, validators, power, resolver); !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("expected invalid signature, got %v", err)
	}
}

func TestValidateLockProofWithAuthorityRejectsUnsignedPrecommitEvidence(t *testing.T) {
	runtime, state, validators, power := runtimeFixture(t)
	proposal := []byte("unsigned-proof")
	votes := []Message{
		runtimeMessage(state, "validator-a", MessageTypePrecommit, string(proposal)),
		runtimeMessage(state, "validator-b", MessageTypePrecommit, string(proposal)),
	}
	certificate, err := NewPrecommitCertificate(
		state, validators, power, QuorumThreshold{Numerator: 2, Denominator: 3},
		proposal, votes,
	)
	if err != nil {
		t.Fatal(err)
	}
	proof, err := NewLockProof(state.Round, proposal, certificate)
	if err != nil {
		t.Fatal(err)
	}
	resolver := evidenceAuthorityResolver{keys: map[string][]byte{}}
	if err := ValidateLockProofWithAuthority(proof, state, validators, power, resolver); !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("expected unsigned proof to fail authentication, got %v", err)
	}
	_ = runtime
}

func TestValidateFinalityCertificateWithAuthorityRequiresExplicitPrecommit(t *testing.T) {
	runtime, state, validators, power := runtimeFixture(t)
	proposal := []byte("finality-proposal")
	voteA, publicA := signedEvidenceVote(t, state, "validator-a", 3, MessageTypePrecommit, proposal)
	voteB, publicB := signedEvidenceVote(t, state, "validator-b", 4, MessageTypePrecommit, proposal)
	certificate, err := NewFinalityCertificate(
		state, validators, power, QuorumThreshold{Numerator: 2, Denominator: 3},
		proposal, []Message{voteA, voteB},
	)
	if err != nil {
		t.Fatal(err)
	}
	resolver := evidenceAuthorityResolver{keys: map[string][]byte{
		"validator-a": publicA,
		"validator-b": publicB,
	}}
	if err := ValidateFinalityCertificateWithAuthority(certificate, state, validators, power, resolver); err != nil {
		t.Fatalf("authenticated finality certificate rejected: %v", err)
	}

	legacy := certificate
	legacy.Votes = cloneVotes(certificate.Votes)
	legacy.Votes[0].Type = MessageTypePrevote
	if err := ValidateFinalityCertificateWithAuthority(legacy, state, validators, power, resolver); !errors.Is(err, ErrInvalidFinalityCertificate) {
		t.Fatalf("expected explicit-precommit rejection, got %v", err)
	}
	_ = runtime
}

func TestTimeoutCertificateRejectsUnprovenSignatureLockProof(t *testing.T) {
	runtime, state, validators, power := runtimeFixture(t)
	proposal := []byte("timeout-proof")
	unsignedVotes := []Message{
		runtimeMessage(state, "validator-a", MessageTypePrecommit, string(proposal)),
		runtimeMessage(state, "validator-b", MessageTypePrecommit, string(proposal)),
	}
	precommit, err := NewPrecommitCertificate(
		state, validators, power, QuorumThreshold{Numerator: 2, Denominator: 3},
		proposal, unsignedVotes,
	)
	if err != nil {
		t.Fatal(err)
	}
	proof, err := NewLockProof(state.Round, proposal, precommit)
	if err != nil {
		t.Fatal(err)
	}

	signerA, publicA := newTimeoutTestSigner(t)
	signerB, publicB := newTimeoutTestSigner(t)
	resolver := evidenceAuthorityResolver{keys: map[string][]byte{
		"validator-a": publicA,
		"validator-b": publicB,
	}}
	msgA, err := NewTimeoutMessageWithLockProof(state, []byte("validator-a"), state.Round+1, proof, signerA)
	if err != nil {
		t.Fatal(err)
	}
	msgB, err := NewTimeoutMessageWithLockProof(state, []byte("validator-b"), state.Round+1, proof, signerB)
	if err != nil {
		t.Fatal(err)
	}

	_, err = NewTimeoutCertificateFromMessages(
		state, validators, power, runtime.threshold,
		[]Message{msgA, msgB}, runtime.rules, resolver,
	)
	if !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("expected unsigned lock proof rejection, got %v", err)
	}
}
