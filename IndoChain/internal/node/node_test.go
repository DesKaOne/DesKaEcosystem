package node

import (
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/storage"
)

func TestNewDevnetInitializesGenesis(t *testing.T) {
	store := storage.NewMemoryStore()
	n, err := NewDevnet(store)
	if err != nil {
		t.Fatal(err)
	}

	if n.Head.Header.Height != 0 {
		t.Fatalf("head height = %d, want 0", n.Head.Header.Height)
	}
	if n.HeadHash == (n.HeadHash) && n.HeadHash == [32]byte{} {
		t.Fatal("genesis head hash must be non-zero")
	}

	storedBlock, storedHash, err := store.Head()
	if err != nil {
		t.Fatal(err)
	}
	if storedBlock.Header.Height != n.Head.Header.Height || storedHash != n.HeadHash {
		t.Fatal("store head does not match node head")
	}

	loadedState, err := store.LoadState()
	if err != nil {
		t.Fatal(err)
	}
	if loadedState.Root() != n.State.Root() {
		t.Fatal("stored state does not match node state")
	}
}

func TestNewDevnetRejectsNilStore(t *testing.T) {
	if _, err := NewDevnet(nil); err != ErrNilStore {
		t.Fatalf("error = %v, want %v", err, ErrNilStore)
	}
}
