package node

import (
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/state"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/storage"
)

func TestChainReaderHeadAndBlockByHeight(t *testing.T) {
	store := storage.NewMemoryStore()
	n, err := NewDevnet(store)
	if err != nil {
		t.Fatal(err)
	}

	head, headHash, err := n.HeadBlock()
	if err != nil {
		t.Fatal(err)
	}
	if head.Header.Height != 0 || headHash != n.HeadHash {
		t.Fatal("head query does not match node head")
	}

	genesis, genesisHash, err := n.BlockByHeight(0)
	if err != nil {
		t.Fatal(err)
	}
	if genesis.Header.Height != 0 || genesisHash != n.HeadHash {
		t.Fatal("height query does not return canonical genesis")
	}

	if _, _, err := n.BlockByHeight(1); err != storage.ErrBlockNotFound {
		t.Fatalf("error = %v, want %v", err, storage.ErrBlockNotFound)
	}
}

func TestChainReaderStateSnapshotIsolated(t *testing.T) {
	store := storage.NewMemoryStore()
	n, err := NewDevnet(store)
	if err != nil {
		t.Fatal(err)
	}

	address := types.Address([]byte("query-isolation"))
	n.State.Set(address, state.Account{Balance: 10, Nonce: 0})

	snapshot, err := n.StateSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	snapshot.Set(address, state.Account{Balance: 999, Nonce: 99})

	got, ok := n.State.Get(address)
	if !ok {
		t.Fatal("canonical state account disappeared")
	}
	if got.Balance != 10 || got.Nonce != 0 {
		t.Fatalf("canonical state mutated through query snapshot: %+v", got)
	}
}

func TestChainReaderRejectsNilNode(t *testing.T) {
	var n *Node

	if _, _, err := n.HeadBlock(); err != ErrNilNode {
		t.Fatalf("HeadBlock error = %v, want %v", err, ErrNilNode)
	}
	if _, _, err := n.BlockByHeight(0); err != ErrNilNode {
		t.Fatalf("BlockByHeight error = %v, want %v", err, ErrNilNode)
	}
	if _, err := n.StateSnapshot(); err != ErrNilNode {
		t.Fatalf("StateSnapshot error = %v, want %v", err, ErrNilNode)
	}
}
