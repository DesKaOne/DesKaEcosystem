package ledger

import (
	"context"
	"testing"
)

func TestMemoryRepositoryRoundTrip(t *testing.T) {
	repo := NewMemoryRepository()
	ctx := context.Background()

	account, err := NewAccount("acc-1", "user-1")
	if err != nil {
		t.Fatal(err)
	}

	if err := repo.SaveAccount(ctx, account); err != nil {
		t.Fatal(err)
	}

	got, err := repo.GetAccount(ctx, "acc-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != account.ID || got.UserID != account.UserID {
		t.Fatalf("unexpected account: %+v", got)
	}

	tx, err := NewTransaction("tx-1", "acc-1", Money{BaseUnits: 1000}, "topup")
	if err != nil {
		t.Fatal(err)
	}

	if err := repo.CreateTransaction(ctx, tx); err != nil {
		t.Fatal(err)
	}

	if err := repo.CreateTransaction(ctx, tx); err != ErrDuplicateTransaction {
		t.Fatalf("expected duplicate transaction error, got %v", err)
	}

	entry := Entry{
		ID:            "entry-1",
		AccountID:     "acc-1",
		TransactionID: "tx-1",
		Type:          EntryCredit,
		Asset:         AssetDIDR,
		Amount:        tx.Amount,
	}

	if err := repo.CreateEntry(ctx, entry); err != nil {
		t.Fatal(err)
	}

	entries, err := repo.ListEntries(ctx, "acc-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
}
