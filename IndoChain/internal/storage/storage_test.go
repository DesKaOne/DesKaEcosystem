package storage

import (
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/state"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

func TestMemoryStoreRoundTrip(t *testing.T) {
	store := NewMemoryStore()
	b := block.Block{Header: block.Header{Height: 7}}
	hash := types.Hash{1, 2, 3}

	if err := store.SaveBlock(b, hash); err != nil {
		t.Fatal(err)
	}

	gotBlock, gotHash, err := store.GetBlock(7)
	if err != nil {
		t.Fatal(err)
	}
	if gotBlock.Header.Height != 7 || gotHash != hash {
		t.Fatal("stored block did not round-trip")
	}

	st := state.New()
	st.Set(types.Address([]byte{1}), state.Account{Balance: 99})
	if err := store.SaveState(st); err != nil {
		t.Fatal(err)
	}

	loaded, err := store.LoadState()
	if err != nil {
		t.Fatal(err)
	}
	account, ok := loaded.Get(types.Address([]byte{1}))
	if !ok || account.Balance != 99 {
		t.Fatal("stored state did not round-trip")
	}
}

func TestMemoryStoreReturnsSnapshots(t *testing.T) {
	store := NewMemoryStore()
	st := state.New()
	st.Set(types.Address([]byte{1}), state.Account{Balance: 10})
	if err := store.SaveState(st); err != nil {
		t.Fatal(err)
	}

	loaded, err := store.LoadState()
	if err != nil {
		t.Fatal(err)
	}
	loaded.Set(types.Address([]byte{1}), state.Account{Balance: 20})

	again, err := store.LoadState()
	if err != nil {
		t.Fatal(err)
	}
	account, _ := again.Get(types.Address([]byte{1}))
	if account.Balance != 10 {
		t.Fatal("store state was not isolated from loaded snapshot")
	}
}

func TestMemoryStoreMissingBlock(t *testing.T) {
	store := NewMemoryStore()
	if _, _, err := store.GetBlock(99); err != ErrBlockNotFound {
		t.Fatalf("GetBlock error = %v, want %v", err, ErrBlockNotFound)
	}
}

func TestMemoryStoreHeadTracksHighestSavedBlock(t *testing.T) {
	store := NewMemoryStore()
	first := block.Block{Header: block.Header{Height: 3}}
	second := block.Block{Header: block.Header{Height: 5}}
	firstHash := types.Hash{3}
	secondHash := types.Hash{5}

	if err := store.SaveBlock(first, firstHash); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveBlock(second, secondHash); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveBlock(first, firstHash); err != nil {
		t.Fatal(err)
	}

	head, hash, err := store.Head()
	if err != nil {
		t.Fatal(err)
	}
	if head.Header.Height != 5 || hash != secondHash {
		t.Fatal("head did not remain at the highest saved block")
	}
}
