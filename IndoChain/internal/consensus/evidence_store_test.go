package consensus

import (
	"crypto/ed25519"
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
