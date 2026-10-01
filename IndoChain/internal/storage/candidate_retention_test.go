package storage

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
)

func TestMemoryCandidateStoreDeleteIsIdempotent(t *testing.T) {
	store := NewMemoryCandidateStore()
	candidate := block.Block{Header: block.Header{Height: 1}}
	hash, err := block.Hash(candidate)
	if err != nil { t.Fatal(err) }
	key := CandidateKey{Height: 1, Hash: hash}
	if err := store.SaveCandidate(key, candidate); err != nil { t.Fatal(err) }
	if err := store.DeleteCandidate(key); err != nil { t.Fatal(err) }
	if err := store.DeleteCandidate(key); err != nil { t.Fatal(err) }
	if _, err := store.GetCandidate(key); !errors.Is(err, ErrCandidateNotFound) { t.Fatalf("expected candidate removal, got %v", err) }
}

func TestFileCandidateStoreDeletePersistsAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "candidate.gob")
	store, err := NewFileCandidateStore(path)
	if err != nil { t.Fatal(err) }
	candidate := block.Block{Header: block.Header{Height: 1, Timestamp: 42}}
	hash, err := block.Hash(candidate)
	if err != nil { t.Fatal(err) }
	key := CandidateKey{Height: 1, Hash: hash}
	if err := store.SaveCandidate(key, candidate); err != nil { t.Fatal(err) }
	if err := store.DeleteCandidate(key); err != nil { t.Fatal(err) }
	reopened, err := NewFileCandidateStore(path)
	if err != nil { t.Fatal(err) }
	if _, err := reopened.GetCandidate(key); !errors.Is(err, ErrCandidateNotFound) { t.Fatalf("expected deleted candidate after reopen, got %v", err) }
}
