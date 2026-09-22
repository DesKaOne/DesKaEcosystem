package ledger

import (
	"context"
	"testing"
)

func TestServiceApplyCreditIsIdempotentByTransactionID(t *testing.T) {
	ctx := context.Background()
	account, err := NewAccount("acc-1", "user-1")
	if err != nil {
		t.Fatal(err)
	}

	repo := NewMemoryRepository()
	service := NewService(repo)
	if err := service.RegisterAccount(ctx, account); err != nil {
		t.Fatal(err)
	}

	tx, err := NewTransaction("topup-1", "acc-1", Money{BaseUnits: 100_000}, "topup")
	if err != nil {
		t.Fatal(err)
	}

	if err := service.ApplyCredit(ctx, tx, "provider:demo-1"); err != nil {
		t.Fatal(err)
	}

	if err := service.ApplyCredit(ctx, tx, "provider:demo-1"); err != ErrDuplicateTransaction {
		t.Fatalf("expected duplicate transaction error, got %v", err)
	}

	accountAfter, err := service.GetAccount(ctx, "acc-1")
	if err != nil {
		t.Fatal(err)
	}

	if accountAfter.Balance.BaseUnits != 100_000 {
		t.Fatalf("expected 100000 base units, got %d", accountAfter.Balance.BaseUnits)
	}

	entries, err := service.Entries(ctx, "acc-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 ledger entry, got %d", len(entries))
	}
}
