package consensus

import (
	"bytes"
	"errors"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/crypto"
)

type authenticatedRuntimeFixture struct {
	runtime   *ValidatorRuntime
	state     RoundState
	resolver  StaticValidatorAuthority
	signerA   *crypto.Ed25519Signer
	signerB   *crypto.Ed25519Signer
	publicA   []byte
	publicB   []byte
	validators ValidatorSet
	power     VotingPowerSet
}

func newAuthenticatedRuntimeFixture(t *testing.T) authenticatedRuntimeFixture {
	t.Helper()
	state, err := NewRoundState(1, 1001, 1, 8)
	if err != nil { t.Fatal(err) }
	validators, err := NewValidatorSet([][]byte{[]byte("validator-a"), []byte("validator-b")})
	if err != nil { t.Fatal(err) }
	power, err := NewVotingPowerSet([]ValidatorVotingPower{
		{ValidatorID: []byte("validator-a"), Power: 1},
		{ValidatorID: []byte("validator-b"), Power: 1},
	})
	if err != nil { t.Fatal(err) }
	kpA, err := crypto.NewEd25519KeyPair(bytes.Repeat([]byte{0x41}, 32)); if err != nil { t.Fatal(err) }
	kpB, err := crypto.NewEd25519KeyPair(bytes.Repeat([]byte{0x42}, 32)); if err != nil { t.Fatal(err) }
	signerA, err := crypto.NewEd25519Signer(kpA.PrivateKey); if err != nil { t.Fatal(err) }
	signerB, err := crypto.NewEd25519Signer(kpB.PrivateKey); if err != nil { t.Fatal(err) }
	resolver, err := NewStaticValidatorAuthority(map[string][]byte{
		"validator-a": kpA.PublicKey, "validator-b": kpB.PublicKey,
	})
	if err != nil { t.Fatal(err) }
	runtime, err := NewValidatorRuntime(RuntimeConfig{
		Rules: ValidationRules{ProtocolVersion: state.ProtocolVersion, ChainID: state.ChainID, RequireSender: true},
		State: state, Validators: validators, VotingPower: power,
		Threshold: QuorumThreshold{Numerator: 2, Denominator: 3},
		Proposer: RoundRobinProposer{},
	})
	if err != nil { t.Fatal(err) }
	if err := runtime.AcceptProposal(runtimeMessage(state, "validator-a", MessageTypeProposal, "authenticated-block")); err != nil { t.Fatal(err) }
	return authenticatedRuntimeFixture{
		runtime: runtime, state: state, resolver: resolver,
		signerA: signerA, signerB: signerB, publicA: append([]byte(nil), kpA.PublicKey...), publicB: append([]byte(nil), kpB.PublicKey...),
		validators: validators, power: power,
	}
}

func authenticatedPrecommit(t *testing.T, state RoundState, sender string, signer crypto.Signer, payload string) Message {
	t.Helper()
	msg := runtimeMessage(state, sender, MessageTypePrecommit, payload)
	signed, err := msg.Sign(signer)
	if err != nil { t.Fatal(err) }
	return signed
}

func TestValidatorRuntimeAuthenticatedFinalizationSuccess(t *testing.T) {
	f := newAuthenticatedRuntimeFixture(t)
	if err := f.runtime.AddVote(runtimeMessage(f.state, "validator-a", MessageTypePrevote, "authenticated-block")); err != nil { t.Fatal(err) }
	if err := f.runtime.AddVote(runtimeMessage(f.state, "validator-b", MessageTypePrevote, "authenticated-block")); err != nil { t.Fatal(err) }
	if err := f.runtime.AddVote(authenticatedPrecommit(t, f.runtime.State(), "validator-a", f.signerA, "authenticated-block")); err != nil { t.Fatal(err) }
	if err := f.runtime.AddVote(authenticatedPrecommit(t, f.runtime.State(), "validator-b", f.signerB, "authenticated-block")); err != nil { t.Fatal(err) }
	certificate, err := f.runtime.FinalizeProposal(f.resolver)
	if err != nil { t.Fatal(err) }
	if f.runtime.State().Phase != PhaseFinalized || len(certificate.Votes) != 2 { t.Fatalf("runtime did not finalize authenticated evidence") }
}

func TestValidatorRuntimeInvalidPrecommitSignatureLeavesStateUnchanged(t *testing.T) {
	f := newAuthenticatedRuntimeFixture(t)
	if err := f.runtime.AddVote(runtimeMessage(f.state, "validator-a", MessageTypePrevote, "authenticated-block")); err != nil { t.Fatal(err) }
	if err := f.runtime.AddVote(runtimeMessage(f.state, "validator-b", MessageTypePrevote, "authenticated-block")); err != nil { t.Fatal(err) }
	voteA := authenticatedPrecommit(t, f.runtime.State(), "validator-a", f.signerA, "authenticated-block")
	voteB := authenticatedPrecommit(t, f.runtime.State(), "validator-b", f.signerB, "authenticated-block")
	voteB.Signature[0] ^= 0xff
	if err := f.runtime.AddVote(voteA); err != nil { t.Fatal(err) }
	if err := f.runtime.AddVote(voteB); err != nil { t.Fatal(err) }
	before := f.runtime.State()
	_, err := f.runtime.FinalizeProposal(f.resolver)
	if !errors.Is(err, ErrInvalidSignature) { t.Fatalf("error = %v, want invalid signature", err) }
	if got := f.runtime.State(); got != before { t.Fatalf("runtime state mutated after invalid signature: before=%+v after=%+v", before, got) }
}

func TestValidatorRuntimeUnauthorizedValidatorLeavesStateUnchanged(t *testing.T) {
	f := newAuthenticatedRuntimeFixture(t)
	if err := f.runtime.AddVote(runtimeMessage(f.state, "validator-a", MessageTypePrevote, "authenticated-block")); err != nil { t.Fatal(err) }
	if err := f.runtime.AddVote(runtimeMessage(f.state, "validator-b", MessageTypePrevote, "authenticated-block")); err != nil { t.Fatal(err) }
	unauthorized := runtimeMessage(f.runtime.State(), "validator-c", MessageTypePrecommit, "authenticated-block")
	before := f.runtime.State()
	if err := f.runtime.AddVote(unauthorized); !errors.Is(err, ErrConsensusMessageUnauthorized) {
		t.Fatalf("error = %v, want unauthorized validator", err)
	}
	if got := f.runtime.State(); got != before { t.Fatalf("state mutated after unauthorized validator: before=%+v after=%+v", before, got) }
}

func TestValidatorRuntimeMissingPublicKeyRejectsFinality(t *testing.T) {
	f := newAuthenticatedRuntimeFixture(t)
	missing, err := NewStaticValidatorAuthority(map[string][]byte{"validator-a": f.publicA})
	if err != nil { t.Fatal(err) }
	if err := f.runtime.AddVote(runtimeMessage(f.state, "validator-a", MessageTypePrevote, "authenticated-block")); err != nil { t.Fatal(err) }
	if err := f.runtime.AddVote(runtimeMessage(f.state, "validator-b", MessageTypePrevote, "authenticated-block")); err != nil { t.Fatal(err) }
	if err := f.runtime.AddVote(authenticatedPrecommit(t, f.runtime.State(), "validator-a", f.signerA, "authenticated-block")); err != nil { t.Fatal(err) }
	if err := f.runtime.AddVote(authenticatedPrecommit(t, f.runtime.State(), "validator-b", f.signerB, "authenticated-block")); err != nil { t.Fatal(err) }
	before := f.runtime.State()
	_, err = f.runtime.FinalizeProposal(missing)
	if !errors.Is(err, ErrConsensusAuthorityMissing) { t.Fatalf("error = %v, want missing authority", err) }
	if got := f.runtime.State(); got != before { t.Fatal("runtime mutated after missing public key") }
}

func TestValidatorRuntimeDuplicatePrecommitLeavesStateUnchanged(t *testing.T) {
	f := newAuthenticatedRuntimeFixture(t)
	if err := f.runtime.AddVote(runtimeMessage(f.state, "validator-a", MessageTypePrevote, "authenticated-block")); err != nil { t.Fatal(err) }
	if err := f.runtime.AddVote(runtimeMessage(f.state, "validator-b", MessageTypePrevote, "authenticated-block")); err != nil { t.Fatal(err) }
	vote := authenticatedPrecommit(t, f.runtime.State(), "validator-a", f.signerA, "authenticated-block")
	if err := f.runtime.AddVote(vote); err != nil { t.Fatal(err) }
	before := f.runtime.State()
	if err := f.runtime.AddVote(vote); !errors.Is(err, ErrDuplicateVote) { t.Fatalf("error = %v, want duplicate vote", err) }
	if got := f.runtime.State(); got != before { t.Fatal("runtime mutated after duplicate precommit") }
}

func TestValidatorRuntimeLegacyVoteCannotFinalizeAuthenticatedEvidence(t *testing.T) {
	f := newAuthenticatedRuntimeFixture(t)
	if err := f.runtime.AddVote(runtimeMessage(f.state, "validator-a", MessageTypeVote, "authenticated-block")); err != nil { t.Fatal(err) }
	if err := f.runtime.AddVote(runtimeMessage(f.state, "validator-b", MessageTypeVote, "authenticated-block")); err != nil { t.Fatal(err) }
	before := f.runtime.State()
	_, err := f.runtime.FinalizeProposal(f.resolver)
	if err == nil { t.Fatal("legacy vote unexpectedly produced authenticated finality") }
	if got := f.runtime.State(); got != before { t.Fatal("runtime mutated after legacy finality rejection") }
}

func TestValidatorRuntimePrevoteCannotFinalize(t *testing.T) {
	f := newAuthenticatedRuntimeFixture(t)
	if err := f.runtime.AddVote(runtimeMessage(f.state, "validator-a", MessageTypePrevote, "authenticated-block")); err != nil { t.Fatal(err) }
	if err := f.runtime.AddVote(runtimeMessage(f.state, "validator-b", MessageTypePrevote, "authenticated-block")); err != nil { t.Fatal(err) }
	before := f.runtime.State()
	_, err := f.runtime.FinalizeProposal(f.resolver)
	if err == nil { t.Fatal("prevote quorum unexpectedly finalized") }
	if got := f.runtime.State(); got != before { t.Fatal("runtime mutated after prevote-only finality rejection") }
}

func TestValidatorRuntimeWrongPrecommitSignatureDoesNotFinalize(t *testing.T) {
	f := newAuthenticatedRuntimeFixture(t)
	if err := f.runtime.AddVote(runtimeMessage(f.state, "validator-a", MessageTypePrevote, "authenticated-block")); err != nil { t.Fatal(err) }
	if err := f.runtime.AddVote(runtimeMessage(f.state, "validator-b", MessageTypePrevote, "authenticated-block")); err != nil { t.Fatal(err) }
	wrong := authenticatedPrecommit(t, f.runtime.State(), "validator-b", f.signerA, "authenticated-block")
	if err := f.runtime.AddVote(authenticatedPrecommit(t, f.runtime.State(), "validator-a", f.signerA, "authenticated-block")); err != nil { t.Fatal(err) }
	if err := f.runtime.AddVote(wrong); err != nil { t.Fatal(err) }
	before := f.runtime.State()
	_, err := f.runtime.FinalizeProposal(f.resolver)
	if !errors.Is(err, ErrInvalidSignature) { t.Fatalf("error = %v, want invalid signature", err) }
	if got := f.runtime.State(); got != before { t.Fatal("runtime mutated after wrong signature rejection") }
}

func TestAuthenticatedFinalityCertificateTamperingRejected(t *testing.T) {
	f := newAuthenticatedRuntimeFixture(t)
	votes := []Message{
		authenticatedPrecommit(t, f.state, "validator-a", f.signerA, "authenticated-block"),
		authenticatedPrecommit(t, f.state, "validator-b", f.signerB, "authenticated-block"),
	}
	certificate, err := NewFinalityCertificate(f.state, f.validators, f.power, QuorumThreshold{Numerator: 2, Denominator: 3}, []byte("authenticated-block"), votes)
	if err != nil { t.Fatal(err) }
	certificate.Votes[0].Signature[0] ^= 0xff
	before := f.state
	if err := ValidateFinalityCertificateWithAuthority(certificate, f.state, f.validators, f.power, f.resolver); !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("error = %v, want invalid signature", err)
	}
	if f.state != before { t.Fatal("state mutated during tampered certificate validation") }
}

func TestStaticValidatorAuthorityDefensiveCopy(t *testing.T) {
	kp, err := crypto.NewEd25519KeyPair(bytes.Repeat([]byte{0x51}, 32)); if err != nil { t.Fatal(err) }
	input := map[string][]byte{"validator-a": append([]byte(nil), kp.PublicKey...)}
	authority, err := NewStaticValidatorAuthority(input); if err != nil { t.Fatal(err) }
	input["validator-a"][0] ^= 0xff
	resolved, err := authority.PublicKeyForValidator([]byte("validator-a")); if err != nil { t.Fatal(err) }
	if bytes.Equal(resolved, input["validator-a"]) { t.Fatal("authority retained mutable input alias") }
	resolved[0] ^= 0xff
	again, err := authority.PublicKeyForValidator([]byte("validator-a")); if err != nil { t.Fatal(err) }
	if bytes.Equal(resolved, again) { t.Fatal("authority returned mutable internal key alias") }
}

func TestValidatorRuntimeFinalityFailureIsAtomic(t *testing.T) {
	f := newAuthenticatedRuntimeFixture(t)
	if err := f.runtime.AddVote(runtimeMessage(f.state, "validator-a", MessageTypePrevote, "authenticated-block")); err != nil { t.Fatal(err) }
	if err := f.runtime.AddVote(runtimeMessage(f.state, "validator-b", MessageTypePrevote, "authenticated-block")); err != nil { t.Fatal(err) }
	invalid := authenticatedPrecommit(t, f.runtime.State(), "validator-b", f.signerB, "authenticated-block")
	invalid.Signature[0] ^= 0xff
	if err := f.runtime.AddVote(authenticatedPrecommit(t, f.runtime.State(), "validator-a", f.signerA, "authenticated-block")); err != nil { t.Fatal(err) }
	if err := f.runtime.AddVote(invalid); err != nil { t.Fatal(err) }
	before := f.runtime.State()
	_, err := f.runtime.FinalizeProposal(f.resolver)
	if err == nil { t.Fatal("invalid authenticated evidence unexpectedly finalized") }
	if got := f.runtime.State(); got != before { t.Fatal("runtime state changed on authenticated finality failure") }
}
