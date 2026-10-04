package consensus

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/rand"
	"errors"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/storage"
)

func evidenceState(t *testing.T) (RoundState, ValidatorSet) {
	t.Helper()
	state, err := NewRoundState(1, 1, 9, 3)
	if err != nil { t.Fatal(err) }
	validators, err := NewValidatorSet([][]byte{[]byte("validator-a")})
	if err != nil { t.Fatal(err) }
	return state, validators
}

func signedEvidenceMessage(t *testing.T, state RoundState, id []byte, typ MessageType, payload []byte, privateKey ed25519.PrivateKey) Message {
	t.Helper()
	msg := Message{
		ProtocolVersion: state.ProtocolVersion,
		ChainID: state.ChainID,
		Epoch: state.Epoch,
		Height: state.Height,
		Round: state.Round,
		Sender: append([]byte(nil), id...),
		Type: typ,
		Payload: append([]byte(nil), payload...),
	}
	msg.Signature = ed25519.Sign(privateKey, msg.SigningBytes())
	return msg
}

func testAuthority(t *testing.T, id []byte) (StaticValidatorAuthority, ed25519.PrivateKey) {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil { t.Fatal(err) }
	authority, err := NewStaticValidatorAuthority(map[string][]byte{string(id): publicKey})
	if err != nil { t.Fatal(err) }
	return authority, privateKey
}

func TestPersistAuthenticatedEvidenceIsIdempotentAndConflictSafe(t *testing.T) {
	state, validators := evidenceState(t)
	id := []byte("validator-a")
	authority, privateKey := testAuthority(t, id)
	msg := signedEvidenceMessage(t, state, id, MessageTypePrevote, []byte("proposal-hash"), privateKey)
	store := storage.NewMemoryConsensusEvidenceStore()

	if err := PersistAuthenticatedEvidence(store, msg, state, validators, authority); err != nil { t.Fatal(err) }
	if err := PersistAuthenticatedEvidence(store, msg, state, validators, authority); err != nil { t.Fatal(err) }

	conflicting := signedEvidenceMessage(t, state, id, MessageTypePrevote, []byte("different-proposal"), privateKey)
	if err := PersistAuthenticatedEvidence(store, conflicting, state, validators, authority); !errors.Is(err, ErrConflictingEvidence) {
		t.Fatalf("conflicting replay error = %v, want %v", err, ErrConflictingEvidence)
	}
}


func TestConsensusEvidencePersistenceKeyBindsPersistenceContext(t *testing.T) {
	state, validators := evidenceState(t)
	id := []byte("validator-a")
	authority, privateKey := testAuthority(t, id)
	msg := signedEvidenceMessage(t, state, id, MessageTypePrevote, []byte("proposal-hash"), privateKey)

	base := PersistenceContext{
		ProtocolVersion: uint64(state.ProtocolVersion),
		ChainID: uint64(state.ChainID),
		Epoch: state.Epoch,
		Height: uint64(state.Height),
		Round: state.Round,
		Phase: uint8(state.Phase),
		ValidatorAuthorityDigest: [32]byte{1},
		VotingPowerDigest: [32]byte{2},
		ThresholdNumerator: 2,
		ThresholdDenominator: 3,
		ProposerPolicy: "round-robin-v0-dev",
		ProposerPolicyVersion: "1",
	}
	first, err := ConsensusEvidencePersistenceKey(msg, PersistenceContextDigest(base))
	if err != nil { t.Fatal(err) }

	changed := base
	changed.VotingPowerDigest[0] ^= 0xff
	second, err := ConsensusEvidencePersistenceKey(msg, PersistenceContextDigest(changed))
	if err != nil { t.Fatal(err) }
	if first == second {
		t.Fatal("persistence context change must change durable evidence key")
	}

	store := storage.NewMemoryConsensusEvidenceStore()
	if err := PersistAuthenticatedEvidenceWithContext(store, msg, state, validators, authority, base); err != nil {
		t.Fatal(err)
	}
	badHeight := base
	badHeight.Height++
	if err := PersistAuthenticatedEvidenceWithContext(store, msg, state, validators, authority, badHeight); !errors.Is(err, ErrEvidencePersistenceContextMismatch) {
		t.Fatalf("context mismatch error = %v, want %v", err, ErrEvidencePersistenceContextMismatch)
	}
}


func TestRecoverAuthenticatedEvidenceWithContextRejectsLegacyAndMismatchedContext(t *testing.T) {
	state, validators := evidenceState(t)
	id := []byte("validator-a")
	authority, privateKey := testAuthority(t, id)
	msg := signedEvidenceMessage(t, state, id, MessageTypePrevote, []byte("proposal-hash"), privateKey)
	context := PersistenceContext{
		ProtocolVersion: uint64(state.ProtocolVersion),
		ChainID: uint64(state.ChainID),
		Epoch: state.Epoch,
		Height: uint64(state.Height),
		Round: state.Round,
		Phase: uint8(state.Phase),
		ValidatorAuthorityDigest: [32]byte{1},
		VotingPowerDigest: [32]byte{2},
		ThresholdNumerator: 2,
		ThresholdDenominator: 3,
		ProposerPolicy: "round-robin-v0-dev",
		ProposerPolicyVersion: "1",
	}
	store := storage.NewMemoryConsensusEvidenceStore()
	if err := PersistAuthenticatedEvidenceWithContext(store, msg, state, validators, authority, context); err != nil {
		t.Fatal(err)
	}
	mismatched := context
	mismatched.VotingPowerDigest[0] ^= 0xff
	if _, err := RecoverAuthenticatedEvidenceWithContext(store, state, validators, authority, mismatched); !errors.Is(err, ErrEvidencePersistenceContextMismatch) {
		t.Fatalf("mismatched context error = %v, want %v", err, ErrEvidencePersistenceContextMismatch)
	}
	if _, err := RecoverAuthenticatedEvidenceWithContext(store, state, validators, authority, context); err != nil {
		t.Fatal(err)
	}
}

func TestRecoverAuthenticatedEvidenceAllowsMultipleRoundsDeterministically(t *testing.T) {
	state, validators := evidenceState(t)
	id := []byte("validator-a")
	authority, privateKey := testAuthority(t, id)
	store := storage.NewMemoryConsensusEvidenceStore()

	msg0 := signedEvidenceMessage(t, state, id, MessageTypeProposal, []byte("p0"), privateKey)
	if err := PersistAuthenticatedEvidence(store, msg0, state, validators, authority); err != nil { t.Fatal(err) }

	state.Round = 2
	msg2 := signedEvidenceMessage(t, state, id, MessageTypePrecommit, []byte("p2"), privateKey)
	if err := PersistAuthenticatedEvidence(store, msg2, state, validators, authority); err != nil { t.Fatal(err) }

	state.Round = 0
	recovered, err := RecoverAuthenticatedEvidence(store, state, validators, authority)
	if err != nil { t.Fatal(err) }
	if len(recovered) != 2 || recovered[0].Round != 0 || recovered[1].Round != 2 {
		t.Fatalf("unexpected recovered evidence ordering: %+v", recovered)
	}
}

func TestRecoverAuthenticatedEvidenceRejectsCorruptRecord(t *testing.T) {
	state, validators := evidenceState(t)
	id := []byte("validator-a")
	authority, privateKey := testAuthority(t, id)
	msg := signedEvidenceMessage(t, state, id, MessageTypeProposal, []byte("proposal"), privateKey)
	key, err := ConsensusEvidenceKey(msg)
	if err != nil { t.Fatal(err) }
	store := storage.NewMemoryConsensusEvidenceStore()
	if err := store.PutConsensusEvidence(key, []byte("corrupt")); err != nil { t.Fatal(err) }
	if _, err := RecoverAuthenticatedEvidence(store, state, validators, authority); err == nil {
		t.Fatal("expected corrupt evidence rejection")
	}
}

func TestFileConsensusEvidenceStoreSurvivesReopen(t *testing.T) {
	path := t.TempDir() + "/consensus-evidence.gob"
	store, err := storage.NewFileConsensusEvidenceStore(path)
	if err != nil { t.Fatal(err) }
	if err := store.PutConsensusEvidence("key", []byte("evidence")); err != nil { t.Fatal(err) }
	reopened, err := storage.NewFileConsensusEvidenceStore(path)
	if err != nil { t.Fatal(err) }
	records, err := reopened.LoadConsensusEvidence()
	if err != nil { t.Fatal(err) }
	if string(records["key"]) != "evidence" { t.Fatalf("reopened evidence = %q", records["key"]) }
}


func authorityDigestFromFixture(authority StaticValidatorAuthority) [32]byte {
	// The context only needs a deterministic authority identity for this fixture.
	return sha256.Sum256([]byte("test-authority"))
}

func TestPersistAndRecoverFinalityCertificateWithContext(t *testing.T) {
	_, state, validators, power := runtimeFixture(t)
	f := newAuthenticatedRuntimeFixture(t)
	authority := f.resolver
	if err := f.runtime.AddVote(runtimeMessage(f.state, "validator-a", MessageTypePrevote, "persisted-finality")); err != nil { t.Fatal(err) }
	if err := f.runtime.AddVote(runtimeMessage(f.state, "validator-b", MessageTypePrevote, "persisted-finality")); err != nil { t.Fatal(err) }
	if err := f.runtime.AddVote(authenticatedPrecommit(t, f.runtime.State(), "validator-a", f.signerA, "persisted-finality")); err != nil { t.Fatal(err) }
	if err := f.runtime.AddVote(authenticatedPrecommit(t, f.runtime.State(), "validator-b", f.signerB, "persisted-finality")); err != nil { t.Fatal(err) }
	certificate, err := f.runtime.FinalizeProposal(authority)
	if err != nil { t.Fatal(err) }

	context := PersistenceContext{
		ProtocolVersion: uint64(state.ProtocolVersion),
		ChainID: uint64(state.ChainID),
		Epoch: state.Epoch,
		Height: uint64(state.Height),
		Round: certificate.Round,
		Phase: uint8(PhaseFinalized),
		ValidatorAuthorityDigest: authorityDigestFromFixture(authority),
		VotingPowerDigest: [32]byte{9},
		ThresholdNumerator: 2,
		ThresholdDenominator: 3,
		ProposerPolicy: "round-robin-v0-dev",
		ProposerPolicyVersion: "1",
	}
	store := storage.NewMemoryConsensusEvidenceStore()
	key, err := PersistFinalityCertificateWithContext(
		store, certificate, state, validators, power, authority, context, f.signerA, []byte("validator-a"),
	)
	if err != nil { t.Fatal(err) }

	recovered, recoveredKey, err := RecoverFinalityCertificateWithContext(
		store, state, validators, power, authority, context,
	)
	if err != nil { t.Fatal(err) }
	if recoveredKey != key { t.Fatalf("recovered key = %q, want %q", recoveredKey, key) }
	if string(recovered.Payload) != string(certificate.Payload) || len(recovered.Votes) != len(certificate.Votes) {
		t.Fatal("recovered finality certificate mismatch")
	}

	restarted, err := NewValidatorRuntime(RuntimeConfig{
		Rules: ValidationRules{ProtocolVersion: state.ProtocolVersion, ChainID: state.ChainID, RequireSender: true},
		State: state, Validators: validators, VotingPower: power,
		Threshold: QuorumThreshold{Numerator: 2, Denominator: 3}, Proposer: RoundRobinProposer{},
	})
	if err != nil { t.Fatal(err) }
	if err := restarted.RestoreFinalizedEvidence(recovered, authority); err != nil { t.Fatal(err) }
	if restarted.State().Phase != PhaseFinalized || string(restarted.Proposal()) != string(certificate.Payload) {
		t.Fatalf("restart restore state = %+v proposal=%q", restarted.State(), restarted.Proposal())
	}
}

func TestPersistFinalityCertificateWithContextRejectsContextChange(t *testing.T) {
	f := newAuthenticatedRuntimeFixture(t)
	if err := f.runtime.AddVote(runtimeMessage(f.state, "validator-a", MessageTypePrevote, "context-finality")); err != nil { t.Fatal(err) }
	if err := f.runtime.AddVote(runtimeMessage(f.state, "validator-b", MessageTypePrevote, "context-finality")); err != nil { t.Fatal(err) }
	if err := f.runtime.AddVote(authenticatedPrecommit(t, f.runtime.State(), "validator-a", f.signerA, "context-finality")); err != nil { t.Fatal(err) }
	if err := f.runtime.AddVote(authenticatedPrecommit(t, f.runtime.State(), "validator-b", f.signerB, "context-finality")); err != nil { t.Fatal(err) }
	certificate, err := f.runtime.FinalizeProposal(f.resolver)
	if err != nil { t.Fatal(err) }
	context := PersistenceContext{
		ProtocolVersion: uint64(f.state.ProtocolVersion), ChainID: uint64(f.state.ChainID),
		Epoch: f.state.Epoch, Height: uint64(f.state.Height), Round: certificate.Round,
		Phase: uint8(PhaseFinalized), ValidatorAuthorityDigest: [32]byte{1},
		VotingPowerDigest: [32]byte{2}, ThresholdNumerator: 2, ThresholdDenominator: 3,
		ProposerPolicy: "round-robin-v0-dev", ProposerPolicyVersion: "1",
	}
	mismatched := context
	mismatched.Height++
	_, err = PersistFinalityCertificateWithContext(
		storage.NewMemoryConsensusEvidenceStore(), certificate, f.state, f.validators, f.power, f.resolver,
		mismatched, f.signerA, []byte("validator-a"),
	)
	if !errors.Is(err, ErrEvidencePersistenceContextMismatch) {
		t.Fatalf("error = %v, want context mismatch", err)
	}
}
