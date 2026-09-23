package postgres

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
	"database/sql"

	"github.com/DesKaOne/DesKaEcosystem/DesKaCash/backend/internal/ledger"
)

func TestRepositoryApplyCreditWithPostgres(t *testing.T) {
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

	account, err := ledger.NewAccount("acc-integration-1", "user-integration-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateAccount(ctx, account); err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateAccount(ctx, account); !errors.Is(err, ledger.ErrDuplicateAccount) {
		t.Fatalf("expected duplicate account error, got %v", err)
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
	tx.ProviderID = "provider-tx-1"

	if err := repo.ApplyCredit(ctx, tx, "provider:integration-1"); err != nil {
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
	if storedTx.ProviderID != tx.ProviderID {
		t.Fatalf("expected provider id %q, got %q", tx.ProviderID, storedTx.ProviderID)
	}

	entries, err := repo.ListEntries(ctx, account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 ledger entry, got %d", len(entries))
	}

	if err := repo.ApplyCredit(ctx, tx, "provider:integration-1"); !errors.Is(err, ledger.ErrDuplicateTransaction) {
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

func resetSchema(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()

	if _, err := db.ExecContext(ctx, "DROP TABLE IF EXISTS ledger_postings, ledger_entries, transactions, accounts CASCADE"); err != nil {
		t.Fatal(err)
	}

	schema, err := os.ReadFile(migrationPath(t))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, string(schema)); err != nil {
		t.Fatal(err)
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


func TestRepositoryStoresImmutablePostingsWithPostgres(t *testing.T) {
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
	account, err := ledger.NewAccount("posting-account-1", "posting-user-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateAccount(ctx, account); err != nil {
		t.Fatal(err)
	}
	tx, err := ledger.NewTransaction("posting-tx-1", account.ID, ledger.Money{BaseUnits: 1000}, "transfer")
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateTransaction(ctx, tx); err != nil {
		t.Fatal(err)
	}

	credit, err := ledger.NewPosting("posting-credit-1", tx.ID, account.ID, ledger.AssetIDR, ledger.PostingCredit, ledger.Money{BaseUnits: 1000}, "transfer")
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.CreatePosting(ctx, credit); err != nil {
		t.Fatal(err)
	}
	if err := repo.CreatePosting(ctx, credit); !errors.Is(err, ledger.ErrDuplicatePosting) {
		t.Fatalf("expected duplicate posting error, got %v", err)
	}

	postings, err := repo.ListPostings(ctx, tx.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(postings) != 1 {
		t.Fatalf("expected 1 posting, got %d", len(postings))
	}
	if postings[0].ID != credit.ID || postings[0].Amount.BaseUnits != 1000 {
		t.Fatalf("unexpected posting: %#v", postings[0])
	}
}
