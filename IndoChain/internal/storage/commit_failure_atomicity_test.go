package storage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/state"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

func TestFileStoreCommitFailureDoesNotPublishPartialCanonicalState(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "chain.gob")

	store, err := NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}

	initialState := state.New()
	initialBlock := block.Block{Header: block.Header{Version: 1, ChainID: 1, Height: 1}}
	initialHash, err := block.Hash(initialBlock)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CommitBlockState(initialBlock, initialHash, initialState); err != nil {
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

	failurePath := filepath.Join(dir, "commit-failure")
	if err := os.Mkdir(failurePath, 0o755); err != nil {
		t.Fatal(err)
	}
	originalPath := store.path
	store.path = failurePath
	t.Cleanup(func() {
		store.path = originalPath
	})

	candidateBlock := block.Block{Header: block.Header{Version: 1, ChainID: 1, Height: 2}}
	candidateHash, err := block.Hash(candidateBlock)
	if err != nil {
		t.Fatal(err)
	}
	candidateState := state.New()

	if err := store.CommitBlockState(candidateBlock, candidateHash, candidateState); err == nil {
		t.Fatal("CommitBlockState unexpectedly succeeded")
	}

	afterBlock, afterHash, err := store.Head()
	if err != nil {
		t.Fatal(err)
	}
	afterState, err := store.LoadState()
	if err != nil {
		t.Fatal(err)
	}

	if afterBlock.Header.Height != beforeBlock.Header.Height || afterHash != beforeHash {
		t.Fatalf("head changed after failed commit: before height=%d hash=%s, after height=%d hash=%s",
			beforeBlock.Header.Height, beforeHash, afterBlock.Header.Height, afterHash)
	}
	if len(afterState.Accounts()) != len(beforeState.Accounts()) {
		t.Fatalf("state changed after failed commit: before accounts=%d, after accounts=%d",
			len(beforeState.Accounts()), len(afterState.Accounts()))
	}

	store.path = originalPath
	reopened, err := NewFileStore(originalPath)
	if err != nil {
		t.Fatal(err)
	}
	persistedBlock, persistedHash, err := reopened.Head()
	if err != nil {
		t.Fatal(err)
	}
	if persistedBlock.Header.Height != beforeBlock.Header.Height || persistedHash != beforeHash {
		t.Fatalf("persisted head changed after failed commit: before height=%d hash=%s, after height=%d hash=%s",
			beforeBlock.Header.Height, beforeHash, persistedBlock.Header.Height, persistedHash)
	}
	persistedState, err := reopened.LoadState()
	if err != nil {
		t.Fatal(err)
	}
	if len(persistedState.Accounts()) != len(beforeState.Accounts()) {
		t.Fatalf("persisted state changed after failed commit: before accounts=%d, after accounts=%d",
			len(beforeState.Accounts()), len(persistedState.Accounts()))
	}
}

func TestMemoryStoreCommitBlockStatePublishesBlockAndStateTogetherOnSuccess(t *testing.T) {
	store := NewMemoryStore()
	st := state.New()
	b := block.Block{Header: block.Header{Version: 1, ChainID: 1, Height: 1}}
	hash, err := block.Hash(b)
	if err != nil {
		t.Fatal(err)
	}

	if err := store.CommitBlockState(b, hash, st); err != nil {
		t.Fatal(err)
	}
	headBlock, headHash, err := store.Head()
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := store.LoadState()
	if err != nil {
		t.Fatal(err)
	}
	if headBlock.Header.Height != b.Header.Height || headHash != hash {
		t.Fatalf("unexpected memory-store head: height=%d hash=%s", headBlock.Header.Height, headHash)
	}
	if len(loaded.Accounts()) != len(st.Accounts()) {
		t.Fatal("memory-store state differs after successful commit")
	}
}
