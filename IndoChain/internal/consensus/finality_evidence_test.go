package consensus

import (
	"bytes"
	"errors"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/crypto"
)

func TestBuildFinalityEvidenceFromAuthenticatedPrecommitQuorum(t *testing.T) {
	runtime, state, _, _ := runtimeFixture(t)
	authority := runtimeTestAuthority(t)

	if err := runtime.AcceptProposal(runtimeMessage(state, "validator-a", MessageTypeProposal, "block-8")); err != nil {
		t.Fatal(err)
	}
	if err := runtime.AddVote(runtimeMessage(state, "validator-a", MessageTypePrevote, "block-8")); err != nil {
		t.Fatal(err)
	}
	if err := runtime.AddVote(runtimeMessage(state, "validator-b", MessageTypePrevote, "block-8")); err != nil {
		t.Fatal(err)
	}

	for _, item := range []struct{ id string; seed byte }{{"validator-a", 0x31}, {"validator-b", 0x32}} {
		kp, err := crypto.NewEd25519KeyPair(bytes.Repeat([]byte{item.seed}, 32))
		if err != nil { t.Fatal(err) }
		signer, err := crypto.NewEd25519Signer(kp.PrivateKey)
		if err != nil { t.Fatal(err) }
		vote := runtimeMessage(runtime.State(), item.id, MessageTypePrecommit, "block-8")
		vote, err = vote.Sign(signer)
		if err != nil { t.Fatal(err) }
		if err := runtime.AddAuthenticatedVote(vote, authority); err != nil { t.Fatal(err) }
	}

	before := runtime.State()
	certificate, err := runtime.BuildFinalityEvidence(authority)
	if err != nil { t.Fatal(err) }
	if got := runtime.State(); got != before {
		t.Fatalf("building evidence mutated runtime: before=%+v after=%+v", before, got)
	}
	if !bytes.Equal(certificate.Payload, []byte("block-8")) {
		t.Fatalf("unexpected evidence payload %q", certificate.Payload)
	}

	encoded, err := EncodeFinalityCertificate(certificate)
	if err != nil { t.Fatal(err) }
	decoded, err := DecodeFinalityCertificate(encoded)
	if err != nil { t.Fatal(err) }
	if err := runtime.ValidateFinalityEvidence(decoded, authority); err != nil {
		t.Fatalf("decoded evidence failed validation: %v", err)
	}
}

func TestValidateFinalityEvidenceRejectsTamperedPrecommit(t *testing.T) {
	runtime, state, _, _ := runtimeFixture(t)
	authority := runtimeTestAuthority(t)
	if err := runtime.AcceptProposal(runtimeMessage(state, "validator-a", MessageTypeProposal, "block-8")); err != nil { t.Fatal(err) }
	if err := runtime.AddVote(runtimeMessage(state, "validator-a", MessageTypePrevote, "block-8")); err != nil { t.Fatal(err) }
	if err := runtime.AddVote(runtimeMessage(state, "validator-b", MessageTypePrevote, "block-8")); err != nil { t.Fatal(err) }

	for _, item := range []struct{ id string; seed byte }{{"validator-a", 0x31}, {"validator-b", 0x32}} {
		kp, err := crypto.NewEd25519KeyPair(bytes.Repeat([]byte{item.seed}, 32))
		if err != nil { t.Fatal(err) }
		signer, err := crypto.NewEd25519Signer(kp.PrivateKey)
		if err != nil { t.Fatal(err) }
		vote, err := runtimeMessage(runtime.State(), item.id, MessageTypePrecommit, "block-8").Sign(signer)
		if err != nil { t.Fatal(err) }
		if err := runtime.AddAuthenticatedVote(vote, authority); err != nil { t.Fatal(err) }
	}
	certificate, err := runtime.BuildFinalityEvidence(authority)
	if err != nil { t.Fatal(err) }
	certificate.Votes[0].Signature[0] ^= 0xff
	if err := runtime.ValidateFinalityEvidence(certificate, authority); err == nil {
		t.Fatal("expected tampered precommit signature rejection")
	}
}

func TestBuildFinalityEvidenceRequiresPrecommitQuorum(t *testing.T) {
	runtime, state, _, _ := runtimeFixture(t)
	authority := runtimeTestAuthority(t)
	if err := runtime.AcceptProposal(runtimeMessage(state, "validator-a", MessageTypeProposal, "block-8")); err != nil { t.Fatal(err) }
	if err := runtime.AddVote(runtimeMessage(state, "validator-a", MessageTypePrevote, "block-8")); err != nil { t.Fatal(err) }
	if err := runtime.AddVote(runtimeMessage(state, "validator-b", MessageTypePrevote, "block-8")); err != nil { t.Fatal(err) }

	kp, err := crypto.NewEd25519KeyPair(bytes.Repeat([]byte{0x31}, 32))
	if err != nil { t.Fatal(err) }
	signer, err := crypto.NewEd25519Signer(kp.PrivateKey)
	if err != nil { t.Fatal(err) }
	vote, err := runtimeMessage(runtime.State(), "validator-a", MessageTypePrecommit, "block-8").Sign(signer)
	if err != nil { t.Fatal(err) }
	if err := runtime.AddAuthenticatedVote(vote, authority); err != nil { t.Fatal(err) }

	_, err = runtime.BuildFinalityEvidence(authority)
	if !errors.Is(err, ErrPrecommitQuorumNotReached) {
		t.Fatalf("expected precommit quorum error, got %v", err)
	}
}
