package storage

import (
	"path/filepath"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/state"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

func TestFileStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "chain.gob")
	store, err := NewFileStore(path)
	if err != nil { t.Fatal(err) }

	st := state.New()
	st.Set(types.Address("alice"), state.Account{Balance: 42, Nonce: 3})
	b := block.Block{Header: block.Header{Height: 7, StateRoot: st.Root()}}
	hash := types.Hash{7}

	if err := store.CommitBlockState(b, hash, st); err != nil { t.Fatal(err) }

	reopened, err := NewFileStore(path)
	if err != nil { t.Fatal(err) }
	gotBlock, gotHash, err := reopened.Head()
	if err != nil { t.Fatal(err) }
	if gotBlock.Header.Height != b.Header.Height || gotHash != hash { t.Fatal("reopened head mismatch") }
	gotState, err := reopened.LoadState()
	if err != nil { t.Fatal(err) }
	if got, ok := gotState.Get(types.Address("alice")); !ok || got != (state.Account{Balance: 42, Nonce: 3}) {
		t.Fatal("reopened state mismatch")
	}
}

func TestFileStoreEmpty(t *testing.T) {
	store, err := NewFileStore(filepath.Join(t.TempDir(), "chain.gob"))
	if err != nil { t.Fatal(err) }
	if _, _, err := store.Head(); err != ErrEmptyStore { t.Fatalf("Head error = %v", err) }
	if _, err := store.LoadState(); err != ErrEmptyStore { t.Fatalf("LoadState error = %v", err) }
}
