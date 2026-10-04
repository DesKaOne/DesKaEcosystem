package storage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/state"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

func TestFileStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "chain.gob")
	store, err := NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}

	st := state.New()
	st.Set(types.Address("alice"), state.Account{Balance: 42, Nonce: 3})
	b := block.Block{Header: block.Header{Height: 7, StateRoot: st.Root()}}
	hash := types.Hash{7}

	if err := store.CommitBlockState(b, hash, st); err != nil {
		t.Fatal(err)
	}

	reopened, err := NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	gotBlock, gotHash, err := reopened.Head()
	if err != nil {
		t.Fatal(err)
	}
	if gotBlock.Header.Height != b.Header.Height || gotHash != hash {
		t.Fatal("reopened head mismatch")
	}
	gotState, err := reopened.LoadState()
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := gotState.Get(types.Address("alice")); !ok || got != (state.Account{Balance: 42, Nonce: 3}) {
		t.Fatal("reopened state mismatch")
	}
}

func TestFileStoreEmpty(t *testing.T) {
	store, err := NewFileStore(filepath.Join(t.TempDir(), "chain.gob"))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Head(); err != ErrEmptyStore {
		t.Fatalf("Head error = %v", err)
	}
	if _, err := store.LoadState(); err != ErrEmptyStore {
		t.Fatalf("LoadState error = %v", err)
	}
}

func TestFileStoreRejectsCorruptFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "chain.gob")
	if err := os.WriteFile(path, []byte("not a gob snapshot"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := NewFileStore(path); err == nil {
		t.Fatal("expected corrupted storage file to fail closed")
	}
}

func TestFileStoreIgnoresOrphanTempFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "chain.gob")
	if err := os.WriteFile(filepath.Join(dir, ".indochain-orphan"), []byte("partial snapshot"), 0o600); err != nil {
		t.Fatal(err)
	}

	store, err := NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Head(); err != ErrEmptyStore {
		t.Fatalf("Head error = %v, want %v", err, ErrEmptyStore)
	}
}

func TestFileStoreFailedPersistenceLeavesMemoryUnchanged(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "chain.gob")
	store, err := NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}

	st := state.New()
	st.Set(types.Address("alice"), state.Account{Balance: 42, Nonce: 3})
	initialBlock := block.Block{Header: block.Header{Height: 0, StateRoot: st.Root()}}
	initialHash := types.Hash{1}
	if err := store.CommitBlockState(initialBlock, initialHash, st); err != nil {
		t.Fatal(err)
	}

	beforeBlock, beforeHash, err := store.Head()
	if err != nil {
		t.Fatal(err)
	}
	beforeState, err := store.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(path)
	})

	candidateState := state.New()
	candidateState.Set(types.Address("bob"), state.Account{Balance: 99, Nonce: 1})
	candidateBlock := block.Block{Header: block.Header{Height: 1, StateRoot: candidateState.Root()}}
	candidateHash := types.Hash{2}

	if err := store.CommitBlockState(candidateBlock, candidateHash, candidateState); err == nil {
		t.Fatal("expected persistence failure")
	}

	afterBlock, afterHash, err := store.Head()
	if err != nil {
		t.Fatal(err)
	}
	if afterBlock.Header.Height != beforeBlock.Header.Height || afterHash != beforeHash {
		t.Fatal("failed persistence mutated in-memory head")
	}
	afterState, err := store.LoadState()
	if err != nil {
		t.Fatal(err)
	}
	beforeAccount, beforeOK := beforeState.Get(types.Address("alice"))
	afterAccount, afterOK := afterState.Get(types.Address("alice"))
	if !beforeOK || !afterOK || beforeAccount != afterAccount {
		t.Fatal("failed persistence mutated in-memory state")
	}
	if _, ok := afterState.Get(types.Address("bob")); ok {
		t.Fatal("failed persistence exposed candidate state")
	}
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		t.Fatal("expected failed target path to remain a directory")
	}
}
