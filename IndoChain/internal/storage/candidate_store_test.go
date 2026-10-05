package storage

import (
	"path/filepath"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

func candidateFixture(t *testing.T) (CandidateKey, block.Block) {
	t.Helper()
	b := block.Block{Header:block.Header{
		Version:1, ChainID:1, Height:7, Timestamp:42,
		PreviousHash:types.Hash{1}, TransactionsRoot:types.Hash{2},
		StateRoot:types.Hash{3}, Proposer:[]byte("validator-a"),
	}}
	h, err := block.Hash(b)
	if err != nil { t.Fatal(err) }
	return CandidateKey{Height:b.Header.Height, Hash:h}, b
}

func TestMemoryCandidateStoreValidatesIdentityAndClones(t *testing.T) {
	key, candidate := candidateFixture(t)
	store := NewMemoryCandidateStore()
	if err := store.SaveCandidate(key, candidate); err != nil { t.Fatal(err) }
	candidate.Header.Proposer[0] = 'x'
	got, err := store.GetCandidate(key)
	if err != nil { t.Fatal(err) }
	if string(got.Header.Proposer) != "validator-a" { t.Fatal("candidate was not cloned") }
	bad := key; bad.Height++
	if err := store.SaveCandidate(bad, got); err != ErrCandidateKeyMismatch { t.Fatalf("error=%v", err) }
}

func TestFileCandidateStoreSurvivesRestart(t *testing.T) {
	key, candidate := candidateFixture(t)
	path := filepath.Join(t.TempDir(), "candidates.bin")
	store, err := NewFileCandidateStore(path)
	if err != nil { t.Fatal(err) }
	if err := store.SaveCandidate(key, candidate); err != nil { t.Fatal(err) }

	reopened, err := NewFileCandidateStore(path)
	if err != nil { t.Fatal(err) }
	got, err := reopened.GetCandidate(key)
	if err != nil { t.Fatal(err) }
	if h, _ := block.Hash(got); h != key.Hash { t.Fatal("reopened candidate hash mismatch") }
}
