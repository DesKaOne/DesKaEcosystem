package runtime

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/routing"
)

func TestOpenTransactionStorePostgresIntegration(t *testing.T) {
	dsn := os.Getenv("DESKAPROVIDER_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("DESKAPROVIDER_POSTGRES_DSN is not configured")
	}

	ctx := context.Background()
	cfg := Config{TransactionStoreDriver: "postgres", PostgresDSN: dsn}
	store, db, err := openTransactionStore(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	schema := "runtime_test_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	if _, err := db.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil { t.Fatal(err) }
	defer db.ExecContext(ctx, "DROP SCHEMA "+schema+" CASCADE")
	if _, err := db.ExecContext(ctx, "SET search_path TO "+schema); err != nil { t.Fatal(err) }

	migration, err := os.ReadFile(filepath.Join("..", "migrations", "001_provider_transactions.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, string(migration)); err != nil {
		t.Fatal(err)
	}

	pgStore, ok := store.(*routing.PostgresTransactionStore)
	if !ok {
		t.Fatalf("expected PostgreSQL transaction store, got %T", store)
	}
	states, err := pgStore.AllContextE(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(states) != 0 {
		t.Fatalf("expected fresh integration table, got %d transactions", len(states))
	}
}
