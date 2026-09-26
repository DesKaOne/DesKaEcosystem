package mempool

import (
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/transaction"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

func TestPoolAddAndGet(t *testing.T) {
	p := New(Config{MaxTransactions: 2})
	tx := transaction.Transaction{
		Version:   1,
		ChainID:   1001,
		Nonce:     0,
		Sender:    types.Address{1},
		Recipient: types.Address{2},
		Value:     10,
	}
	if err := p.Add(tx); err != nil {
		t.Fatalf("add: %v", err)
	}
	if got, ok := p.Get(transaction.Hash(tx).String()); !ok || got.Value != tx.Value {
		t.Fatalf("get mismatch")
	}
}

func TestPoolRejectsDuplicate(t *testing.T) {
	p := New(Config{MaxTransactions: 2})
	tx := transaction.Transaction{Version: 1, ChainID: 1001, Sender: types.Address{1}, Recipient: types.Address{2}, Value: 10}
	if err := p.Add(tx); err != nil {
		t.Fatalf("first add: %v", err)
	}
	if err := p.Add(tx); err != ErrDuplicate {
		t.Fatalf("expected duplicate, got %v", err)
	}
}

func TestPoolRejectsFull(t *testing.T) {
	p := New(Config{MaxTransactions: 1})
	tx1 := transaction.Transaction{Version: 1, ChainID: 1001, Sender: types.Address{1}, Recipient: types.Address{2}, Value: 10}
	tx2 := transaction.Transaction{Version: 1, ChainID: 1001, Sender: types.Address{3}, Recipient: types.Address{4}, Value: 20}
	if err := p.Add(tx1); err != nil {
		t.Fatalf("first add: %v", err)
	}
	if err := p.Add(tx2); err != ErrFull {
		t.Fatalf("expected full, got %v", err)
	}
}

func TestPoolSnapshotSortedIsDeterministic(t *testing.T) {
	p := New(Config{MaxTransactions: 10})
	txs := []transaction.Transaction{
		{Version: 1, ChainID: 1001, Sender: types.Address{3}, Recipient: types.Address{4}, Value: 30},
		{Version: 1, ChainID: 1001, Sender: types.Address{1}, Recipient: types.Address{2}, Value: 10},
		{Version: 1, ChainID: 1001, Sender: types.Address{2}, Recipient: types.Address{3}, Value: 20},
	}
	for _, tx := range txs {
		if err := p.Add(tx); err != nil {
			t.Fatalf("add: %v", err)
		}
	}

	first := p.SnapshotSorted()
	second := p.SnapshotSorted()
	if len(first) != len(second) {
		t.Fatalf("snapshot lengths differ: %d vs %d", len(first), len(second))
	}
	for i := range first {
		if transaction.Hash(first[i]) != transaction.Hash(second[i]) {
			t.Fatalf("snapshot order changed at index %d", i)
		}
		if i > 0 && transaction.Hash(first[i-1]).String() >= transaction.Hash(first[i]).String() {
			t.Fatalf("snapshot is not strictly hash ordered at index %d", i)
		}
	}
}

func TestPoolSnapshotSortedDoesNotMutatePool(t *testing.T) {
	p := New(Config{MaxTransactions: 10})
	tx := transaction.Transaction{Version: 1, ChainID: 1001, Sender: types.Address{1}, Recipient: types.Address{2}, Value: 10}
	if err := p.Add(tx); err != nil {
		t.Fatalf("add: %v", err)
	}
	_ = p.SnapshotSorted()
	if p.Len() != 1 {
		t.Fatalf("expected pool length 1, got %d", p.Len())
	}
}
