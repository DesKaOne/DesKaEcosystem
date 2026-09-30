package consensus

import (
	"errors"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/state"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

type canonicalCommitTestStore struct {
	calls int
	block block.Block
	hash types.Hash
	state *state.State
	err   error
}

func (s *canonicalCommitTestStore) CommitBlockState(b block.Block, hash types.Hash, st *state.State) error {
	s.calls++
	s.block = b
	s.hash = hash
	if st != nil {
		s.state = st.Snapshot()
	}
	return s.err
}

func canonicalCommitCandidate(t *testing.T) CanonicalCommitCandidate {
	t.Helper()
	st := state.New()
	b := block.Block{Header: block.Header{Version: 1, ChainID: 1, Height: 1}}
	hash, err := block.Hash(b)
	if err != nil {
		t.Fatal(err)
	}
	return CanonicalCommitCandidate{Block: b, Hash: hash, State: st}
}

func TestValidateCanonicalCommitCandidateAcceptsMatchingCandidate(t *testing.T) {
	candidate := canonicalCommitCandidate(t)
	if err := ValidateCanonicalCommitCandidate(candidate); err != nil {
		t.Fatalf("validation error = %v", err)
	}
}

func TestValidateCanonicalCommitCandidateRejectsHashMismatch(t *testing.T) {
	candidate := canonicalCommitCandidate(t)
	candidate.Hash[0] ^= 0xff
	if err := ValidateCanonicalCommitCandidate(candidate); !errors.Is(err, ErrCanonicalCommitHashMismatch) {
		t.Fatalf("error = %v, want hash mismatch", err)
	}
}

func TestValidateCanonicalCommitCandidateRejectsStateRootMismatch(t *testing.T) {
	candidate := canonicalCommitCandidate(t)
	candidate.Block.Header.StateRoot[0] = 1
	var err error
	candidate.Hash, err = block.Hash(candidate.Block)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateCanonicalCommitCandidate(candidate); !errors.Is(err, ErrInvalidCanonicalCommit) {
		t.Fatalf("error = %v, want invalid canonical commit", err)
	}
}

func TestCommitCanonicalCandidateValidatesBeforeCallingStore(t *testing.T) {
	store := &canonicalCommitTestStore{}
	candidate := canonicalCommitCandidate(t)
	candidate.Hash[0] ^= 0xff

	if err := CommitCanonicalCandidate(store, candidate); !errors.Is(err, ErrCanonicalCommitHashMismatch) {
		t.Fatalf("error = %v, want hash mismatch", err)
	}
	if store.calls != 0 {
		t.Fatalf("store calls = %d, want 0", store.calls)
	}
}

func TestCommitCanonicalCandidateCallsStoreExactlyOnce(t *testing.T) {
	store := &canonicalCommitTestStore{}
	candidate := canonicalCommitCandidate(t)

	if err := CommitCanonicalCandidate(store, candidate); err != nil {
		t.Fatal(err)
	}
	if store.calls != 1 {
		t.Fatalf("store calls = %d, want 1", store.calls)
	}
	if store.hash != candidate.Hash || store.block.Header.Height != candidate.Block.Header.Height {
		t.Fatal("store received a different canonical candidate")
	}
}

func TestCommitCanonicalCandidateDoesNotPublishOnStoreFailure(t *testing.T) {
	errExpected := errors.New("store failure")
	store := &canonicalCommitTestStore{err: errExpected}
	candidate := canonicalCommitCandidate(t)

	err := CommitCanonicalCandidate(store, candidate)
	if !errors.Is(err, ErrCanonicalCommitFailed) {
		t.Fatalf("error = %v, want canonical commit failure", err)
	}
	if !errors.Is(err, errExpected) {
		t.Fatalf("wrapped error = %v, want %v", err, errExpected)
	}
	if store.calls != 1 {
		t.Fatalf("store calls = %d, want 1", store.calls)
	}
}

func TestCommitCanonicalCandidateRejectsNilStore(t *testing.T) {
	candidate := canonicalCommitCandidate(t)
	if err := CommitCanonicalCandidate(nil, candidate); !errors.Is(err, ErrNilCanonicalCommitter) {
		t.Fatalf("error = %v, want nil committer", err)
	}
}
