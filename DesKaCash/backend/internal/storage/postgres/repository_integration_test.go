package postgres

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/DesKaOne/DesKaEcosystem/DesKaCash/backend/internal/ledger"
)

func TestRepositoryCreditWithPostgres(t *testing.T) {
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

	schemaPath := migrationPath(t)
	schema, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, string(schema)); err != nil {
		t.Fatal(err)
	}

	repo := NewRepository(db)

	account, err := ledger.NewAccount("acc-integration-1", "user-integration-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateAccount(ctx, account); err != nil {
		t.Fatal(err)
	}

	tx, err := ledger.NewTransaction(
		"tx-integration-1",
		account.ID,
		ledger.Money{BaseUnits: 100_000},
		"topup",
	)
	if err != nil {
		t.Fatal(err)
	}

	if err := repo.Credit(ctx, tx, "provider:integration-1"); err != nil {
		t.Fatal(err)
	}

	got, err := repo.GetAccount(ctx, account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Balance.BaseUnits != 100_000 {
		t.Fatalf("expected 100000 base units, got %d", got.Balance.BaseUnits)
	}
	if got.Version != 2 {
		t.Fatalf("expected account version 2, got %d", got.Version)
	}

	storedTx, err := repo.GetTransaction(ctx, tx.ID)
	if err != nil {
		t.Fatal(err)
	}
	if storedTx.Status != ledger.StatusSucceeded {
		t.Fatalf("expected succeeded transaction, got %s", storedTx.Status)
	}

	entries, err := repo.ListEntries(ctx, account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 ledger entry, got %d", len(entries))
	}

	if err := repo.Credit(ctx, tx, "provider:integration-1"); !errors.Is(err, ledger.ErrDuplicateTransaction) {
		t.Fatalf("expected duplicate transaction error, got %v", err)
	}

	got, err = repo.GetAccount(ctx, account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Balance.BaseUnits != 100_000 {
		t.Fatalf("expected balance to remain 100000 base units, got %d", got.Balance.BaseUnits)
	}

	entries, err = repo.ListEntries(ctx, account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 ledger entry after duplicate credit, got %d", len(entries))
	}
}

func migrationPath(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to resolve integration test path")
	}

	return filepath.Join(filepath.Dir(file), "../../../migrations/001_init_ledger.sql")
}
