package postgres

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/DesKaOne/DesKaEcosystem/DesKaCash/internal/ledger"
)

func TestRepositoryApplyDebitWithPostgres(t *testing.T) {
	dsn := os.Getenv("DESKACASH_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("DESKACASH_TEST_DATABASE_URL is not set")
	}

	db, err := Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}

	resetSchema(t, ctx, db)

	repo := NewRepository(db)
	account, err := ledger.NewAccount("acc-debit-integration-1", "user-debit-integration-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateAccount(ctx, account); err != nil {
		t.Fatal(err)
	}

	credit, err := ledger.NewTransaction(
		"tx-debit-integration-credit",
		account.ID,
		ledger.Money{BaseUnits: 100_000},
		"topup",
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.ApplyCredit(ctx, credit, "seed:debit-integration"); err != nil {
		t.Fatal(err)
	}

	debit, err := ledger.NewTransaction(
		"tx-debit-integration-1",
		account.ID,
		ledger.Money{BaseUnits: 40_000},
		"withdrawal",
	)
	if err != nil {
		t.Fatal(err)
	}
	debit.ProviderID = "provider-debit-1"

	if err := repo.ApplyDebit(ctx, debit, "withdrawal:integration-1"); err != nil {
		t.Fatal(err)
	}

	got, err := repo.GetAccount(ctx, account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Balance.BaseUnits != 60_000 {
		t.Fatalf("expected 60000 base units, got %d", got.Balance.BaseUnits)
	}
	if got.Version != 3 {
		t.Fatalf("expected account version 3, got %d", got.Version)
	}

	storedTx, err := repo.GetTransaction(ctx, debit.ID)
	if err != nil {
		t.Fatal(err)
	}
	if storedTx.Status != ledger.StatusSucceeded {
		t.Fatalf("expected succeeded transaction, got %s", storedTx.Status)
	}
	if storedTx.ProviderID != debit.ProviderID {
		t.Fatalf("expected provider id %q, got %q", debit.ProviderID, storedTx.ProviderID)
	}

	entries, err := repo.ListEntries(ctx, account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 ledger entries, got %d", len(entries))
	}
	if entries[1].Type != ledger.EntryDebit || entries[1].Amount.BaseUnits != 40_000 {
		t.Fatalf("unexpected debit entry: %+v", entries[1])
	}

	if err := repo.ApplyDebit(ctx, debit, "withdrawal:integration-1"); !errors.Is(err, ledger.ErrDuplicateTransaction) {
		t.Fatalf("expected duplicate transaction error, got %v", err)
	}
}

func TestRepositoryApplyDebitInsufficientFundsRollsBack(t *testing.T) {
	dsn := os.Getenv("DESKACASH_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("DESKACASH_TEST_DATABASE_URL is not set")
	}

	db, err := Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
	resetSchema(t, ctx, db)

	repo := NewRepository(db)
	account, err := ledger.NewAccount("acc-debit-integration-2", "user-debit-integration-2")
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateAccount(ctx, account); err != nil {
		t.Fatal(err)
	}

	credit, err := ledger.NewTransaction("tx-debit-integration-seed", account.ID, ledger.Money{BaseUnits: 10_000}, "topup")
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.ApplyCredit(ctx, credit, "seed:debit-integration-2"); err != nil {
		t.Fatal(err)
	}

	debit, err := ledger.NewTransaction("tx-debit-integration-insufficient", account.ID, ledger.Money{BaseUnits: 20_000}, "withdrawal")
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.ApplyDebit(ctx, debit, "withdrawal:insufficient"); !errors.Is(err, ledger.ErrInsufficientFunds) {
		t.Fatalf("expected insufficient funds, got %v", err)
	}

	got, err := repo.GetAccount(ctx, account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Balance.BaseUnits != 10_000 {
		t.Fatalf("expected balance to remain 10000 base units, got %d", got.Balance.BaseUnits)
	}

	if _, err := repo.GetTransaction(ctx, debit.ID); !errors.Is(err, ledger.ErrNotFound) {
		t.Fatalf("expected debit transaction to be rolled back, got %v", err)
	}

	entries, err := repo.ListEntries(ctx, account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected only seed entry, got %d", len(entries))
	}
}
