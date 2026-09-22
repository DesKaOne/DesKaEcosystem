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

func TestMemoryRepositoryDuplicateAccount(t *testing.T) {
	repo := NewMemoryRepository()
	ctx := context.Background()

	account, err := NewAccount("acc-duplicate", "user-1")
	if err != nil {
		t.Fatal(err)
	}

	if err := repo.CreateAccount(ctx, account); err != nil {
		t.Fatal(err)
	}

	if err := repo.CreateAccount(ctx, account); err != ErrDuplicateAccount {
		t.Fatalf("expected duplicate account error, got %v", err)
	}
}


func TestMemoryRepositoryApplyDebit(t *testing.T) {
	repo := NewMemoryRepository()
	ctx := context.Background()

	account, err := NewAccount("acc-debit", "user-debit")
	if err != nil {
		t.Fatal(err)
	}
	account.Balance = Money{BaseUnits: 5000}
	if err := repo.SaveAccount(ctx, account); err != nil {
		t.Fatal(err)
	}

	tx, err := NewTransaction("tx-debit", "acc-debit", Money{BaseUnits: 2000}, "withdrawal")
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.ApplyDebit(ctx, tx, "withdrawal-ref"); err != nil {
		t.Fatal(err)
	}

	got, err := repo.GetAccount(ctx, "acc-debit")
	if err != nil {
		t.Fatal(err)
	}
	if got.Balance.BaseUnits != 3000 {
		t.Fatalf("expected balance 3000, got %d", got.Balance.BaseUnits)
	}

	stored, err := repo.GetTransaction(ctx, "tx-debit")
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != StatusSucceeded {
		t.Fatalf("expected succeeded transaction, got %s", stored.Status)
	}

	entries, err := repo.ListEntries(ctx, "acc-debit")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Type != EntryDebit || entries[0].Amount.BaseUnits != 2000 || entries[0].Reference != "withdrawal-ref" {
		t.Fatalf("unexpected debit entry: %+v", entries[0])
	}
}

func TestMemoryRepositoryApplyDebitInsufficientFunds(t *testing.T) {
	repo := NewMemoryRepository()
	ctx := context.Background()

	account, err := NewAccount("acc-insufficient", "user-insufficient")
	if err != nil {
		t.Fatal(err)
	}
	account.Balance = Money{BaseUnits: 1000}
	if err := repo.SaveAccount(ctx, account); err != nil {
		t.Fatal(err)
	}

	tx, err := NewTransaction("tx-insufficient", "acc-insufficient", Money{BaseUnits: 2000}, "withdrawal")
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.ApplyDebit(ctx, tx, "withdrawal-ref"); err != ErrInsufficientFunds {
		t.Fatalf("expected insufficient funds, got %v", err)
	}

	got, err := repo.GetAccount(ctx, "acc-insufficient")
	if err != nil {
		t.Fatal(err)
	}
	if got.Balance.BaseUnits != 1000 {
		t.Fatalf("expected balance unchanged at 1000, got %d", got.Balance.BaseUnits)
	}
	if _, err := repo.GetTransaction(ctx, "tx-insufficient"); err != ErrNotFound {
		t.Fatalf("expected transaction not persisted, got %v", err)
	}
	entries, err := repo.ListEntries(ctx, "acc-insufficient")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no entries, got %d", len(entries))
	}
}

func TestMemoryRepositoryApplyDebitRejectsDuplicateAndInvalidAmount(t *testing.T) {
	repo := NewMemoryRepository()
	ctx := context.Background()

	account, err := NewAccount("acc-validation", "user-validation")
	if err != nil {
		t.Fatal(err)
	}
	account.Balance = Money{BaseUnits: 5000}
	if err := repo.SaveAccount(ctx, account); err != nil {
		t.Fatal(err)
	}

	tx, err := NewTransaction("tx-validation", "acc-validation", Money{BaseUnits: 1000}, "withdrawal")
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.ApplyDebit(ctx, tx, "first"); err != nil {
		t.Fatal(err)
	}
	if err := repo.ApplyDebit(ctx, tx, "duplicate"); err != ErrDuplicateTransaction {
		t.Fatalf("expected duplicate transaction, got %v", err)
	}

	invalid, err := NewTransaction("tx-invalid", "acc-validation", Money{BaseUnits: 1000}, "withdrawal")
	if err != nil {
		t.Fatal(err)
	}
	invalid.Amount = Money{BaseUnits: 0}
	if err := repo.ApplyDebit(ctx, invalid, "invalid"); err != ErrInvalidAmount {
		t.Fatalf("expected invalid amount, got %v", err)
	}
}
