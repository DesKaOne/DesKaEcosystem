package consensus

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/storage"
)

type evidenceTestSigner struct {
	key ed25519.PrivateKey
}

func (s evidenceTestSigner) Sign(message []byte) ([]byte, error) {
	return s.key.Sign(nil, message, cryptoHashOptions{})
}

// cryptoHashOptions is a zero-value placeholder because ed25519.PrivateKey.Sign
// accepts a crypto.SignerOpts argument; the development tests do not need a
// pre-hash mode.
type cryptoHashOptions struct{}

func (cryptoHashOptions) HashFunc() cryptoHash {
	return cryptoHash(0)
}

type cryptoHash uint

func signedEvidenceMessage(t *testing.T, state RoundState, id []byte, typ MessageType, payload []byte) (Message, StaticValidatorAuthority) {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil { t.Fatal(err) }
	authority, err := NewStaticValidatorAuthority(map[string][]byte{string(id): publicKey})
	if err != nil { t.Fatal(err) }
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
	return msg, authority
}

func evidenceState(t *testing.T) (RoundState, ValidatorSet) {
	t.Helper()
	state, err := NewRoundState(1, 1, 9, 3)
	if err != nil { t.Fatal(err) }
	validators, err := NewValidatorSet([][]byte{[]byte("validator-a")})
	if err != nil { t.Fatal(err) }
	return state, validators
}

func TestPersistAuthenticatedEvidenceIsIdempotentAndConflictSafe(t *testing.T) {
	state, validators := evidenceState(t)
	msg, authority := signedEvidenceMessage(t, state, []byte("validator-a"), MessageTypePrevote, []byte("proposal-hash"))
	store := storage.NewMemoryConsensusEvidenceStore()

	if err := PersistAuthenticatedEvidence(store, msg, state, validators, authority); err != nil { t.Fatal(err) }
	if err := PersistAuthenticatedEvidence(store, msg, state, validators, authority); err != nil { t.Fatal(err) }

	conflicting := msg
	conflicting.Payload = []byte("different-proposal")
	conflicting.Signature = ed25519.Sign(mustPrivateKeyForTest(t), conflicting.SigningBytes())
	if err := PersistAuthenticatedEvidence(store, conflicting, state, validators, authority); !errors.Is(err, ErrInvalidSignature) && !errors.Is(err, ErrConflictingEvidence) {
		t.Fatalf("conflicting replay error = %v", err)
	}
}

func TestRecoverAuthenticatedEvidenceAllowsMultipleRoundsDeterministically(t *testing.T) {
	state, validators := evidenceState(t)
	store := storage.NewMemoryConsensusEvidenceStore()
	ids := [][]byte{[]byte("validator-a")}
	msg0, authority := signedEvidenceMessage(t, state, ids[0], MessageTypeProposal, []byte("p0"))
	if err := PersistAuthenticatedEvidence(store, msg0, state, validators, authority); err != nil { t.Fatal(err) }

	state.Round = 2
	msg2, _ := signedEvidenceMessage(t, state, ids[0], MessageTypePrecommit, []byte("p2"))
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
	msg, authority := signedEvidenceMessage(t, state, []byte("validator-a"), MessageTypeProposal, []byte("proposal"))
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

// mustPrivateKeyForTest exists only to generate a different signature for the
// conflict-path test; the authority intentionally does not contain this key.
func mustPrivateKeyForTest(t *testing.T) ed25519.PrivateKey {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil { t.Fatal(err) }
	return privateKey
}
