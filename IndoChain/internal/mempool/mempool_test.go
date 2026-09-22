package mempool

import (
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/transaction"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

func testTx(nonce uint64) transaction.Transaction {
	return transaction.Transaction{
		Version:   1,
		ChainID:   1001,
		Nonce:     types.Nonce(nonce),
		Sender:    types.Address{1, 2, 3},
		Recipient: types.Address{4, 5, 6},
		Value:     100,
		GasLimit:  21000,
	}
}

func TestPoolAddDuplicateAndRemove(t *testing.T) {
	p := New(Config{MaxTransactions: 2})
	tx := testTx(1)

	if err := p.Add(tx); err != nil {
		t.Fatal(err)
	}
	if err := p.Add(tx); err != ErrDuplicate {
		t.Fatalf("expected duplicate, got %v", err)
	}
	if p.Len() != 1 {
		t.Fatalf("expected length 1, got %d", p.Len())
	}

	hash := transaction.Hash(tx).String()
	if !p.Remove(hash) {
		t.Fatal("expected removal")
	}
	if p.Len() != 0 {
		t.Fatalf("expected empty pool")
	}
}

func TestPoolCapacity(t *testing.T) {
	p := New(Config{MaxTransactions: 1})
	if err := p.Add(testTx(1)); err != nil {
		t.Fatal(err)
	}
	if err := p.Add(testTx(2)); err != ErrFull {
		t.Fatalf("expected full, got %v", err)
	}
}
