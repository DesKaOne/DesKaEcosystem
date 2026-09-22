package state

import (
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

func TestTransferUpdatesBalanceAndNonce(t *testing.T) {
	s := New()
	sender := types.Address{1}
	recipient := types.Address{2}
	s.Set(sender, Account{Balance: 100, Nonce: 7})

	if err := s.Transfer(sender, recipient, 30, 7); err != nil {
		t.Fatal(err)
	}

	from, ok := s.Get(sender)
	if !ok {
		t.Fatal("sender account missing")
	}
	to, ok := s.Get(recipient)
	if !ok {
		t.Fatal("recipient account missing")
	}

	if from.Balance != 70 || from.Nonce != 8 {
		t.Fatalf("unexpected sender state: %+v", from)
	}
	if to.Balance != 30 || to.Nonce != 0 {
		t.Fatalf("unexpected recipient state: %+v", to)
	}
}

func TestTransferRejectsInvalidNonceAndInsufficientBalance(t *testing.T) {
	s := New()
	sender := types.Address{1}
	recipient := types.Address{2}
	s.Set(sender, Account{Balance: 10, Nonce: 2})

	if err := s.Transfer(sender, recipient, 1, 1); err != ErrNonceMismatch {
		t.Fatalf("expected nonce mismatch, got %v", err)
	}
	if err := s.Transfer(sender, recipient, 11, 2); err != ErrInsufficientBalance {
		t.Fatalf("expected insufficient balance, got %v", err)
	}
}

func TestSnapshotAndReplace(t *testing.T) {
	s := New()
	address := types.Address{1}
	s.Set(address, Account{Balance: 42, Nonce: 3})

	snapshot := s.Snapshot()
	s.Set(address, Account{Balance: 99, Nonce: 4})
	s.Replace(snapshot)

	account, ok := s.Get(address)
	if !ok {
		t.Fatal("account missing after replace")
	}
	if account.Balance != 42 || account.Nonce != 3 {
		t.Fatalf("unexpected restored account: %+v", account)
	}
}
